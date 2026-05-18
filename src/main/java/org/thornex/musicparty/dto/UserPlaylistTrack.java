package org.thornex.musicparty.dto;

public record UserPlaylistTrack(
        String id,
        String playlistId,
        Music music,
        int sortOrder,
        long createdAt
) {
}
