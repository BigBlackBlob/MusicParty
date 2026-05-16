package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.sqlite.SQLiteDataSource;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.config.SqliteSchemaInitializer;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.enums.QueueItemStatus;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.JdbcChatRepository;
import org.thornex.musicparty.persistence.JdbcMigrationStateRepository;
import org.thornex.musicparty.persistence.JdbcPlaybackStateRepository;
import org.thornex.musicparty.persistence.JdbcQueueRepository;
import org.thornex.musicparty.persistence.JdbcRoomRepository;
import org.thornex.musicparty.persistence.MigrationStateRepository;
import org.thornex.musicparty.persistence.QueueRepository;
import org.thornex.musicparty.persistence.RoomRepository;

import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class QueuePersistenceServiceTests {

    @Test
    void initImportsLegacyJsonWhenOnlyDefaultRoomExistsAndPersistenceTablesAreEmpty() throws Exception {
        AppProperties properties = new AppProperties();
        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        InMemoryChatRepository chatRepository = new InMemoryChatRepository();
        InMemoryMigrationStateRepository migrationStateRepository = new InMemoryMigrationStateRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                new InMemoryRoomRepository(),
                migrationStateRepository
        );
        roomService.init();

        ChatService chatService = new ChatService(
                null,
                null,
                properties,
                new RoomStatePersistenceService(
                        queueRepository,
                        chatRepository,
                        new InMemoryPlaybackStateRepository()
                ),
                new RoomStateMutationService(new TestTransactionManager()),
                new RoomSessionCoordinator(roomService, event -> {}),
                new AfterCommitExecutor(),
                List.of()
        );

        Path tempDir = Path.of("target", "tmp", "queue-persistence-tests");
        Files.createDirectories(tempDir);
        File legacyFile = tempDir.resolve("queue-data-init.json").toFile();
        Files.writeString(legacyFile.toPath(), """
                {
                  "queue": [
                    {
                      "queueId": "queue-legacy",
                      "music": {
                        "id": "music-legacy",
                        "name": "Legacy Song",
                        "artists": ["Legacy Artist"],
                        "duration": 123000,
                        "platform": "netease",
                        "coverUrl": "legacy-cover"
                      },
                      "enqueuedBy": {
                        "publicId": "u-legacy",
                        "name": "Legacy User",
                        "isGuest": false
                      },
                      "status": "READY"
                    }
                  ],
                  "history": [
                    {
                      "id": "music-history",
                      "name": "Legacy History",
                      "artists": ["Legacy Artist"],
                      "duration": 456000,
                      "platform": "netease",
                      "coverUrl": "history-cover"
                    }
                  ],
                  "chatHistory": [
                    {
                      "id": "chat-legacy",
                      "userId": "u-legacy",
                      "userName": "Legacy User",
                      "content": "legacy room message",
                      "timestamp": 10,
                      "type": "CHAT"
                    }
                  ],
                  "publicChatHistory": [
                    {
                      "id": "chat-public",
                      "userId": "u-public",
                      "userName": "Public User",
                      "content": "legacy public message",
                      "timestamp": 20,
                      "type": "CHAT"
                    }
                  ]
                }
                """);

        QueuePersistenceService service = new QueuePersistenceService(
                chatService,
                properties,
                new ObjectMapper(),
                queueRepository,
                chatRepository,
                roomService,
                migrationStateRepository,
                legacyFile.getPath()
        );

        service.init();

        assertThat(queueRepository.loadQueue(RoomService.DEFAULT_ROOM_ID)).hasSize(1);
        assertThat(queueRepository.loadHistory(RoomService.DEFAULT_ROOM_ID, 10)).hasSize(1);
        assertThat(chatRepository.fetchMessages(RoomService.DEFAULT_ROOM_ID, 0, 10)).hasSize(1);
        assertThat(chatService.getPublicHistoryFull()).hasSize(1);
        assertThat(migrationStateRepository.isCompleted(QueuePersistenceService.QUEUE_JSON_MIGRATION_KEY)).isTrue();
    }

    @Test
    void initRestoresPublicChatFromDatabaseWithoutRewritingQueueHistoryOrRoomChat() {
        AppProperties properties = new AppProperties();
        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        InMemoryChatRepository chatRepository = new InMemoryChatRepository();
        InMemoryMigrationStateRepository migrationStateRepository = new InMemoryMigrationStateRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                new InMemoryRoomRepository(),
                migrationStateRepository
        );
        roomService.init();

        String roomId = RoomService.DEFAULT_ROOM_ID;
        MusicQueueItem queueItem = new MusicQueueItem(
                "queue-1",
                new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a"),
                new UserSummary("u-1", "Alice", false),
                QueueItemStatus.READY
        );
        Music historyMusic = new Music("music-2", "Song B", List.of("Artist B"), 200_000L, "netease", "cover-b");
        ChatMessage roomMessage = new ChatMessage("chat-room", "u-1", "Alice", "hello room", 1L, MessageType.CHAT);
        ChatMessage publicMessage = new ChatMessage("chat-public", "u-2", "Bob", "hello public", 2L, MessageType.CHAT);

        queueRepository.replaceQueue(roomId, List.of(queueItem));
        queueRepository.replaceHistory(roomId, List.of(historyMusic));
        chatRepository.replaceMessages(roomId, List.of(roomMessage));
        chatRepository.replaceMessages(null, List.of(publicMessage));

        ChatService chatService = new ChatService(
                null,
                null,
                properties,
                new RoomStatePersistenceService(
                        queueRepository,
                        chatRepository,
                        new InMemoryPlaybackStateRepository()
                ),
                new RoomStateMutationService(new TestTransactionManager()),
                new RoomSessionCoordinator(roomService, event -> {}),
                new AfterCommitExecutor(),
                List.of()
        );

        QueuePersistenceService service = new QueuePersistenceService(
                chatService,
                properties,
                new ObjectMapper(),
                queueRepository,
                chatRepository,
                roomService,
                migrationStateRepository
        );

        service.init();

        assertThat(queueRepository.loadQueue(roomId)).containsExactly(queueItem);
        assertThat(queueRepository.loadHistory(roomId, 10)).extracting(entry -> entry.music()).containsExactly(historyMusic);
        assertThat(chatRepository.fetchMessages(roomId, 0, 10)).containsExactly(roomMessage);
        assertThat(chatService.getPublicHistoryFull()).containsExactly(publicMessage);
        assertThat(migrationStateRepository.isCompleted(QueuePersistenceService.QUEUE_JSON_MIGRATION_KEY)).isTrue();
    }

    @Test
    void initImportsLegacyJsonIntoDatabaseOnceWhenDatabaseIsEmpty() throws Exception {
        AppProperties properties = new AppProperties();
        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        InMemoryChatRepository chatRepository = new InMemoryChatRepository();
        InMemoryMigrationStateRepository migrationStateRepository = new InMemoryMigrationStateRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                new InMemoryRoomRepository(),
                migrationStateRepository
        );
        roomService.init();

        ChatService chatService = new ChatService(
                null,
                null,
                properties,
                new RoomStatePersistenceService(
                        queueRepository,
                        chatRepository,
                        new InMemoryPlaybackStateRepository()
                ),
                new RoomStateMutationService(new TestTransactionManager()),
                new RoomSessionCoordinator(roomService, event -> {}),
                new AfterCommitExecutor(),
                List.of()
        );

        QueuePersistenceService service = new QueuePersistenceService(
                chatService,
                properties,
                new ObjectMapper(),
                queueRepository,
                chatRepository,
                roomService,
                migrationStateRepository
        );

        Path tempDir = Path.of("target", "tmp", "queue-persistence-tests");
        Files.createDirectories(tempDir);
        File legacyFile = tempDir.resolve("queue-data.json").toFile();
        Files.writeString(legacyFile.toPath(), """
                {
                  "queue": [
                    {
                      "queueId": "queue-legacy",
                      "music": {
                        "id": "music-legacy",
                        "name": "Legacy Song",
                        "artists": ["Legacy Artist"],
                        "duration": 123000,
                        "platform": "netease",
                        "coverUrl": "legacy-cover"
                      },
                      "enqueuedBy": {
                        "publicId": "u-legacy",
                        "name": "Legacy User",
                        "isGuest": false
                      },
                      "status": "READY"
                    }
                  ],
                  "history": [
                    {
                      "id": "music-history",
                      "name": "Legacy History",
                      "artists": ["Legacy Artist"],
                      "duration": 456000,
                      "platform": "netease",
                      "coverUrl": "history-cover"
                    }
                  ],
                  "chatHistory": [
                    {
                      "id": "chat-legacy",
                      "userId": "u-legacy",
                      "userName": "Legacy User",
                      "content": "legacy room message",
                      "timestamp": 10,
                      "type": "CHAT"
                    }
                  ],
                  "publicChatHistory": [
                    {
                      "id": "chat-public",
                      "userId": "u-public",
                      "userName": "Public User",
                      "content": "legacy public message",
                      "timestamp": 20,
                      "type": "CHAT"
                    }
                  ]
                }
                """);

        service.loadDataFromFileForTest(legacyFile);

        assertThat(queueRepository.loadQueue(RoomService.DEFAULT_ROOM_ID)).hasSize(1);
        assertThat(queueRepository.loadHistory(RoomService.DEFAULT_ROOM_ID, 10)).hasSize(1);
        assertThat(chatRepository.fetchMessages(RoomService.DEFAULT_ROOM_ID, 0, 10)).hasSize(1);
        assertThat(chatService.getPublicHistoryFull()).hasSize(1);
        assertThat(migrationStateRepository.isCompleted(QueuePersistenceService.QUEUE_JSON_MIGRATION_KEY)).isTrue();
    }

    @Test
    void initDoesNotMarkMigrationCompletedWhenLegacyFileIsMissingAndDatabaseIsEmpty() throws Exception {
        AppProperties properties = new AppProperties();
        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        InMemoryChatRepository chatRepository = new InMemoryChatRepository();
        InMemoryMigrationStateRepository migrationStateRepository = new InMemoryMigrationStateRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                new InMemoryRoomRepository(),
                migrationStateRepository
        );
        roomService.init();

        ChatService chatService = new ChatService(
                null,
                null,
                properties,
                new RoomStatePersistenceService(
                        queueRepository,
                        chatRepository,
                        new InMemoryPlaybackStateRepository()
                ),
                new RoomStateMutationService(new TestTransactionManager()),
                new RoomSessionCoordinator(roomService, event -> {}),
                new AfterCommitExecutor(),
                List.of()
        );

        Path tempDir = Path.of("target", "tmp", "queue-persistence-tests");
        Files.createDirectories(tempDir);
        File missingLegacyFile = tempDir.resolve("missing-queue-data.json").toFile();
        Files.deleteIfExists(missingLegacyFile.toPath());

        QueuePersistenceService service = new QueuePersistenceService(
                chatService,
                properties,
                new ObjectMapper(),
                queueRepository,
                chatRepository,
                roomService,
                migrationStateRepository,
                missingLegacyFile.getPath()
        );

        service.init();

        assertThat(queueRepository.loadQueue(RoomService.DEFAULT_ROOM_ID)).isEmpty();
        assertThat(queueRepository.loadHistory(RoomService.DEFAULT_ROOM_ID, 10)).isEmpty();
        assertThat(chatRepository.fetchMessages(RoomService.DEFAULT_ROOM_ID, 0, 10)).isEmpty();
        assertThat(chatService.getPublicHistoryFull()).isEmpty();
        assertThat(migrationStateRepository.isCompleted(QueuePersistenceService.QUEUE_JSON_MIGRATION_KEY)).isFalse();
    }

    @Test
    void initImportsLegacyJsonOnRealSqliteStartupWhenOnlyDefaultRoomExists() throws Exception {
        AppProperties properties = new AppProperties();
        Path tempDir = Path.of("target", "tmp", "queue-persistence-tests", "sqlite-startup");
        Files.createDirectories(tempDir);

        Path dbPath = tempDir.resolve("musicparty.db");
        Files.deleteIfExists(dbPath);

        File legacyFile = tempDir.resolve("queue-data.json").toFile();
        Files.writeString(legacyFile.toPath(), """
                {
                  "queue": [
                    {
                      "queueId": "queue-sqlite",
                      "music": {
                        "id": "music-sqlite",
                        "name": "SQLite Song",
                        "artists": ["SQLite Artist"],
                        "duration": 321000,
                        "platform": "netease",
                        "coverUrl": "sqlite-cover"
                      },
                      "enqueuedBy": {
                        "publicId": "u-sqlite",
                        "name": "SQLite User",
                        "isGuest": false
                      },
                      "status": "READY"
                    }
                  ],
                  "history": [
                    {
                      "id": "music-history-sqlite",
                      "name": "SQLite History",
                      "artists": ["SQLite Artist"],
                      "duration": 654000,
                      "platform": "netease",
                      "coverUrl": "sqlite-history-cover"
                    }
                  ],
                  "chatHistory": [
                    {
                      "id": "chat-room-sqlite",
                      "userId": "u-sqlite",
                      "userName": "SQLite User",
                      "content": "sqlite room message",
                      "timestamp": 30,
                      "type": "CHAT"
                    }
                  ],
                  "publicChatHistory": [
                    {
                      "id": "chat-public-sqlite",
                      "userId": "u-public-sqlite",
                      "userName": "SQLite Public User",
                      "content": "sqlite public message",
                      "timestamp": 40,
                      "type": "CHAT"
                    }
                  ]
                }
                """);

        SQLiteDataSource dataSource = new SQLiteDataSource();
        dataSource.setUrl("jdbc:sqlite:" + dbPath.toAbsolutePath());
        ResourceDatabasePopulator populator = new ResourceDatabasePopulator(
                new org.springframework.core.io.ClassPathResource("db/schema.sql")
        );
        new SqliteSchemaInitializer(dataSource, populator).initialize();

        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        ObjectMapper objectMapper = new ObjectMapper();
        QueueRepository queueRepository = new JdbcQueueRepository(jdbcTemplate, objectMapper);
        JdbcChatRepository chatRepository = new JdbcChatRepository(jdbcTemplate);
        MigrationStateRepository migrationStateRepository = new JdbcMigrationStateRepository(jdbcTemplate);
        RoomRepository roomRepository = new JdbcRoomRepository(jdbcTemplate);

        RoomService roomService = new RoomService(
                objectMapper,
                event -> {},
                properties,
                roomRepository,
                migrationStateRepository
        );
        roomService.init();

        ChatService chatService = new ChatService(
                null,
                null,
                properties,
                new RoomStatePersistenceService(
                        queueRepository,
                        chatRepository,
                        new JdbcPlaybackStateRepository(jdbcTemplate, objectMapper)
                ),
                new RoomStateMutationService(new DataSourceTransactionManager(dataSource)),
                new RoomSessionCoordinator(roomService, event -> {}),
                new AfterCommitExecutor(),
                List.of()
        );

        QueuePersistenceService service = new QueuePersistenceService(
                chatService,
                properties,
                objectMapper,
                queueRepository,
                chatRepository,
                roomService,
                migrationStateRepository,
                legacyFile.getPath()
        );

        service.init();

        assertThat(queueRepository.loadQueue(RoomService.DEFAULT_ROOM_ID)).hasSize(1);
        assertThat(queueRepository.loadHistory(RoomService.DEFAULT_ROOM_ID, 10)).extracting(entry -> entry.music().name())
                .containsExactly("SQLite History");
        assertThat(chatRepository.fetchMessages(RoomService.DEFAULT_ROOM_ID, 0, 10)).extracting(ChatMessage::content)
                .containsExactly("sqlite room message");
        assertThat(chatService.getPublicHistoryFull()).extracting(ChatMessage::content)
                .containsExactly("sqlite public message");
        assertThat(migrationStateRepository.isCompleted(QueuePersistenceService.QUEUE_JSON_MIGRATION_KEY)).isTrue();
        assertThat(jdbcTemplate.queryForObject("select count(1) from room where deleted_at is null", Integer.class)).isEqualTo(1);
    }
}
