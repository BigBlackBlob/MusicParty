package org.thornex.musicparty.persistence;

import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryUserProfileRepository implements UserProfileRepository {

    private final Map<String, PersistedUserProfile> profiles = new ConcurrentHashMap<>();
    private final Map<String, PersistedSession> sessions = new ConcurrentHashMap<>();

    @Override
    public void upsertProfile(PersistedUserProfile profile) {
        profiles.put(profile.publicId(), profile);
    }

    @Override
    public Optional<PersistedUserProfile> findByPublicId(String publicId) {
        return Optional.ofNullable(profiles.get(publicId));
    }

    @Override
    public void upsertSession(PersistedSession session) {
        sessions.put(session.sessionTokenHash(), session);
    }

    @Override
    public Optional<PersistedSession> findSessionByHash(String sessionTokenHash) {
        return Optional.ofNullable(sessions.get(sessionTokenHash));
    }
}
