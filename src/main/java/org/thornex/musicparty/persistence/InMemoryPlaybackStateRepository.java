package org.thornex.musicparty.persistence;

import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryPlaybackStateRepository implements PlaybackStateRepository {

    private final Map<String, PersistedPlaybackState> states = new ConcurrentHashMap<>();

    @Override
    public Optional<PersistedPlaybackState> findByRoomId(String roomId) {
        return Optional.ofNullable(states.get(roomId));
    }

    @Override
    public void upsert(PersistedPlaybackState state) {
        states.put(state.roomId(), state);
    }

    @Override
    public void delete(String roomId) {
        states.remove(roomId);
    }
}
