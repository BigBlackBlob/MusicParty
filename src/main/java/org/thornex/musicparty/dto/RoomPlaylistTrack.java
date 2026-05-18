package org.thornex.musicparty.dto;

public record RoomPlaylistTrack(
        String id,
        String playlistId,
        Music music,
        int sortOrder,
        long createdAt
) {}
