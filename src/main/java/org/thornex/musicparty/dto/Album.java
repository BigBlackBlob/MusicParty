package org.thornex.musicparty.dto;

public record Album(
        String id,
        String name,
        String artistName,
        String coverUrl,
        int trackCount,
        String platform
) {
}
