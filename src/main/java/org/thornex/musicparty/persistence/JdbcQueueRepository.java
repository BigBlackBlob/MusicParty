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
