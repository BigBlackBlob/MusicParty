package org.thornex.musicparty.persistence;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBean;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.RoomPlaylist;
import org.thornex.musicparty.dto.RoomPlaylistTrack;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

@Repository
@RequiredArgsConstructor
@ConditionalOnBean(JdbcTemplate.class)
public class JdbcRoomPlaylistRepository implements RoomPlaylistRepository {
    private final JdbcTemplate jdbcTemplate;
    private final ObjectMapper objectMapper;

    @Override
    public List<RoomPlaylist> listPlaylists(String roomId) {
        return jdbcTemplate.query("""
                select p.id, p.room_id, p.name, p.created_at, p.updated_at, count(t.id) as track_count
                from room_playlist p
                left join room_playlist_track t on t.playlist_id = p.id
                where p.room_id = ?
                group by p.id, p.room_id, p.name, p.created_at, p.updated_at
                order by p.created_at
                """, (rs, rowNum) -> new RoomPlaylist(
                rs.getString("id"),
                rs.getString("room_id"),
                rs.getString("name"),
                rs.getInt("track_count"),
                rs.getLong("created_at"),
                rs.getLong("updated_at")
        ), roomId);
    }

    @Override
    public Optional<RoomPlaylist> findPlaylist(String roomId, String playlistId) {
        List<RoomPlaylist> result = jdbcTemplate.query("""
                select p.id, p.room_id, p.name, p.created_at, p.updated_at, count(t.id) as track_count
                from room_playlist p
                left join room_playlist_track t on t.playlist_id = p.id
                where p.room_id = ? and p.id = ?
                group by p.id, p.room_id, p.name, p.created_at, p.updated_at
                """, (rs, rowNum) -> new RoomPlaylist(
                rs.getString("id"),
                rs.getString("room_id"),
                rs.getString("name"),
                rs.getInt("track_count"),
                rs.getLong("created_at"),
                rs.getLong("updated_at")
        ), roomId, playlistId);
        return result.stream().findFirst();
    }

    @Override
    public RoomPlaylist createPlaylist(String roomId, String name) {
        long now = System.currentTimeMillis();
        String id = UUID.randomUUID().toString();
        jdbcTemplate.update("insert into room_playlist(id, room_id, name, created_at, updated_at) values (?, ?, ?, ?, ?)",
                id, roomId, name, now, now);
        return new RoomPlaylist(id, roomId, name, 0, now, now);
    }

    @Override
    public Optional<RoomPlaylist> renamePlaylist(String roomId, String playlistId, String name) {
        long now = System.currentTimeMillis();
        int updated = jdbcTemplate.update("update room_playlist set name = ?, updated_at = ? where room_id = ? and id = ?",
                name, now, roomId, playlistId);
        return updated == 0 ? Optional.empty() : findPlaylist(roomId, playlistId);
    }

    @Override
    public boolean deletePlaylist(String roomId, String playlistId) {
        jdbcTemplate.update("delete from room_playlist_track where playlist_id = ? and exists (select 1 from room_playlist where id = ? and room_id = ?)",
                playlistId, playlistId, roomId);
        return jdbcTemplate.update("delete from room_playlist where room_id = ? and id = ?", roomId, playlistId) > 0;
    }

    @Override
    public List<RoomPlaylistTrack> listTracks(String roomId, String playlistId, int offset, int limit) {
        int safeOffset = Math.max(0, offset);
        int safeLimit = Math.max(1, Math.min(500, limit));
        return jdbcTemplate.query("""
                select t.id, t.playlist_id, t.music_json, t.sort_order, t.created_at
                from room_playlist_track t
                join room_playlist p on p.id = t.playlist_id
                where p.room_id = ? and t.playlist_id = ?
                order by t.sort_order
                limit ? offset ?
                """, (rs, rowNum) -> new RoomPlaylistTrack(
                rs.getString("id"),
                rs.getString("playlist_id"),
                readMusic(rs.getString("music_json")),
                rs.getInt("sort_order"),
                rs.getLong("created_at")
        ), roomId, playlistId, safeLimit, safeOffset);
    }

    @Override
    public Optional<RoomPlaylistTrack> addTrack(String roomId, String playlistId, Music music) {
        if (music == null || findPlaylist(roomId, playlistId).isEmpty()) return Optional.empty();
        int sortOrder = Optional.ofNullable(jdbcTemplate.queryForObject(
                "select coalesce(max(sort_order), -1) + 1 from room_playlist_track where playlist_id = ?",
                Integer.class,
                playlistId
        )).orElse(0);
        long now = System.currentTimeMillis();
        String id = UUID.randomUUID().toString();
        jdbcTemplate.update("insert into room_playlist_track(id, playlist_id, music_json, sort_order, created_at) values (?, ?, ?, ?, ?)",
                id, playlistId, writeMusic(music), sortOrder, now);
        touch(playlistId);
        return Optional.of(new RoomPlaylistTrack(id, playlistId, music, sortOrder, now));
    }

    @Override
    public boolean deleteTrack(String roomId, String playlistId, String trackId) {
        if (findPlaylist(roomId, playlistId).isEmpty()) return false;
        boolean deleted = jdbcTemplate.update("delete from room_playlist_track where playlist_id = ? and id = ?", playlistId, trackId) > 0;
        if (deleted) {
            rewriteOrder(roomId, playlistId);
            touch(playlistId);
        }
        return deleted;
    }

    @Override
    public void reorderTracks(String roomId, String playlistId, List<String> orderedTrackIds) {
        if (findPlaylist(roomId, playlistId).isEmpty() || orderedTrackIds == null) return;
        int index = 0;
        for (String trackId : orderedTrackIds) {
            jdbcTemplate.update("update room_playlist_track set sort_order = ? where playlist_id = ? and id = ?",
                    index++, playlistId, trackId);
        }
        rewriteOrder(roomId, playlistId);
        touch(playlistId);
    }

    @Override
    public void deleteRoomData(String roomId) {
        jdbcTemplate.update("""
                delete from room_playlist_track
                where playlist_id in (select id from room_playlist where room_id = ?)
                """, roomId);
        jdbcTemplate.update("delete from room_playlist where room_id = ?", roomId);
    }

    private void rewriteOrder(String roomId, String playlistId) {
        List<String> ids = jdbcTemplate.query("""
                select t.id from room_playlist_track t
                join room_playlist p on p.id = t.playlist_id
                where p.room_id = ? and t.playlist_id = ?
                order by t.sort_order, t.created_at
                """, (rs, rowNum) -> rs.getString("id"), roomId, playlistId);
        for (int i = 0; i < ids.size(); i++) {
            jdbcTemplate.update("update room_playlist_track set sort_order = ? where id = ?", i, ids.get(i));
        }
    }

    private void touch(String playlistId) {
        jdbcTemplate.update("update room_playlist set updated_at = ? where id = ?", System.currentTimeMillis(), playlistId);
    }

    private String writeMusic(Music music) {
        try {
            return objectMapper.writeValueAsString(music);
        } catch (JsonProcessingException e) {
            throw new IllegalArgumentException("Unable to serialize playlist track", e);
        }
    }

    private Music readMusic(String json) {
        try {
            return objectMapper.readValue(json, Music.class);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Unable to read playlist track", e);
        }
    }
}
