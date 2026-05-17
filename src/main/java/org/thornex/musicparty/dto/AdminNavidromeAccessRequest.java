package org.thornex.musicparty.dto;

public record AdminNavidromeAccessRequest(
        String adminPassword,
        String roomId,
        String userName
) {
}
