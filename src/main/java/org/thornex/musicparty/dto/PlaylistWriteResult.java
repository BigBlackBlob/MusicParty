package org.thornex.musicparty.dto;

import java.util.List;

public record PlaylistWriteResult(
        int addedCount,
        int skippedCount,
        List<UserPlaylistTrack> tracks
) {
}
