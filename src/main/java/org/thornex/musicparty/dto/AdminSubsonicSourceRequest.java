package org.thornex.musicparty.dto;

public record AdminSubsonicSourceRequest(
        String adminPassword,
        String roomId,
        String id,
        String label,
        String baseUrl,
        String username,
        String password,
        String allowedUsers,
        Boolean enabled,
        Integer sortOrder
) {
}
