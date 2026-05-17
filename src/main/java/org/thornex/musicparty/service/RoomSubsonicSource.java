package org.thornex.musicparty.service;

import org.springframework.util.StringUtils;

public record RoomSubsonicSource(
        String roomId,
        SubsonicSource source,
        boolean enabled,
        String displayLabel,
        String allowedUsers,
        int sortOrder,
        long createdAt,
        long updatedAt
) {
    public String id() {
        return source.id();
    }

    public String label() {
        return StringUtils.hasText(displayLabel) ? displayLabel : source.label();
    }

    public String platformId() {
        return source.platformId();
    }

    public boolean system() {
        return source.system();
    }

    public boolean isConfigured() {
        return source.isConfigured();
    }

    public boolean active() {
        return enabled && source.enabled() && source.isConfigured();
    }
}
