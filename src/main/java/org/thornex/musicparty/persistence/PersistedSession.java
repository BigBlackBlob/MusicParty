package org.thornex.musicparty.persistence;

public record PersistedSession(
        String sessionTokenHash,
        String publicId,
        long createdAt,
        long lastSeenAt
) {
}
