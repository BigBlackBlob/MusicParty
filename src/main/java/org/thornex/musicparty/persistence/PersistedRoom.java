package org.thornex.musicparty.persistence;

public record PersistedRoom(
        String id,
        String name,
        String ownerPublicId,
        String visibility,
        String passwordHash,
        int passwordVersion,
        boolean system,
        long createdAt,
        long lastActiveAt,
        Long deletedAt
) {
}
