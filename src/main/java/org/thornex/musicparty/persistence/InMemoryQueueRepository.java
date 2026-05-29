package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.RoomPlaylistTrack;

import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryQueueRepository implements QueueRepository {

    private final Map<String, List<MusicQueueItem>> queues = new ConcurrentHashMap<>();
    private final Map<String, List<PersistedHistoryEntry>> histories = new ConcurrentHashMap<>();
    private final Map<String, Map<String, HistoryTrack>> historyTracks = new ConcurrentHashMap<>();

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
    public int countHistoryTracks(String roomId) {
        return historyTracks.getOrDefault(roomId, Map.of()).size();
    }

    @Override
    public List<RoomPlaylistTrack> listHistoryTracks(String roomId, int offset, int limit) {
        int safeOffset = Math.max(0, offset);
        int safeLimit = Math.max(1, Math.min(500, limit));
        AtomicInteger sortOrder = new AtomicInteger(safeOffset);
        return historyTracks.getOrDefault(roomId, Map.of()).values().stream()
                .sorted(Comparator.comparingLong(HistoryTrack::lastPlayedAt).reversed())
                .skip(safeOffset)
                .limit(safeLimit)
                .map(track -> new RoomPlaylistTrack(track.key(), "__room_history__", track.music(), sortOrder.getAndIncrement(), track.lastPlayedAt()))
                .toList();
    }

    @Override
    public List<Music> listHistoryMusics(String roomId, int limit) {
        return listHistoryTracks(roomId, 0, limit).stream()
                .map(RoomPlaylistTrack::music)
                .toList();
    }

    @Override
    public void appendHistory(PersistedHistoryEntry historyEntry) {
        histories.computeIfAbsent(historyEntry.roomId(), ignored -> new ArrayList<>()).add(0, historyEntry);
        upsertHistoryTrack(historyEntry);
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
        historyTracks.remove(roomId);
    }

    private void upsertHistoryTrack(PersistedHistoryEntry entry) {
        Music music = entry.music();
        if (music == null || music.platform() == null || music.platform().isBlank() || music.id() == null || music.id().isBlank()) {
            return;
        }
        String key = music.platform() + ":" + music.id();
        Map<String, HistoryTrack> roomTracks = historyTracks.computeIfAbsent(entry.roomId(), ignored -> new LinkedHashMap<>());
        HistoryTrack current = roomTracks.get(key);
        if (current == null) {
            roomTracks.put(key, new HistoryTrack(key, music, 1, entry.playedAt(), entry.playedAt()));
            return;
        }
        Music latestMusic = entry.playedAt() >= current.lastPlayedAt() ? music : current.music();
        roomTracks.put(key, new HistoryTrack(
                key,
                latestMusic,
                current.playCount() + 1,
                Math.min(current.firstPlayedAt(), entry.playedAt()),
                Math.max(current.lastPlayedAt(), entry.playedAt())
        ));
    }

    private record HistoryTrack(String key, Music music, int playCount, long firstPlayedAt, long lastPlayedAt) {}
}
