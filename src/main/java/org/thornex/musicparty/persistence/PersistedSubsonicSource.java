package org.thornex.musicparty.persistence;

public record PersistedSubsonicSource(
        String id,
        String ownerRoomId,
        String label,
        String baseUrl,
        String username,
        String password,
        String client,
        String apiVersion,
        String allowedUsers,
        boolean enabled,
        boolean system,
        long createdAt,
        long updatedAt
) {
}
