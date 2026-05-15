package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.MusicQueueItem;

public record PersistedQueueEntry(
        String id,
        String roomId,
        MusicQueueItem item,
        String enqueuerPublicId,
        String enqueuerNameSnapshot,
        String status,
        int sortOrder,
        long createdAt
) {
}
