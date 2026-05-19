package org.thornex.musicparty.dto;

import org.thornex.musicparty.enums.LocalTrackStatus;

import java.util.List;

public record LocalTrack(
        String id,
        String originalHash,
        String originalFileName,
        String sourcePath,
        String sourceMimeType,
        long sourceSizeBytes,
        String title,
        List<String> artists,
        String album,
        long durationMs,
        String coverPath,
        String coverMimeType,
        String oggPath,
        LocalTrackStatus status,
        String errorMessage,
        String statusMessage,
        Integer progressPercent,
        String uploadedBy,
        long createdAt,
        long updatedAt,
        Long startedAt,
        Long completedAt
) {
    public Music toMusic() {
        return new Music(id, title, artists, durationMs, "local",
                coverPath == null || coverPath.isBlank() ? "" : "/api/local/cover/" + id);
    }

    public PlayableMusic toPlayableMusic() {
        return new PlayableMusic(id, title, artists, durationMs, "local",
                "/api/local/media/" + id,
                coverPath == null || coverPath.isBlank() ? "" : "/api/local/cover/" + id,
                false);
    }
}
