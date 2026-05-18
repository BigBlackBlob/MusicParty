package org.thornex.musicparty.dto;

public record RoomPlaylist(
        String id,
        String roomId,
        String name,
        int trackCount,
        long createdAt,
        long updatedAt
) {}
