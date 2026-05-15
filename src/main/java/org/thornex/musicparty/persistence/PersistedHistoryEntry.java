package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.Music;

public record PersistedHistoryEntry(
        String id,
        String roomId,
        Music music,
        String enqueuerPublicId,
        long playedAt
) {
}
