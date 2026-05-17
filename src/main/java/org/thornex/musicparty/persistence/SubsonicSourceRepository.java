package org.thornex.musicparty.persistence;

import java.util.List;
import java.util.Optional;

public interface SubsonicSourceRepository {
    List<PersistedSubsonicSource> findAll();
    Optional<PersistedSubsonicSource> findById(String id);
    void upsert(PersistedSubsonicSource source);
    void delete(String id);
    List<PersistedRoomSubsonicSource> findRoomBindings(String roomId);
    List<PersistedRoomSubsonicSource> findAllRoomBindings();
    Optional<PersistedRoomSubsonicSource> findRoomBinding(String roomId, String sourceId);
    void upsertRoomBinding(PersistedRoomSubsonicSource binding);
    void deleteRoomBinding(String roomId, String sourceId);
}
