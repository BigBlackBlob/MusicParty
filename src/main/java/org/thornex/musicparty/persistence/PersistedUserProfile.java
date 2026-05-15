package org.thornex.musicparty.persistence;

public record PersistedUserProfile(
        String publicId,
        String displayName,
        boolean guest,
        String currentRoomId,
        long createdAt,
        long lastSeenAt
) {
}
