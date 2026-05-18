package org.thornex.musicparty.service;

import org.springframework.transaction.annotation.Transactional;
import org.springframework.stereotype.Service;
import org.springframework.beans.factory.annotation.Autowired;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.persistence.ChatRepository;
import org.thornex.musicparty.persistence.PersistedPlaybackState;
import org.thornex.musicparty.persistence.PersistedHistoryEntry;
import org.thornex.musicparty.persistence.PlaybackStateRepository;
import org.thornex.musicparty.persistence.QueueRepository;
import org.thornex.musicparty.persistence.RoomPlaylistRepository;
import org.thornex.musicparty.persistence.InMemoryRoomPlaylistRepository;

import java.util.List;
import java.util.Optional;
import java.util.UUID;

@Service
public class RoomStatePersistenceService {

    private final QueueRepository queueRepository;
    private final ChatRepository chatRepository;
    private final PlaybackStateRepository playbackStateRepository;
    private final RoomPlaylistRepository roomPlaylistRepository;

    public RoomStatePersistenceService(QueueRepository queueRepository,
                                       ChatRepository chatRepository,
                                       PlaybackStateRepository playbackStateRepository) {
        this(queueRepository, chatRepository, playbackStateRepository, new InMemoryRoomPlaylistRepository());
    }

    @Autowired
    public RoomStatePersistenceService(QueueRepository queueRepository,
                                       ChatRepository chatRepository,
                                       PlaybackStateRepository playbackStateRepository,
                                       RoomPlaylistRepository roomPlaylistRepository) {
        this.queueRepository = queueRepository;
        this.chatRepository = chatRepository;
        this.playbackStateRepository = playbackStateRepository;
        this.roomPlaylistRepository = roomPlaylistRepository;
    }

    public void persistQueueSnapshot(String roomId, List<MusicQueueItem> queueItems) {
        queueRepository.replaceQueue(roomId, queueItems);
    }

    public void persistHistorySnapshot(String roomId, List<Music> historyItems) {
        queueRepository.replaceHistory(roomId, historyItems);
    }

    public void appendHistoryEntry(String roomId, Music music, String enqueuerPublicId) {
        if (music == null) {
            return;
        }
        queueRepository.appendHistory(new PersistedHistoryEntry(
                UUID.randomUUID().toString(),
                roomId,
                music,
                enqueuerPublicId,
                System.currentTimeMillis()
        ));
    }

    public void persistRoomMessage(String roomId, ChatMessage message) {
        chatRepository.appendMessage(roomId, message);
    }

    public void persistPublicMessage(ChatMessage message) {
        chatRepository.appendMessage(null, message);
    }

    public void replaceRoomMessages(String roomId, List<ChatMessage> messages) {
        chatRepository.replaceMessages(roomId, messages);
    }

    public void replacePublicMessages(List<ChatMessage> messages) {
        chatRepository.replaceMessages(null, messages);
    }

    public List<ChatMessage> loadRoomMessages(String roomId, int limit) {
        return restoreChronological(chatRepository.fetchMessages(roomId, 0, limit));
    }

    public List<ChatMessage> loadPublicMessages(int limit) {
        return restoreChronological(chatRepository.fetchMessages(null, 0, limit));
    }

    public void deleteRoomData(String roomId) {
        queueRepository.deleteRoomData(roomId);
        chatRepository.deleteRoomHistory(roomId);
        playbackStateRepository.delete(roomId);
        roomPlaylistRepository.deleteRoomData(roomId);
    }

    public List<MusicQueueItem> loadQueue(String roomId) {
        return queueRepository.loadQueue(roomId);
    }

    public List<Music> loadHistory(String roomId, int limit) {
        return queueRepository.loadHistory(roomId, limit).stream()
                .map(PersistedHistoryEntry::music)
                .toList();
    }

    public Optional<PersistedPlaybackState> loadPlaybackState(String roomId) {
        return playbackStateRepository.findByRoomId(roomId);
    }

    public void persistPlaybackState(PersistedPlaybackState state) {
        playbackStateRepository.upsert(state);
    }

    @Transactional
    public void flushPlayerState(String roomId,
                                List<MusicQueueItem> queueItems,
                                List<Music> historyItems,
                                PersistedPlaybackState playbackState) {
        queueRepository.replaceQueue(roomId, queueItems);
        queueRepository.replaceHistory(roomId, historyItems);
        playbackStateRepository.upsert(playbackState);
    }

    public void deletePlaybackState(String roomId) {
        playbackStateRepository.delete(roomId);
    }

    private List<ChatMessage> restoreChronological(List<ChatMessage> reverseChronological) {
        java.util.ArrayList<ChatMessage> chronological = new java.util.ArrayList<>(reverseChronological);
        java.util.Collections.reverse(chronological);
        return chronological;
    }
}
