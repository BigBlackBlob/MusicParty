package org.thornex.musicparty.persistence;

import java.util.Optional;

public interface UserProfileRepository {
    void upsertProfile(PersistedUserProfile profile);
    Optional<PersistedUserProfile> findByPublicId(String publicId);
    void upsertSession(PersistedSession session);
    Optional<PersistedSession> findSessionByHash(String sessionTokenHash);
    void moveUsersToRoom(String fromRoomId, String toRoomId);
}
