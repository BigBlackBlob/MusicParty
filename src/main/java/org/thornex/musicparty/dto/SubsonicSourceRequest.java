package org.thornex.musicparty.dto;

public record SubsonicSourceRequest(
        String id,
        String label,
        String baseUrl,
        String username,
        String password,
        String allowedUsers,
        Boolean enabled
) {
}
