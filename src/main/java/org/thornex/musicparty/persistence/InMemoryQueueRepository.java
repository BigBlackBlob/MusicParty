package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryQueueRepository implements QueueRepository {

    private final Map<String, List<MusicQueueItem>> queues = new ConcurrentHashMap<>();
    private final Map<String, List<PersistedHistoryEntry>> histories = new ConcurrentHashMap<>();

    @Override
    public List<MusicQueueItem> loadQueue(String roomId) {
        return new ArrayList<>(queues.getOrDefault(roomId, Collections.emptyList()));
    }

    @Override
    public void replaceQueue(String roomId, List<MusicQueueItem> queueItems) {
        queues.put(roomId, new ArrayList<>(queueItems));
    }

    @Override
    public List<PersistedHistoryEntry> loadHistory(String roomId, int limit) {
        List<PersistedHistoryEntry> entries = histories.getOrDefault(roomId, Collections.emptyList());
        return new ArrayList<>(entries.subList(0, Math.min(limit, entries.size())));
    }

    @Override
    public void appendHistory(PersistedHistoryEntry historyEntry) {
        histories.computeIfAbsent(historyEntry.roomId(), ignored -> new ArrayList<>()).add(0, historyEntry);
    }

    @Override
    public void replaceHistory(String roomId, List<Music> historyItems) {
        List<PersistedHistoryEntry> entries = new ArrayList<>();
        for (int i = 0; i < historyItems.size(); i++) {
            entries.add(new PersistedHistoryEntry(
                    UUID.randomUUID().toString(),
                    roomId,
                    historyItems.get(i),
                    null,
                    System.currentTimeMillis() - i
            ));
        }
        histories.put(roomId, entries);
    }

    @Override
    public void deleteRoomData(String roomId) {
        queues.remove(roomId);
        histories.remove(roomId);
    }
}
