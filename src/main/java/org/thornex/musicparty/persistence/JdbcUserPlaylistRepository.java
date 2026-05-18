package org.thornex.musicparty.persistence;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.UserPlaylist;
import org.thornex.musicparty.dto.UserPlaylistTrack;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcUserPlaylistRepository implements UserPlaylistRepository {
    private final JdbcTemplate jdbcTemplate;
    private final ObjectMapper objectMapper;

    @Override
    public List<UserPlaylist> listPlaylists(String ownerPublicId) {
        return jdbcTemplate.query("""
                select p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at, count(t.id) as track_count
                from user_playlist p
                left join user_playlist_track t on t.playlist_id = p.id
                where p.owner_public_id = ?
                group by p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at
                order by case when p.system_key is null then 1 else 0 end, p.created_at
                """, (rs, rowNum) -> toPlaylist(rs), ownerPublicId);
    }

    @Override
    public Optional<UserPlaylist> findPlaylist(String ownerPublicId, String playlistId) {
        return jdbcTemplate.query("""
                select p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at, count(t.id) as track_count
                from user_playlist p
                left join user_playlist_track t on t.playlist_id = p.id
                where p.owner_public_id = ? and p.id = ?
                group by p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at
                """, (rs, rowNum) -> toPlaylist(rs), ownerPublicId, playlistId).stream().findFirst();
    }

    @Override
    public Optional<UserPlaylist> findSystemPlaylist(String ownerPublicId, String systemKey) {
        return jdbcTemplate.query("""
                select p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at, count(t.id) as track_count
                from user_playlist p
                left join user_playlist_track t on t.playlist_id = p.id
                where p.owner_public_id = ? and p.system_key = ?
                group by p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at
                """, (rs, rowNum) -> toPlaylist(rs), ownerPublicId, systemKey).stream().findFirst();
    }

    @Override
    public UserPlaylist createPlaylist(String ownerPublicId, String name) {
        long now = System.currentTimeMillis();
        String id = UUID.randomUUID().toString();
        jdbcTemplate.update("insert into user_playlist(id, owner_public_id, name, system_key, created_at, updated_at) values (?, ?, ?, null, ?, ?)",
                id, ownerPublicId, name, now, now);
        return new UserPlaylist(id, ownerPublicId, name, null, 0, now, now);
    }

    @Override
    public UserPlaylist createSystemPlaylist(String ownerPublicId, String name, String systemKey) {
        long now = System.currentTimeMillis();
        String id = UUID.randomUUID().toString();
        jdbcTemplate.update("""
                insert into user_playlist(id, owner_public_id, name, system_key, created_at, updated_at)
                values (?, ?, ?, ?, ?, ?)
                on conflict(owner_public_id, system_key) do update set name = excluded.name
                """, id, ownerPublicId, name, systemKey, now, now);
        return findSystemPlaylist(ownerPublicId, systemKey).orElseGet(() -> new UserPlaylist(id, ownerPublicId, name, systemKey, 0, now, now));
    }

    @Override
    public Optional<UserPlaylist> renamePlaylist(String ownerPublicId, String playlistId, String name) {
        long now = System.currentTimeMillis();
        int updated = jdbcTemplate.update("update user_playlist set name = ?, updated_at = ? where owner_public_id = ? and id = ?",
                name, now, ownerPublicId, playlistId);
        return updated == 0 ? Optional.empty() : findPlaylist(ownerPublicId, playlistId);
    }

    @Override
    public boolean deletePlaylist(String ownerPublicId, String playlistId) {
        jdbcTemplate.update("""
                delete from user_playlist_track
                where playlist_id = ? and exists (
                    select 1 from user_playlist where id = ? and owner_public_id = ?
                )
                """, playlistId, playlistId, ownerPublicId);
        return jdbcTemplate.update("delete from user_playlist where owner_public_id = ? and id = ?", ownerPublicId, playlistId) > 0;
    }

    @Override
    public List<UserPlaylistTrack> listTracks(String ownerPublicId, String playlistId, int offset, int limit) {
        int safeOffset = Math.max(0, offset);
        int safeLimit = Math.max(1, Math.min(500, limit));
        return jdbcTemplate.query("""
                select t.id, t.playlist_id, t.music_json, t.sort_order, t.created_at
                from user_playlist_track t
                join user_playlist p on p.id = t.playlist_id
                where p.owner_public_id = ? and t.playlist_id = ?
                order by t.sort_order
                limit ? offset ?
                """, (rs, rowNum) -> new UserPlaylistTrack(
                rs.getString("id"),
                rs.getString("playlist_id"),
                readMusic(rs.getString("music_json")),
                rs.getInt("sort_order"),
                rs.getLong("created_at")
        ), ownerPublicId, playlistId, safeLimit, safeOffset);
    }

    @Override
    public Optional<UserPlaylistTrack> addTrackIfAbsent(String ownerPublicId, String playlistId, Music music) {
        if (music == null || findPlaylist(ownerPublicId, playlistId).isEmpty()) return Optional.empty();
        String musicKey = musicKey(music);
        Integer existing = jdbcTemplate.queryForObject("""
                select count(1) from user_playlist_track
                where playlist_id = ? and music_key = ?
                """, Integer.class, playlistId, musicKey);
        if (existing != null && existing > 0) return Optional.empty();
        int sortOrder = Optional.ofNullable(jdbcTemplate.queryForObject(
                "select coalesce(max(sort_order), -1) + 1 from user_playlist_track where playlist_id = ?",
                Integer.class,
                playlistId
        )).orElse(0);
        long now = System.currentTimeMillis();
        String id = UUID.randomUUID().toString();
        jdbcTemplate.update("""
                insert into user_playlist_track(id, playlist_id, music_json, music_key, sort_order, created_at)
                values (?, ?, ?, ?, ?, ?)
                """, id, playlistId, writeMusic(music), musicKey, sortOrder, now);
        touch(playlistId);
        return Optional.of(new UserPlaylistTrack(id, playlistId, music, sortOrder, now));
    }

    @Override
    public boolean deleteTrack(String ownerPublicId, String playlistId, String trackId) {
        if (findPlaylist(ownerPublicId, playlistId).isEmpty()) return false;
        boolean deleted = jdbcTemplate.update("delete from user_playlist_track where playlist_id = ? and id = ?", playlistId, trackId) > 0;
        if (deleted) {
            rewriteOrder(ownerPublicId, playlistId);
            touch(playlistId);
        }
        return deleted;
    }

    @Override
    public boolean deleteTrackByMusicKey(String ownerPublicId, String playlistId, String musicKey) {
        if (findPlaylist(ownerPublicId, playlistId).isEmpty()) return false;
        boolean deleted = jdbcTemplate.update("delete from user_playlist_track where playlist_id = ? and music_key = ?", playlistId, musicKey) > 0;
        if (deleted) {
            rewriteOrder(ownerPublicId, playlistId);
            touch(playlistId);
        }
        return deleted;
    }

    @Override
    public void reorderTracks(String ownerPublicId, String playlistId, List<String> orderedTrackIds) {
        if (findPlaylist(ownerPublicId, playlistId).isEmpty() || orderedTrackIds == null) return;
        int index = 0;
        for (String trackId : orderedTrackIds) {
            jdbcTemplate.update("update user_playlist_track set sort_order = ? where playlist_id = ? and id = ?",
                    index++, playlistId, trackId);
        }
        rewriteOrder(ownerPublicId, playlistId);
        touch(playlistId);
    }

    private void rewriteOrder(String ownerPublicId, String playlistId) {
        List<String> ids = jdbcTemplate.query("""
                select t.id from user_playlist_track t
                join user_playlist p on p.id = t.playlist_id
                where p.owner_public_id = ? and t.playlist_id = ?
                order by t.sort_order, t.created_at
                """, (rs, rowNum) -> rs.getString("id"), ownerPublicId, playlistId);
        for (int i = 0; i < ids.size(); i++) {
            jdbcTemplate.update("update user_playlist_track set sort_order = ? where id = ?", i, ids.get(i));
        }
    }

    private void touch(String playlistId) {
        jdbcTemplate.update("update user_playlist set updated_at = ? where id = ?", System.currentTimeMillis(), playlistId);
    }

    private UserPlaylist toPlaylist(java.sql.ResultSet rs) throws java.sql.SQLException {
        return new UserPlaylist(
                rs.getString("id"),
                rs.getString("owner_public_id"),
                rs.getString("name"),
                rs.getString("system_key"),
                rs.getInt("track_count"),
                rs.getLong("created_at"),
                rs.getLong("updated_at")
        );
    }

    private String musicKey(Music music) {
        return String.valueOf(music.platform()) + ":" + String.valueOf(music.id());
    }

    private String writeMusic(Music music) {
        try {
            return objectMapper.writeValueAsString(music);
        } catch (JsonProcessingException e) {
            throw new IllegalArgumentException("Unable to serialize user playlist track", e);
        }
    }

    private Music readMusic(String json) {
        try {
            return objectMapper.readValue(json, Music.class);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Unable to read user playlist track", e);
        }
    }
}
