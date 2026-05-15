package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.MusicQueueItem;

import java.util.List;

public interface QueueRepository {
    List<MusicQueueItem> loadQueue(String roomId);
    void replaceQueue(String roomId, List<MusicQueueItem> queueItems);
    List<PersistedHistoryEntry> loadHistory(String roomId, int limit);
    void appendHistory(PersistedHistoryEntry historyEntry);
}
