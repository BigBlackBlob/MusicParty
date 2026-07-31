package org.thornex.musicparty.persistence;

public record PersistedRoomInvite(
        String id, String roomId, String createdByPublicId, String secretHash, String label,
        long expiresAt, int maxUses, Long usedAt, String usedByPublicId, Long revokedAt, long createdAt
) {
    public boolean active(long now) { return revokedAt == null && usedAt == null && expiresAt >= now; }
}
