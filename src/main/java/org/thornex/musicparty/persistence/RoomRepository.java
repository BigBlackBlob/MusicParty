package org.thornex.musicparty.persistence;

import java.util.List;
import java.util.Optional;

public interface RoomRepository {
    List<PersistedRoom> findAllActive();
    List<PersistedRoom> findLobbyRooms(String requesterPublicId);
    Optional<PersistedRoom> findById(String roomId);
    void upsert(PersistedRoom room);
    void touch(String roomId, long lastActiveAt);
    void softDelete(String roomId, long deletedAt);
}
