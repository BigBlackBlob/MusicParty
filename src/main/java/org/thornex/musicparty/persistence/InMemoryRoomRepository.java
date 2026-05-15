package org.thornex.musicparty.persistence;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryRoomRepository implements RoomRepository {

    private final Map<String, PersistedRoom> rooms = new ConcurrentHashMap<>();

    @Override
    public List<PersistedRoom> findAllActive() {
        return rooms.values().stream()
                .filter(room -> room.deletedAt() == null)
                .toList();
    }

    @Override
    public List<PersistedRoom> findLobbyRooms(String requesterPublicId) {
        return rooms.values().stream()
                .filter(room -> room.deletedAt() == null)
                .filter(room -> room.system()
                        || "PUBLIC".equals(room.visibility())
                        || (requesterPublicId != null && requesterPublicId.equals(room.ownerPublicId())))
                .sorted((left, right) -> {
                    if (left.system() != right.system()) {
                        return left.system() ? -1 : 1;
                    }
                    int activeCompare = Long.compare(right.lastActiveAt(), left.lastActiveAt());
                    if (activeCompare != 0) {
                        return activeCompare;
                    }
                    return Long.compare(left.createdAt(), right.createdAt());
                })
                .toList();
    }

    @Override
    public Optional<PersistedRoom> findById(String roomId) {
        PersistedRoom room = rooms.get(roomId);
        return room == null || room.deletedAt() != null ? Optional.empty() : Optional.of(room);
    }

    @Override
    public void upsert(PersistedRoom room) {
        rooms.put(room.id(), room);
    }

    @Override
    public void touch(String roomId, long lastActiveAt) {
        rooms.computeIfPresent(roomId, (key, room) -> new PersistedRoom(
                room.id(),
                room.name(),
                room.ownerPublicId(),
                room.visibility(),
                room.passwordHash(),
                room.passwordVersion(),
                room.system(),
                room.createdAt(),
                lastActiveAt,
                room.deletedAt()
        ));
    }

    @Override
    public void softDelete(String roomId, long deletedAt) {
        rooms.computeIfPresent(roomId, (key, room) -> new PersistedRoom(
                room.id(),
                room.name(),
                room.ownerPublicId(),
                room.visibility(),
                room.passwordHash(),
                room.passwordVersion(),
                room.system(),
                room.createdAt(),
                room.lastActiveAt(),
                deletedAt
        ));
    }
}
