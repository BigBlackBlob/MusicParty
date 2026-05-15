package org.thornex.musicparty.service;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import lombok.Data;
import lombok.RequiredArgsConstructor;
import org.springframework.context.event.EventListener;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.event.RoomDeletedEvent;
import org.thornex.musicparty.persistence.ChatRepository;
import org.thornex.musicparty.persistence.PersistedHistoryEntry;
import org.thornex.musicparty.persistence.QueueRepository;

import java.io.File;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

@Slf4j
@Service
@RequiredArgsConstructor
public class QueuePersistenceService {

    private final MusicPlayerService musicPlayerService;
    private final ChatService chatService;
    private final AppProperties appProperties;
    private final ObjectMapper objectMapper;
    private final QueueRepository queueRepository;
    private final ChatRepository chatRepository;
    private final RoomService roomService;

    @PostConstruct
    public void init() {
        loadData();
    }

    @PreDestroy
    public void cleanup() {
        saveData();
    }

    @Scheduled(fixedDelayString = "${app.music-api.queue.persistence-interval-ms:60000}")
    public void scheduledSave() {
        saveData();
    }

    private synchronized void saveData() {
        try {
            roomService.listRooms().forEach(room -> {
                String roomId = room.roomId();
                MusicQueueManager manager = musicPlayerService.getSession(roomId).getQueueManager();
                queueRepository.replaceQueue(roomId, manager.getQueueSnapshot());
                queueRepository.replaceHistory(roomId, manager.getHistorySnapshot());
                chatRepository.replaceMessages(roomId, chatService.getHistoryFull(roomId));
            });
            chatRepository.replaceMessages(null, chatService.getPublicHistoryFull());
            log.debug("Queue, music history and chat history saved to SQLite");
        } catch (Exception e) {
            log.error("Failed to save persistence data", e);
        }
    }

    private synchronized void loadData() {
        File file = getPersistenceFile();
        if (!file.exists()) {
            restoreDatabaseData();
            log.info("No legacy persistence file found at {}, restored from SQLite if available.", file.getAbsolutePath());
            return;
        }

        try {
            if (restoreDatabaseData()) {
                log.info("Restored persistence data from SQLite");
                return;
            }

            PersistentData data = objectMapper.readValue(file, new TypeReference<PersistentData>() {});
            importLegacyData(data);
            saveData();
            log.info("Imported legacy persistence data from {} into SQLite", file.getAbsolutePath());
        } catch (Exception e) {
            log.error("Failed to load persistence data from {}", file.getAbsolutePath(), e);
        }
    }

    @EventListener
    public void onRoomDeleted(RoomDeletedEvent event) {
        queueRepository.deleteRoomData(event.getRoomId());
        chatRepository.deleteRoomHistory(event.getRoomId());
    }

    private File getPersistenceFile() {
        String path = appProperties.getQueue().getPersistenceFile();
        File file = new File(path);
        if (file.getParentFile() != null && !file.getParentFile().exists()) {
            file.getParentFile().mkdirs();
        }
        return file;
    }

    private boolean restoreDatabaseData() {
        boolean restoredAny = false;

        for (var room : roomService.listRooms()) {
            String roomId = room.roomId();
            List<MusicQueueItem> queue = queueRepository.loadQueue(roomId);
            List<Music> history = queueRepository.loadHistory(roomId, appProperties.getQueue().getHistorySize()).stream()
                    .map(PersistedHistoryEntry::music)
                    .toList();
            List<ChatMessage> chatHistory = restoreChronological(chatRepository.fetchMessages(roomId, 0, appProperties.getChat().getMaxHistorySize()));

            if (!queue.isEmpty() || !history.isEmpty() || !chatHistory.isEmpty()) {
                restoredAny = true;
            }

            musicPlayerService.getSession(roomId).getQueueManager().restore(queue, history);
            chatService.restore(roomId, chatHistory);
        }

        List<ChatMessage> publicHistory = restoreChronological(chatRepository.fetchMessages(null, 0, appProperties.getChat().getMaxHistorySize()));
        chatService.restorePublic(publicHistory);
        return restoredAny || !publicHistory.isEmpty();
    }

    private void importLegacyData(PersistentData data) {
        if (data.getRooms() != null && !data.getRooms().isEmpty()) {
            data.getRooms().forEach((roomId, roomData) -> {
                musicPlayerService.getSession(roomId).getQueueManager().restore(
                        roomData.getQueue() != null ? roomData.getQueue() : Collections.emptyList(),
                        roomData.getHistory() != null ? roomData.getHistory() : Collections.emptyList()
                );
                chatService.restore(roomId, roomData.getChatHistory() != null ? roomData.getChatHistory() : Collections.emptyList());
            });
        } else {
            musicPlayerService.getSession(RoomService.DEFAULT_ROOM_ID).getQueueManager().restore(
                    data.getQueue() != null ? data.getQueue() : Collections.emptyList(),
                    data.getHistory() != null ? data.getHistory() : Collections.emptyList()
            );
            chatService.restore(RoomService.DEFAULT_ROOM_ID, data.getChatHistory() != null ? data.getChatHistory() : Collections.emptyList());
        }

        chatService.restorePublic(data.getPublicChatHistory() != null ? data.getPublicChatHistory() : Collections.emptyList());
    }

    private List<ChatMessage> restoreChronological(List<ChatMessage> reverseChronological) {
        List<ChatMessage> chronological = new ArrayList<>(reverseChronological);
        Collections.reverse(chronological);
        return chronological;
    }

    @Data
    private static class PersistentData {
        private List<MusicQueueItem> queue;
        private List<Music> history;
        private List<org.thornex.musicparty.dto.ChatMessage> chatHistory;
        private java.util.Map<String, RoomPersistentData> rooms = new java.util.HashMap<>();
        private List<org.thornex.musicparty.dto.ChatMessage> publicChatHistory;
    }

    @Data
    private static class RoomPersistentData {
        private List<MusicQueueItem> queue;
        private List<Music> history;
        private List<org.thornex.musicparty.dto.ChatMessage> chatHistory;
    }
}
