package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.enums.LocalTrackStatus;

import java.util.List;
import java.util.Optional;
import java.util.Set;

public interface LocalTrackRepository {
    Optional<LocalTrack> findById(String id);
    Optional<LocalTrack> findActiveByOriginalHash(String originalHash);
    List<LocalTrack> findAll();
    List<LocalTrack> searchCompleted(String keyword, int offset, int limit);
    void upsert(LocalTrack track);
    void updateStatus(String id, LocalTrackStatus status, String oggPath, String errorMessage, String statusMessage,
                      Integer progressPercent, Long startedAt, Long completedAt, long updatedAt);
    void markDeleted(String id, long updatedAt);
    void deleteLocalReferences(String trackId);
    Set<String> findAllowedUploadUsers();
    void grantUploadUser(String userName, long now);
    void revokeUploadUser(String userName);
}
