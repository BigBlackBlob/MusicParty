package org.thornex.musicparty.dto;

public record AdminNavidromeAccessRequest(
        String adminPassword,
        String sessionToken,
        String roomId,
        String userName
) {
}
