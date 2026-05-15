package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.persistence.ChatRepository;
import org.thornex.musicparty.persistence.PersistedHistoryEntry;
import org.thornex.musicparty.persistence.QueueRepository;

import java.util.List;
import java.util.UUID;

@Service
@RequiredArgsConstructor
public class RoomStatePersistenceService {

    private final QueueRepository queueRepository;
    private final ChatRepository chatRepository;

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

    public void deleteRoomData(String roomId) {
        queueRepository.deleteRoomData(roomId);
        chatRepository.deleteRoomHistory(roomId);
    }
}
