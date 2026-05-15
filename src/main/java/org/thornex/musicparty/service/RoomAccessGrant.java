package org.thornex.musicparty.service;

public record RoomAccessGrant(
        boolean allowed,
        String roomAccessToken,
        long expiresAt
) {
    public static RoomAccessGrant denied() {
        return new RoomAccessGrant(false, "", 0L);
    }

    public static RoomAccessGrant allowedWithoutToken() {
        return new RoomAccessGrant(true, "", 0L);
    }

    public static RoomAccessGrant allowedWithToken(String roomAccessToken, long expiresAt) {
        return new RoomAccessGrant(true, roomAccessToken, expiresAt);
    }
}
