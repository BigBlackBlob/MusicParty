package org.thornex.musicparty.dto;

public record RoomInfo(
        String roomId,
        String name,
        String creatorPublicId,
        long createdAt,
        boolean privateRoom,
        boolean system,
        int onlineCount
) {
}
