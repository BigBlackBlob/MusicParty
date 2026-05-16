package org.thornex.musicparty.persistence;

import java.util.Map;
import java.util.Optional;

public interface UserProfileRepository {
    void upsertProfile(PersistedUserProfile profile);
    Optional<PersistedUserProfile> findByPublicId(String publicId);
    Map<String, String> findBindingsByPublicId(String publicId);
    void replaceBindings(String publicId, Map<String, String> bindings);
    void upsertSession(PersistedSession session);
    Optional<PersistedSession> findSessionByHash(String sessionTokenHash);
    void moveUsersToRoom(String fromRoomId, String toRoomId);
}
