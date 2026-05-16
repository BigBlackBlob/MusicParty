package org.thornex.musicparty.service;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.annotation.PostConstruct;
import lombok.Data;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.persistence.ChatRepository;
import org.thornex.musicparty.persistence.MigrationStateRepository;
import org.thornex.musicparty.persistence.QueueRepository;

import java.io.File;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

@Slf4j
@Service
public class QueuePersistenceService {
    static final String QUEUE_JSON_MIGRATION_KEY = "legacy.queue-data.json";
    private static final String DEFAULT_LEGACY_QUEUE_DATA_FILE = "data/queue-data.json";

    private final ChatService chatService;
    private final AppProperties appProperties;
    private final ObjectMapper objectMapper;
    private final QueueRepository queueRepository;
    private final ChatRepository chatRepository;
    private final RoomService roomService;
    private final MigrationStateRepository migrationStateRepository;
    private final String legacyQueueDataFilePath;

    @Autowired
    public QueuePersistenceService(ChatService chatService,
                                   AppProperties appProperties,
                                   ObjectMapper objectMapper,
                                   QueueRepository queueRepository,
                                   ChatRepository chatRepository,
                                   RoomService roomService,
                                   MigrationStateRepository migrationStateRepository) {
        this(
                chatService,
                appProperties,
                objectMapper,
                queueRepository,
                chatRepository,
                roomService,
                migrationStateRepository,
                DEFAULT_LEGACY_QUEUE_DATA_FILE
        );
    }

    QueuePersistenceService(ChatService chatService,
                            AppProperties appProperties,
                            ObjectMapper objectMapper,
                            QueueRepository queueRepository,
                            ChatRepository chatRepository,
                            RoomService roomService,
                            MigrationStateRepository migrationStateRepository,
                            String legacyQueueDataFilePath) {
        this.chatService = chatService;
        this.appProperties = appProperties;
        this.objectMapper = objectMapper;
        this.queueRepository = queueRepository;
        this.chatRepository = chatRepository;
        this.roomService = roomService;
        this.migrationStateRepository = migrationStateRepository;
        this.legacyQueueDataFilePath = legacyQueueDataFilePath;
    }

    @PostConstruct
    public void init() {
        loadData();
    }

    private synchronized void loadData() {
        if (migrationStateRepository.isCompleted(QUEUE_JSON_MIGRATION_KEY)) {
            restoreDatabaseData();
            log.info("Legacy queue migration already completed; skipped JSON import.");
            return;
        }

        if (hasPersistedQueueHistoryOrChatData()) {
            restoreDatabaseData();
            migrationStateRepository.markCompleted(QUEUE_JSON_MIGRATION_KEY);
            log.info("Restored persistence data from SQLite");
            return;
        }

        loadDataFromFileForTest(getPersistenceFile());
    }

    void loadDataFromFileForTest(File file) {
        if (!file.exists()) {
            restoreDatabaseData();
            log.info("No legacy persistence file found at {}, restored from SQLite if available.", file.getAbsolutePath());
            return;
        }

        try {
            PersistentData data = objectMapper.readValue(file, new TypeReference<PersistentData>() {});
            importLegacyData(data);
            migrationStateRepository.markCompleted(QUEUE_JSON_MIGRATION_KEY);
            log.info("Imported legacy persistence data from {} into SQLite", file.getAbsolutePath());
        } catch (Exception e) {
            log.error("Failed to load persistence data from {}", file.getAbsolutePath(), e);
        }
    }

    private File getPersistenceFile() {
        File file = new File(legacyQueueDataFilePath);
        if (file.getParentFile() != null && !file.getParentFile().exists()) {
            file.getParentFile().mkdirs();
        }
        return file;
    }

    private void restoreDatabaseData() {
        List<ChatMessage> publicHistory = restoreChronological(chatRepository.fetchMessages(null, 0, appProperties.getChat().getMaxHistorySize()));
        chatService.restorePublic(publicHistory);
    }

    private boolean hasPersistedQueueHistoryOrChatData() {
        if (!chatRepository.fetchMessages(null, 0, 1).isEmpty()) {
            return true;
        }

        return roomService.listRooms().stream()
                .map(room -> room.roomId())
                .anyMatch(roomId -> !queueRepository.loadQueue(roomId).isEmpty()
                        || !queueRepository.loadHistory(roomId, 1).isEmpty()
                        || !chatRepository.fetchMessages(roomId, 0, 1).isEmpty());
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
