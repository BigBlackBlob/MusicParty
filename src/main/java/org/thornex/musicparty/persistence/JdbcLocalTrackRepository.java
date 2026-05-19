package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;
import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.enums.LocalTrackStatus;

import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcLocalTrackRepository implements LocalTrackRepository {

    private static final RowMapper<LocalTrack> ROW_MAPPER = (rs, rowNum) -> new LocalTrack(
            rs.getString("id"),
            rs.getString("original_hash"),
            rs.getString("original_file_name"),
            rs.getString("source_path"),
            rs.getString("source_mime_type"),
            rs.getLong("source_size_bytes"),
            rs.getString("title"),
            splitArtists(rs.getString("artists")),
            rs.getString("album"),
            rs.getLong("duration_ms"),
            rs.getString("cover_path"),
            rs.getString("cover_mime_type"),
            rs.getString("ogg_path"),
            LocalTrackStatus.valueOf(rs.getString("status")),
            rs.getString("error_message"),
            rs.getString("status_message"),
            (Integer) rs.getObject("progress_percent"),
            rs.getString("uploaded_by"),
            rs.getLong("created_at"),
            rs.getLong("updated_at"),
            (Long) rs.getObject("started_at"),
            (Long) rs.getObject("completed_at")
    );

    private final JdbcTemplate jdbcTemplate;

    @Override
    public Optional<LocalTrack> findById(String id) {
        return jdbcTemplate.query("select * from local_track where id = ?", ROW_MAPPER, id).stream().findFirst();
    }

    @Override
    public Optional<LocalTrack> findActiveByOriginalHash(String originalHash) {
        if (originalHash == null || originalHash.isBlank()) return Optional.empty();
        return jdbcTemplate.query("""
                select * from local_track
                where original_hash = ? and status <> 'DELETED'
                order by created_at asc
                limit 1
                """, ROW_MAPPER, originalHash).stream().findFirst();
    }

    @Override
    public List<LocalTrack> findAll() {
        return jdbcTemplate.query("select * from local_track where status <> 'DELETED' order by updated_at desc", ROW_MAPPER);
    }

    @Override
    public List<LocalTrack> searchCompleted(String keyword, int offset, int limit) {
        String like = "%" + (keyword == null ? "" : keyword.trim().toLowerCase()) + "%";
        return jdbcTemplate.query("""
                select * from local_track
                where status = 'COMPLETED'
                  and (lower(title) like ? or lower(artists) like ? or lower(coalesce(album, '')) like ?)
                order by updated_at desc
                limit ? offset ?
                """, ROW_MAPPER, like, like, like, Math.max(1, limit), Math.max(0, offset));
    }

    @Override
    public void upsert(LocalTrack track) {
        jdbcTemplate.update("""
                insert into local_track(
                    id, original_hash, original_file_name, source_path, source_mime_type, source_size_bytes,
                    title, artists, album, duration_ms, cover_path, cover_mime_type, ogg_path, status,
                    error_message, status_message, progress_percent, uploaded_by, created_at, updated_at,
                    started_at, completed_at
                )
                values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                on conflict(id) do update set
                  original_hash = excluded.original_hash,
                  original_file_name = excluded.original_file_name,
                  source_path = excluded.source_path,
                  source_mime_type = excluded.source_mime_type,
                  source_size_bytes = excluded.source_size_bytes,
                  title = excluded.title,
                  artists = excluded.artists,
                  album = excluded.album,
                  duration_ms = excluded.duration_ms,
                  cover_path = excluded.cover_path,
                  cover_mime_type = excluded.cover_mime_type,
                  ogg_path = excluded.ogg_path,
                  status = excluded.status,
                  error_message = excluded.error_message,
                  status_message = excluded.status_message,
                  progress_percent = excluded.progress_percent,
                  uploaded_by = excluded.uploaded_by,
                  updated_at = excluded.updated_at,
                  started_at = excluded.started_at,
                  completed_at = excluded.completed_at
                """,
                track.id(), track.originalHash(), track.originalFileName(), track.sourcePath(), track.sourceMimeType(),
                track.sourceSizeBytes(), track.title(), joinArtists(track.artists()), track.album(), track.durationMs(),
                track.coverPath(), track.coverMimeType(), track.oggPath(), track.status().name(), track.errorMessage(),
                track.statusMessage(), track.progressPercent(), track.uploadedBy(), track.createdAt(), track.updatedAt(),
                track.startedAt(), track.completedAt());
    }

    @Override
    public void updateStatus(String id, LocalTrackStatus status, String oggPath, String errorMessage, String statusMessage,
                             Integer progressPercent, Long startedAt, Long completedAt, long updatedAt) {
        jdbcTemplate.update("""
                update local_track
                set status = ?,
                    ogg_path = coalesce(?, ogg_path),
                    source_path = case when ? = 'COMPLETED' then null else source_path end,
                    error_message = ?,
                    status_message = ?,
                    progress_percent = ?,
                    started_at = coalesce(?, started_at),
                    completed_at = coalesce(?, completed_at),
                    updated_at = ?
                where id = ?
                """, status.name(), oggPath, status.name(), errorMessage, statusMessage, progressPercent,
                startedAt, completedAt, updatedAt, id);
    }

    @Override
    public void markDeleted(String id, long updatedAt) {
        jdbcTemplate.update("update local_track set status = 'DELETED', updated_at = ? where id = ?", updatedAt, id);
    }

    @Override
    public void deleteLocalReferences(String trackId) {
        String idNeedle = "\"id\":\"" + trackId.replace("\"", "\\\"") + "\"";
        String platformNeedle = "\"platform\":\"local\"";
        jdbcTemplate.update("delete from room_queue where music_json like ? and music_json like ?", "%" + platformNeedle + "%", "%" + idNeedle + "%");
        jdbcTemplate.update("delete from room_history where music_json like ? and music_json like ?", "%" + platformNeedle + "%", "%" + idNeedle + "%");
        jdbcTemplate.update("delete from user_playlist_track where music_json like ? and music_json like ?", "%" + platformNeedle + "%", "%" + idNeedle + "%");
        jdbcTemplate.update("delete from room_playlist_track where music_json like ? and music_json like ?", "%" + platformNeedle + "%", "%" + idNeedle + "%");
        jdbcTemplate.update("update room_playback_state set current_music_json = null where current_music_json like ? and current_music_json like ?",
                "%" + platformNeedle + "%", "%" + idNeedle + "%");
    }

    @Override
    public Set<String> findAllowedUploadUsers() {
        return jdbcTemplate.query("select user_name from local_upload_access order by user_name", (rs, rowNum) -> rs.getString("user_name"))
                .stream()
                .collect(Collectors.toUnmodifiableSet());
    }

    @Override
    public void grantUploadUser(String userName, long now) {
        jdbcTemplate.update("""
                insert into local_upload_access(user_name, created_at, updated_at)
                values (?, ?, ?)
                on conflict(user_name) do update set updated_at = excluded.updated_at
                """, userName, now, now);
    }

    @Override
    public void revokeUploadUser(String userName) {
        jdbcTemplate.update("delete from local_upload_access where user_name = ?", userName);
    }

    private static String joinArtists(List<String> artists) {
        if (artists == null || artists.isEmpty()) return "Unknown";
        return artists.stream().filter(v -> v != null && !v.isBlank()).collect(Collectors.joining("; "));
    }

    private static List<String> splitArtists(String artists) {
        if (artists == null || artists.isBlank()) return List.of("Unknown");
        return Arrays.stream(artists.split(";"))
                .map(String::trim)
                .filter(v -> !v.isBlank())
                .toList();
    }
}
