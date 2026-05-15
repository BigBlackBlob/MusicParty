package org.thornex.musicparty.persistence;

import java.util.Optional;

public interface UserProfileRepository {
    void upsertProfile(PersistedUserProfile profile);
    Optional<PersistedUserProfile> findByPublicId(String publicId);
    void upsertSession(PersistedSession session);
}
