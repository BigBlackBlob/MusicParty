package org.thornex.musicparty.persistence;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.beans.factory.annotation.Autowired;
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
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

@Repository
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcQueueRepository implements QueueRepository {

    private static final TypeReference<MusicQueueItem> QUEUE_ITEM_TYPE = new TypeReference<>() {};
    private static final TypeReference<Music> MUSIC_TYPE = new TypeReference<>() {};

    private final JdbcTemplate jdbcTemplate;
    private final ObjectMapper objectMapper;
    private final MeterRegistry meterRegistry;

    public JdbcQueueRepository(JdbcTemplate jdbcTemplate, ObjectMapper objectMapper) {
        this(jdbcTemplate, objectMapper, new io.micrometer.core.instrument.simple.SimpleMeterRegistry());
    }

    @Autowired
    public JdbcQueueRepository(JdbcTemplate jdbcTemplate, ObjectMapper objectMapper, MeterRegistry meterRegistry) {
        this.jdbcTemplate = jdbcTemplate;
        this.objectMapper = objectMapper;
        this.meterRegistry = meterRegistry;
    }

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
    @Transactional
    public void synchronizeQueue(String roomId, List<MusicQueueItem> queueItems) {
        Timer.Sample sample = Timer.start(meterRegistry);
        List<MusicQueueItem> desired = queueItems == null ? List.of() : queueItems;
        Set<String> existingIds = Set.copyOf(jdbcTemplate.queryForList(
                "select id from room_queue where room_id = ?", String.class, roomId));
        Map<String, Integer> desiredPositions = java.util.stream.IntStream.range(0, desired.size())
                .boxed()
                .collect(Collectors.toMap(index -> desired.get(index).queueId(), index -> index));

        List<String> removedIds = existingIds.stream()
                .filter(id -> !desiredPositions.containsKey(id))
                .toList();
        if (!removedIds.isEmpty()) {
            jdbcTemplate.batchUpdate("delete from room_queue where room_id = ? and id = ?", removedIds,
                    removedIds.size(), (ps, id) -> {
                        ps.setString(1, roomId);
                        ps.setString(2, id);
                    });
        }

        List<MusicQueueItem> inserted = desired.stream()
                .filter(item -> !existingIds.contains(item.queueId()))
                .toList();
        if (!inserted.isEmpty()) {
            jdbcTemplate.batchUpdate("""
                    insert into room_queue(id, room_id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order, created_at)
                    values (?, ?, ?, ?, ?, ?, ?, ?)
                    """, inserted, inserted.size(), (ps, item) -> {
                        ps.setString(1, item.queueId());
                        ps.setString(2, roomId);
                        ps.setString(3, writeJson(item));
                        ps.setString(4, item.enqueuedBy().publicId());
                        ps.setString(5, item.enqueuedBy().name());
                        ps.setString(6, item.status().name());
                        ps.setInt(7, desiredPositions.get(item.queueId()));
                        ps.setLong(8, System.currentTimeMillis());
                    });
        }

        List<MusicQueueItem> updated = desired.stream()
                .filter(item -> existingIds.contains(item.queueId()))
                .toList();
        if (!updated.isEmpty()) {
            jdbcTemplate.batchUpdate("""
                    update room_queue
                    set music_json = ?, enqueuer_public_id = ?, enqueuer_name_snapshot = ?, status = ?, sort_order = ?
                    where room_id = ? and id = ?
                    """, updated, updated.size(), (ps, item) -> {
                        ps.setString(1, writeJson(item));
                        ps.setString(2, item.enqueuedBy().publicId());
                        ps.setString(3, item.enqueuedBy().name());
                        ps.setString(4, item.status().name());
                        ps.setInt(5, desiredPositions.get(item.queueId()));
                        ps.setString(6, roomId);
                        ps.setString(7, item.queueId());
                    });
        }
        meterRegistry.counter("musicparty.queue.persistence.rows", "operation", "delete").increment(removedIds.size());
        meterRegistry.counter("musicparty.queue.persistence.rows", "operation", "insert").increment(inserted.size());
        meterRegistry.counter("musicparty.queue.persistence.rows", "operation", "update").increment(updated.size());
        meterRegistry.summary("musicparty.queue.length").record(desired.size());
        sample.stop(meterRegistry.timer("musicparty.queue.persistence.transaction"));
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
