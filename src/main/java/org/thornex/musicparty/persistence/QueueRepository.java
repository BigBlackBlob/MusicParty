package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.RoomPlaylistTrack;

import java.util.List;

public interface QueueRepository {
    List<MusicQueueItem> loadQueue(String roomId);
    void replaceQueue(String roomId, List<MusicQueueItem> queueItems);
    void synchronizeQueue(String roomId, List<MusicQueueItem> queueItems);
    List<PersistedHistoryEntry> loadHistory(String roomId, int limit);
    int countHistoryTracks(String roomId);
    List<RoomPlaylistTrack> listHistoryTracks(String roomId, int offset, int limit);
    List<Music> listHistoryMusics(String roomId, int limit);
    void appendHistory(PersistedHistoryEntry historyEntry);
    void replaceHistory(String roomId, List<Music> historyItems);
    void deleteRoomData(String roomId);
}
