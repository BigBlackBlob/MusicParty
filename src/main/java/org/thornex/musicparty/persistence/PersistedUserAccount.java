package org.thornex.musicparty.persistence;

public record PersistedUserAccount(
        String username,
        String publicId,
        String passwordHash,
        String role,
        boolean enabled,
        long createdAt,
        long updatedAt,
        Long lastLoginAt
) {
}
