package org.thornex.musicparty.persistence;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

public class InMemorySubsonicSourceRepository implements SubsonicSourceRepository {
    private final ConcurrentHashMap<String, PersistedSubsonicSource> sources = new ConcurrentHashMap<>();
    private final ConcurrentHashMap<String, PersistedRoomSubsonicSource> bindings = new ConcurrentHashMap<>();

    @Override
    public List<PersistedSubsonicSource> findAll() {
        return sources.values().stream()
                .sorted(Comparator.comparing(PersistedSubsonicSource::id))
                .collect(ArrayList::new, ArrayList::add, ArrayList::addAll);
    }

    @Override
    public Optional<PersistedSubsonicSource> findById(String id) {
        return Optional.ofNullable(sources.get(id));
    }

    @Override
    public void upsert(PersistedSubsonicSource source) {
        sources.put(source.id(), source);
    }

    @Override
    public void delete(String id) {
        sources.remove(id);
        bindings.keySet().removeIf(key -> key.endsWith("::" + id));
    }

    @Override
    public List<PersistedRoomSubsonicSource> findRoomBindings(String roomId) {
        return bindings.values().stream()
                .filter(binding -> binding.roomId().equals(roomId))
                .sorted(Comparator.comparingInt(PersistedRoomSubsonicSource::sortOrder).thenComparing(PersistedRoomSubsonicSource::sourceId))
                .collect(ArrayList::new, ArrayList::add, ArrayList::addAll);
    }

    @Override
    public List<PersistedRoomSubsonicSource> findAllRoomBindings() {
        return bindings.values().stream()
                .sorted(Comparator.comparing(PersistedRoomSubsonicSource::roomId)
                        .thenComparingInt(PersistedRoomSubsonicSource::sortOrder)
                        .thenComparing(PersistedRoomSubsonicSource::sourceId))
                .collect(ArrayList::new, ArrayList::add, ArrayList::addAll);
    }

    @Override
    public Optional<PersistedRoomSubsonicSource> findRoomBinding(String roomId, String sourceId) {
        return Optional.ofNullable(bindings.get(key(roomId, sourceId)));
    }

    @Override
    public void upsertRoomBinding(PersistedRoomSubsonicSource binding) {
        bindings.put(key(binding.roomId(), binding.sourceId()), binding);
    }

    @Override
    public void deleteRoomBinding(String roomId, String sourceId) {
        bindings.remove(key(roomId, sourceId));
    }

    private String key(String roomId, String sourceId) {
        return roomId + "::" + sourceId;
    }
}
