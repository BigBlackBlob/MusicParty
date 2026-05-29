package org.thornex.musicparty.persistence;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.RoomPlaylistTrack;

import java.sql.ResultSet;
import java.sql.SQLException;
import java.util.List;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcQueueRepository implements QueueRepository {

    private static final TypeReference<MusicQueueItem> QUEUE_ITEM_TYPE = new TypeReference<>() {};
    private static final TypeReference<Music> MUSIC_TYPE = new TypeReference<>() {};

    private final JdbcTemplate jdbcTemplate;
    private final ObjectMapper objectMapper;

    @Override
    public List<MusicQueueItem> loadQueue(String roomId) {
        return jdbcTemplate.query("""
                select music_json
                from room_queue
                where room_id = ?
                order by sort_order asc
                """, queueItemRowMapper(), roomId);
    }

    @Override
    @Transactional
    public void replaceQueue(String roomId, List<MusicQueueItem> queueItems) {
        jdbcTemplate.update("delete from room_queue where room_id = ?", roomId);
        for (int i = 0; i < queueItems.size(); i++) {
            MusicQueueItem item = queueItems.get(i);
            jdbcTemplate.update("""
                    insert into room_queue(id, room_id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order, created_at)
                    values (?, ?, ?, ?, ?, ?, ?, ?)
                    """,
                    item.queueId(),
                    roomId,
                    writeJson(item),
                    item.enqueuedBy().publicId(),
                    item.enqueuedBy().name(),
                    item.status().name(),
                    i,
                    System.currentTimeMillis());
        }
    }

    @Override
    public List<PersistedHistoryEntry> loadHistory(String roomId, int limit) {
        return jdbcTemplate.query("""
                select id, room_id, music_json, enqueuer_public_id, played_at
                from room_history
                where room_id = ?
                order by played_at desc
                limit ?
                """, historyRowMapper(), roomId, limit);
    }

    @Override
    public int countHistoryTracks(String roomId) {
        Integer count = jdbcTemplate.queryForObject(
                "select count(1) from room_history_track where room_id = ?",
                Integer.class,
                roomId
        );
        return count == null ? 0 : count;
    }

    @Override
    public List<RoomPlaylistTrack> listHistoryTracks(String roomId, int offset, int limit) {
        int safeOffset = Math.max(0, offset);
        int safeLimit = Math.max(1, Math.min(500, limit));
        return jdbcTemplate.query("""
                select room_id, platform, music_id, music_json, last_played_at
                from room_history_track
                where room_id = ?
                order by last_played_at desc
                limit ? offset ?
                """, (rs, rowNum) -> new RoomPlaylistTrack(
                historyTrackId(rs.getString("platform"), rs.getString("music_id")),
                "__room_history__",
                readJson(rs.getString("music_json"), MUSIC_TYPE),
                safeOffset + rowNum,
                rs.getLong("last_played_at")
        ), roomId, safeLimit, safeOffset);
    }

    @Override
    public List<Music> listHistoryMusics(String roomId, int limit) {
        return listHistoryTracks(roomId, 0, limit).stream()
                .map(RoomPlaylistTrack::music)
                .toList();
    }

    @Override
    @Transactional
    public void appendHistory(PersistedHistoryEntry historyEntry) {
        jdbcTemplate.update("""
                insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at)
                values (?, ?, ?, ?, ?)
                """,
                historyEntry.id(),
                historyEntry.roomId(),
                writeJson(historyEntry.music()),
                historyEntry.enqueuerPublicId(),
                historyEntry.playedAt());
        upsertHistoryTrack(historyEntry);
    }

    @Override
    @Transactional
    public void replaceHistory(String roomId, List<Music> historyItems) {
        jdbcTemplate.update("delete from room_history where room_id = ?", roomId);
        for (int i = 0; i < historyItems.size(); i++) {
            Music music = historyItems.get(i);
            jdbcTemplate.update("""
                    insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at)
                    values (?, ?, ?, ?, ?)
                    """,
                    roomId + "-history-" + i,
                    roomId,
                    writeJson(music),
                    null,
                    System.currentTimeMillis() - i);
        }
    }

    @Override
    @Transactional
    public void deleteRoomData(String roomId) {
        jdbcTemplate.update("delete from room_queue where room_id = ?", roomId);
        jdbcTemplate.update("delete from room_history where room_id = ?", roomId);
        jdbcTemplate.update("delete from room_history_track where room_id = ?", roomId);
    }

    private void upsertHistoryTrack(PersistedHistoryEntry historyEntry) {
        Music music = historyEntry.music();
        if (music == null || music.platform() == null || music.platform().isBlank() || music.id() == null || music.id().isBlank()) {
            return;
        }
        jdbcTemplate.update("""
                insert into room_history_track(room_id, platform, music_id, music_json, play_count, first_played_at, last_played_at)
                values (?, ?, ?, ?, 1, ?, ?)
                on conflict(room_id, platform, music_id) do update set
                    music_json = case
                        when excluded.last_played_at >= room_history_track.last_played_at then excluded.music_json
                        else room_history_track.music_json
                    end,
                    play_count = room_history_track.play_count + 1,
                    first_played_at = min(room_history_track.first_played_at, excluded.first_played_at),
                    last_played_at = max(room_history_track.last_played_at, excluded.last_played_at)
                """,
                historyEntry.roomId(),
                music.platform(),
                music.id(),
                writeJson(music),
                historyEntry.playedAt(),
                historyEntry.playedAt());
    }

    private String historyTrackId(String platform, String musicId) {
        return String.valueOf(platform) + ":" + String.valueOf(musicId);
    }

    private RowMapper<MusicQueueItem> queueItemRowMapper() {
        return (rs, rowNum) -> readJson(rs.getString("music_json"), QUEUE_ITEM_TYPE);
    }

    private RowMapper<PersistedHistoryEntry> historyRowMapper() {
        return new PersistedHistoryEntryRowMapper();
    }

    private String writeJson(Object value) {
        try {
            return objectMapper.writeValueAsString(value);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Failed to serialize value for SQLite persistence", e);
        }
    }

    private <T> T readJson(String json, TypeReference<T> typeReference) {
        try {
            return objectMapper.readValue(json, typeReference);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Failed to deserialize value from SQLite persistence", e);
        }
    }

    private final class PersistedHistoryEntryRowMapper implements RowMapper<PersistedHistoryEntry> {
        @Override
        public PersistedHistoryEntry mapRow(ResultSet rs, int rowNum) throws SQLException {
            return new PersistedHistoryEntry(
                    rs.getString("id"),
                    rs.getString("room_id"),
                    readJson(rs.getString("music_json"), MUSIC_TYPE),
                    rs.getString("enqueuer_public_id"),
                    rs.getLong("played_at")
            );
        }
    }
}
