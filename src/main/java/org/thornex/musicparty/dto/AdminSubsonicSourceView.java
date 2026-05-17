package org.thornex.musicparty.dto;

public record AdminSubsonicSourceView(
        String id,
        String platformId,
        String label,
        String baseUrl,
        String username,
        String allowedUsers,
        boolean enabled,
        boolean active,
        boolean system,
        int sortOrder,
        long updatedAt
) {
}
