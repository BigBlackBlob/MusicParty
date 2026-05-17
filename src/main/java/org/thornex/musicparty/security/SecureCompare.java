package org.thornex.musicparty.security;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;

public final class SecureCompare {
    private SecureCompare() {
    }

    public static boolean equals(String expected, String actual) {
        byte[] expectedBytes = normalize(expected).getBytes(StandardCharsets.UTF_8);
        byte[] actualBytes = normalize(actual).getBytes(StandardCharsets.UTF_8);
        return MessageDigest.isEqual(expectedBytes, actualBytes);
    }

    private static String normalize(String value) {
        return value == null ? "" : value;
    }
}
