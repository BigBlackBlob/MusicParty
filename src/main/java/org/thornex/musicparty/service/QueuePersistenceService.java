package org.thornex.musicparty.service;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import lombok.Data;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;

import java.io.File;
import java.io.IOException;
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
            File file = getPersistenceFile();
            PersistentData data = new PersistentData();
            musicPlayerService.getActiveRoomIds().forEach(roomId -> {
                RoomPersistentData roomData = new RoomPersistentData();
                MusicQueueManager manager = musicPlayerService.getSession(roomId).getQueueManager();
                roomData.setQueue(manager.getQueueSnapshot());
                roomData.setHistory(manager.getHistorySnapshot());
                roomData.setChatHistory(chatService.getHistoryFull(roomId));
                data.getRooms().put(roomId, roomData);
            });
            data.setPublicChatHistory(chatService.getPublicHistoryFull());

            objectMapper.writeValue(file, data);
            log.debug("Queue, music history and chat history saved to {}", file.getAbsolutePath());
        } catch (Exception e) {
            log.error("Failed to save persistence data", e);
        }
    }

    private synchronized void loadData() {
        File file = getPersistenceFile();
        if (!file.exists()) {
            log.info("No persistence file found at {}, starting fresh.", file.getAbsolutePath());
            return;
        }

        try {
            PersistentData data = objectMapper.readValue(file, new TypeReference<PersistentData>() {});
            
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

            log.info("Restored {} queue items, {} music history items and {} chat messages from {}",
                data.getQueue() != null ? data.getQueue().size() : 0, 
                data.getHistory() != null ? data.getHistory().size() : 0, 
                data.getChatHistory() != null ? data.getChatHistory().size() : 0,
                file.getAbsolutePath());
        } catch (Exception e) {
            log.error("Failed to load persistence data from {}", file.getAbsolutePath(), e);
        }
    }

    private File getPersistenceFile() {
        String path = appProperties.getQueue().getPersistenceFile();
        File file = new File(path);
        if (file.getParentFile() != null && !file.getParentFile().exists()) {
            file.getParentFile().mkdirs();
        }
        return file;
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
