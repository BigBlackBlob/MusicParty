package org.thornex.musicparty.service;

public record AccountSession(
        String sessionToken,
        String publicId,
        String username,
        String role,
        boolean guest
) {
    public boolean admin() {
        return "ADMIN".equals(role);
    }
}
