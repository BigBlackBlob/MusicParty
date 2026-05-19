package org.thornex.musicparty.persistence;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Repository;
import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.enums.LocalTrackStatus;

import java.util.Comparator;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Collectors;

@Repository
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "false")
public class InMemoryLocalTrackRepository implements LocalTrackRepository {
    private final ConcurrentHashMap<String, LocalTrack> tracks = new ConcurrentHashMap<>();
    private final Set<String> allowedUsers = ConcurrentHashMap.newKeySet();

    @Override
    public Optional<LocalTrack> findById(String id) {
        return Optional.ofNullable(tracks.get(id));
    }

    @Override
    public Optional<LocalTrack> findActiveByOriginalHash(String originalHash) {
        if (originalHash == null || originalHash.isBlank()) return Optional.empty();
        return tracks.values().stream()
                .filter(track -> originalHash.equals(track.originalHash()))
                .filter(track -> track.status() != LocalTrackStatus.DELETED)
                .min(Comparator.comparingLong(LocalTrack::createdAt));
    }

    @Override
    public List<LocalTrack> findAll() {
        return tracks.values().stream()
                .filter(track -> track.status() != LocalTrackStatus.DELETED)
                .sorted(Comparator.comparingLong(LocalTrack::updatedAt).reversed())
                .toList();
    }

    @Override
    public List<LocalTrack> searchCompleted(String keyword, int offset, int limit) {
        String needle = keyword == null ? "" : keyword.trim().toLowerCase();
        return tracks.values().stream()
                .filter(track -> track.status() == LocalTrackStatus.COMPLETED)
                .filter(track -> contains(track.title(), needle)
                        || track.artists().stream().anyMatch(artist -> contains(artist, needle))
                        || contains(track.album(), needle))
                .sorted(Comparator.comparingLong(LocalTrack::updatedAt).reversed())
                .skip(Math.max(0, offset))
                .limit(Math.max(1, limit))
                .toList();
    }

    @Override
    public void upsert(LocalTrack track) {
        tracks.put(track.id(), track);
    }

    @Override
    public void updateStatus(String id, LocalTrackStatus status, String oggPath, String errorMessage, String statusMessage,
                             Integer progressPercent, Long startedAt, Long completedAt, long updatedAt) {
        tracks.computeIfPresent(id, (key, old) -> new LocalTrack(
                old.id(), old.originalHash(), old.originalFileName(),
                status == LocalTrackStatus.COMPLETED ? null : old.sourcePath(),
                old.sourceMimeType(), old.sourceSizeBytes(), old.title(), old.artists(), old.album(),
                old.durationMs(), old.coverPath(), old.coverMimeType(),
                oggPath == null ? old.oggPath() : oggPath, status, errorMessage, statusMessage,
                progressPercent, old.uploadedBy(), old.createdAt(), updatedAt,
                startedAt == null ? old.startedAt() : startedAt,
                completedAt == null ? old.completedAt() : completedAt));
    }

    @Override
    public void markDeleted(String id, long updatedAt) {
        tracks.computeIfPresent(id, (key, old) -> new LocalTrack(
                old.id(), old.originalHash(), old.originalFileName(), old.sourcePath(), old.sourceMimeType(),
                old.sourceSizeBytes(), old.title(), old.artists(), old.album(), old.durationMs(), old.coverPath(),
                old.coverMimeType(), old.oggPath(), LocalTrackStatus.DELETED, old.errorMessage(), old.statusMessage(),
                old.progressPercent(), old.uploadedBy(), old.createdAt(), updatedAt, old.startedAt(), old.completedAt()));
    }

    @Override
    public void deleteLocalReferences(String trackId) {
        // In-memory mode keeps queue/playback state in separate services; JDBC performs durable cleanup.
    }

    @Override
    public Set<String> findAllowedUploadUsers() {
        return allowedUsers.stream().collect(Collectors.toUnmodifiableSet());
    }

    @Override
    public void grantUploadUser(String userName, long now) {
        allowedUsers.add(userName);
    }

    @Override
    public void revokeUploadUser(String userName) {
        allowedUsers.remove(userName);
    }

    private boolean contains(String value, String needle) {
        return value != null && value.toLowerCase().contains(needle);
    }
}
