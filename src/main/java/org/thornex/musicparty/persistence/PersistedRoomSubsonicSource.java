package org.thornex.musicparty.persistence;

public record PersistedRoomSubsonicSource(
        String roomId,
        String sourceId,
        boolean enabled,
        String displayLabel,
        String allowedUsers,
        int sortOrder,
        long createdAt,
        long updatedAt
) {
}
