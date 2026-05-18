package org.thornex.musicparty.dto;

public record UserPlaylist(
        String id,
        String ownerPublicId,
        String name,
        String systemKey,
        int trackCount,
        long createdAt,
        long updatedAt
) {
}
