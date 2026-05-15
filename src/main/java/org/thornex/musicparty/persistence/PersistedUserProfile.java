package org.thornex.musicparty.persistence;

public record PersistedUserProfile(
        String publicId,
        String displayName,
        boolean guest,
        long createdAt,
        long lastSeenAt
) {
}
