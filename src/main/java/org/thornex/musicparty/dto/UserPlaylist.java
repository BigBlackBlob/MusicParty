package org.thornex.musicparty.dto;

public record UserPlaylist(
        String id,
        String ownerPublicId,
        String name,
        int trackCount,
        long createdAt,
        long updatedAt
) {
}
