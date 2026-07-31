package org.thornex.musicparty.persistence;

public record PersistedRoomMembership(String roomId, String publicId, String role, long createdAt, long updatedAt) {
    public boolean owner() { return "OWNER".equals(role); }
}
