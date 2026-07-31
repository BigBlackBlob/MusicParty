package org.thornex.musicparty.service;

public record AccountSession(
        String sessionToken,
        String publicId,
        String username,
        String displayName,
        String role,
        boolean guest,
        boolean enabled,
        Long lastLoginAt
) {
    public boolean admin() {
        return "PLATFORM_ADMIN".equals(role) || "ADMIN".equals(role);
    }
}
