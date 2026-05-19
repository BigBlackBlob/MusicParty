package org.thornex.musicparty.dto;

import java.util.List;

public record LocalTrackUpdateRequest(
        String title,
        List<String> artists,
        String album,
        String adminPassword,
        String token
) {
}
