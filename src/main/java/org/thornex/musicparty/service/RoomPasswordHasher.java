package org.thornex.musicparty.service;

import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;

final class RoomPasswordHasher {
    private static final BCryptPasswordEncoder ENCODER = new BCryptPasswordEncoder();

    private RoomPasswordHasher() {
    }

    static String hash(String rawPassword) {
        return ENCODER.encode(rawPassword);
    }

    static boolean matches(String rawPassword, String passwordHash) {
        return passwordHash != null && ENCODER.matches(rawPassword, passwordHash);
    }
}
