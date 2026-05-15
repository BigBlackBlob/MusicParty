package org.thornex.musicparty.persistence;

import java.util.Optional;

public interface PlaybackStateRepository {
    Optional<PersistedPlaybackState> findByRoomId(String roomId);
    void upsert(PersistedPlaybackState state);
    void delete(String roomId);
}
