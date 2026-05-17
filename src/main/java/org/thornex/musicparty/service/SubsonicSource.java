package org.thornex.musicparty.service;

import org.springframework.util.StringUtils;

public record SubsonicSource(
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
    public String platformId() {
        return "subsonic-" + id;
    }

    public boolean isConfigured() {
        return StringUtils.hasText(baseUrl)
                && StringUtils.hasText(username)
                && StringUtils.hasText(password);
    }

    public SubsonicSource withUpdatedConfig(String nextLabel,
                                            String nextBaseUrl,
                                            String nextUsername,
                                            String nextPassword,
                                            String nextAllowedUsers,
                                            boolean nextEnabled) {
        return new SubsonicSource(
                id,
                ownerRoomId,
                StringUtils.hasText(nextLabel) ? nextLabel.trim() : label,
                StringUtils.hasText(nextBaseUrl) ? nextBaseUrl.trim() : baseUrl,
                StringUtils.hasText(nextUsername) ? nextUsername.trim() : username,
                StringUtils.hasText(nextPassword) ? nextPassword : password,
                client,
                apiVersion,
                StringUtils.hasText(nextAllowedUsers) ? nextAllowedUsers.trim() : allowedUsers,
                nextEnabled,
                system,
                createdAt,
                System.currentTimeMillis()
        );
    }
}
