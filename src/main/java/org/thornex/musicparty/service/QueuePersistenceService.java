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
            musicPlayerService.getLoadedRoomIds().forEach(roomId -> {
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
        List<ChatMessage> publicHistory = restoreChronological(chatRepository.fetchMessages(null, 0, appProperties.getChat().getMaxHistorySize()));
        chatService.restorePublic(publicHistory);
        return !publicHistory.isEmpty();
    }

    private void importLegacyData(PersistentData data) {
        if (data.getRooms() != null && !data.getRooms().isEmpty()) {
            data.getRooms().forEach((roomId, roomData) -> {
                queueRepository.replaceQueue(roomId, roomData.getQueue() != null ? roomData.getQueue() : Collections.emptyList());
                queueRepository.replaceHistory(roomId, roomData.getHistory() != null ? roomData.getHistory() : Collections.emptyList());
                chatRepository.replaceMessages(roomId, roomData.getChatHistory() != null ? roomData.getChatHistory() : Collections.emptyList());
            });
        } else {
            queueRepository.replaceQueue(RoomService.DEFAULT_ROOM_ID, data.getQueue() != null ? data.getQueue() : Collections.emptyList());
            queueRepository.replaceHistory(RoomService.DEFAULT_ROOM_ID, data.getHistory() != null ? data.getHistory() : Collections.emptyList());
            chatRepository.replaceMessages(RoomService.DEFAULT_ROOM_ID, data.getChatHistory() != null ? data.getChatHistory() : Collections.emptyList());
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
