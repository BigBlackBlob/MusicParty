package org.thornex.musicparty.persistence;

import java.util.List;
import java.util.Optional;

public interface RoomAccessRepository {
    void upsertMembership(PersistedRoomMembership membership);
    Optional<PersistedRoomMembership> findMembership(String roomId, String publicId);
    List<PersistedRoomMembership> listMemberships(String roomId);
    int countOwners(String roomId);
    void deleteMembership(String roomId, String publicId);
    void createInvite(PersistedRoomInvite invite);
    Optional<PersistedRoomInvite> findInviteById(String inviteId);
    Optional<PersistedRoomInvite> findInviteBySecretHash(String secretHash);
    List<PersistedRoomInvite> listInvites(String roomId);
    boolean consumeInvite(String inviteId, String publicId, long usedAt);
    boolean revokeInvite(String inviteId, long revokedAt);
    boolean revokeInvite(String roomId, String inviteId, long revokedAt);
}
