package org.thornex.musicparty.persistence;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;
import org.thornex.musicparty.dto.PlayableMusic;

import java.util.List;
import java.util.Optional;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcPlaybackStateRepository implements PlaybackStateRepository {

    private static final TypeReference<PlayableMusic> PLAYABLE_MUSIC_TYPE = new TypeReference<>() {};
    private final JdbcTemplate jdbcTemplate;
    private final ObjectMapper objectMapper;

    private final RowMapper<PersistedPlaybackState> rowMapper = (rs, rowNum) -> new PersistedPlaybackState(
            rs.getString("room_id"),
            readPlayableMusic(rs.getString("current_music_json")),
            rs.getString("current_enqueuer_id"),
            rs.getString("current_enqueuer_name"),
            rs.getLong("position_anchor"),
            rs.getLong("timestamp_anchor"),
            rs.getLong("position_updated_at"),
            rs.getBoolean("is_shuffle"),
            rs.getBoolean("is_paused"),
            rs.getBoolean("is_pause_locked"),
            rs.getBoolean("is_skip_locked"),
            rs.getBoolean("is_shuffle_locked"),
            rs.getBoolean("is_loading"),
            rs.getLong("play_epoch"),
            rs.getLong("state_version"),
            rs.getLong("last_persisted_at")
    );

    @Override
    public Optional<PersistedPlaybackState> findByRoomId(String roomId) {
        List<PersistedPlaybackState> rows = jdbcTemplate.query("""
                select room_id, current_music_json, current_enqueuer_id, current_enqueuer_name,
                       position_anchor, timestamp_anchor, position_updated_at,
                       is_shuffle, is_paused, is_pause_locked, is_skip_locked, is_shuffle_locked,
                       is_loading, play_epoch, state_version, last_persisted_at
                from room_playback_state
                where room_id = ?
                """, rowMapper, roomId);
        return rows.stream().findFirst();
    }

    @Override
    public void upsert(PersistedPlaybackState state) {
        jdbcTemplate.update("""
                insert into room_playback_state(
                    room_id, current_music_json, current_enqueuer_id, current_enqueuer_name,
                    position_anchor, timestamp_anchor, position_updated_at,
                    is_shuffle, is_paused, is_pause_locked, is_skip_locked, is_shuffle_locked,
                    is_loading, play_epoch, state_version, last_persisted_at
                )
                values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                on conflict(room_id) do update set
                    current_music_json = excluded.current_music_json,
                    current_enqueuer_id = excluded.current_enqueuer_id,
                    current_enqueuer_name = excluded.current_enqueuer_name,
                    position_anchor = excluded.position_anchor,
                    timestamp_anchor = excluded.timestamp_anchor,
                    position_updated_at = excluded.position_updated_at,
                    is_shuffle = excluded.is_shuffle,
                    is_paused = excluded.is_paused,
                    is_pause_locked = excluded.is_pause_locked,
                    is_skip_locked = excluded.is_skip_locked,
                    is_shuffle_locked = excluded.is_shuffle_locked,
                    is_loading = excluded.is_loading,
                    play_epoch = excluded.play_epoch,
                    state_version = excluded.state_version,
                    last_persisted_at = excluded.last_persisted_at
                """,
                state.roomId(),
                writePlayableMusic(state.currentMusic()),
                state.currentEnqueuerId(),
                state.currentEnqueuerName(),
                state.positionAnchor(),
                state.timestampAnchor(),
                state.positionUpdatedAt(),
                state.shuffle(),
                state.paused(),
                state.pauseLocked(),
                state.skipLocked(),
                state.shuffleLocked(),
                state.loading(),
                state.playEpoch(),
                state.stateVersion(),
                state.lastPersistedAt());
    }

    @Override
    public void delete(String roomId) {
        jdbcTemplate.update("delete from room_playback_state where room_id = ?", roomId);
    }

    private String writePlayableMusic(PlayableMusic music) {
        if (music == null) {
            return null;
        }
        try {
            return objectMapper.writeValueAsString(music);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Failed to serialize playable music", e);
        }
    }

    private PlayableMusic readPlayableMusic(String json) {
        if (json == null || json.isBlank()) {
            return null;
        }
        try {
            return objectMapper.readValue(json, PLAYABLE_MUSIC_TYPE);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("Failed to deserialize playable music", e);
        }
    }
}
