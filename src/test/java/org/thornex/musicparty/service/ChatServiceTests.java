package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.transaction.PlatformTransactionManager;
import org.sqlite.SQLiteDataSource;
import org.thornex.musicparty.dto.ChatRequest;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.enums.PlayerAction;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.persistence.ChatRepository;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.JdbcChatRepository;
import org.thornex.musicparty.persistence.JdbcMigrationStateRepository;
import org.thornex.musicparty.persistence.JdbcRoomRepository;
import org.thornex.musicparty.persistence.JdbcUserProfileRepository;
import org.thornex.musicparty.config.SqliteSchemaInitializer;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;

class ChatServiceTests {

    @Test
    void resetSystemEventReplacesRoomHistoryInsteadOfAppendingDuplicateSystemMessages() {
        ChatService chatService = createChatService();
        String roomId = "room-1";

        chatService.addMessage(roomId, new ChatMessage("msg-1", "user-1", "Alice", "hello", 1L, MessageType.CHAT));

        chatService.onSystemEvent(new SystemMessageEvent(
                this,
                SystemMessageEvent.Level.WARN,
                PlayerAction.RESET,
                "SYSTEM",
                null,
                roomId
        ));

        List<ChatMessage> history = chatService.getHistoryFull(roomId);
        assertThat(history).hasSize(1);
        assertThat(history.getFirst().type()).isEqualTo(MessageType.SYSTEM);
        assertThat(history.getFirst().content()).isEqualTo("系统已被重置");
    }

    @Test
    void clearHistoryAndNotifyLeavesSingleSystemMessageAsNewHistoryStart() {
        ChatService chatService = createChatService();
        String roomId = "room-2";

        chatService.addMessage(roomId, new ChatMessage("msg-1", "user-1", "Alice", "hello", 1L, MessageType.CHAT));
        chatService.addMessage(roomId, new ChatMessage("msg-2", "user-2", "Bob", "world", 2L, MessageType.CHAT));

        chatService.clearHistoryAndNotify(roomId);

        List<ChatMessage> history = chatService.getHistoryFull(roomId);
        assertThat(history).hasSize(1);
        assertThat(history.getFirst().type()).isEqualTo(MessageType.SYSTEM);
        assertThat(history.getFirst().content()).isEqualTo("聊天记录已由管理员清空");
    }

    @Test
    void addMessageUpdatesMemoryAndBroadcastOnlyAfterCommit() {
        RecordingMessagingTemplate messagingTemplate = new RecordingMessagingTemplate();
        ChatService chatService = createChatService(messagingTemplate, new TestTransactionManager());
        String roomId = "room-commit";
        ChatMessage message = new ChatMessage("msg-1", "user-1", "Alice", "hello", 1L, MessageType.CHAT);

        chatService.addMessage(roomId, message);

        assertThat(chatService.getHistoryFull(roomId)).containsExactly(message);
        assertThat(messagingTemplate.destinations).containsExactly("/topic/rooms/" + roomId + "/chat");
        assertThat(messagingTemplate.payloads).containsExactly(message);
    }

    @Test
    void addMessageDoesNotUpdateMemoryOrBroadcastWhenPersistenceFails() {
        RecordingMessagingTemplate messagingTemplate = new RecordingMessagingTemplate();
        ChatService chatService = createChatService(messagingTemplate, new TestTransactionManager(), createUserService(), true);
        String roomId = "room-fail";
        ChatMessage message = new ChatMessage("msg-1", "user-1", "Alice", "hello", 1L, MessageType.CHAT);

        try {
            chatService.addMessage(roomId, message);
        } catch (IllegalStateException ex) {
            assertThat(ex).hasMessage("db write failed");
        }

        assertThat(chatService.getHistoryFull(roomId)).isEmpty();
        assertThat(messagingTemplate.destinations).isEmpty();
    }

    @Test
    void handlePublicChatPersistsAndBroadcastsAfterCommit() {
        RecordingMessagingTemplate messagingTemplate = new RecordingMessagingTemplate();
        UserService userService = createUserService();
        User user = userService.handleConnect("session-public", null, "Alice");
        ChatService chatService = createChatService(messagingTemplate, new TestTransactionManager(), userService);

        chatService.handlePublicChat("session-public", new ChatRequest("hello public"));

        List<ChatMessage> history = chatService.getPublicHistoryFull();
        assertThat(history).hasSize(1);
        assertThat(history.getFirst().userId()).isEqualTo(user.getPublicId());
        assertThat(history.getFirst().content()).isEqualTo("hello public");
        assertThat(messagingTemplate.destinations).containsExactly("/topic/public/chat");
        assertThat(((ChatMessage) messagingTemplate.payloads.getFirst()).content()).isEqualTo("hello public");
    }

    @Test
    void handleRoomChatPersistsToRealSqliteAndBroadcastsAfterCommit() throws Exception {
        SqliteChatContext context = createSqliteChatContext("chat-room-success.db", false);
        User user = context.userService.handleConnect("session-room", null, "Alice");

        context.chatService.handleRoomChat("session-room", new ChatRequest("hello sqlite room"));

        assertThat(context.chatService.getHistoryFull(RoomService.DEFAULT_ROOM_ID)).hasSize(1);
        assertThat(context.jdbcChatRepository.fetchMessages(RoomService.DEFAULT_ROOM_ID, 0, 10))
                .extracting(ChatMessage::content)
                .containsExactly("hello sqlite room");
        assertThat(context.messagingTemplate.destinations).containsExactly("/topic/rooms/" + RoomService.DEFAULT_ROOM_ID + "/chat");
        assertThat(((ChatMessage) context.messagingTemplate.payloads.getFirst()).userId()).isEqualTo(user.getPublicId());
    }

    @Test
    void addMessageWithRealSqliteDoesNotUpdateMemoryOrBroadcastWhenPersistenceFails() throws Exception {
        SQLiteDataSource dataSource = createSqliteDataSource("chat-room-failure.db");
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        JdbcChatRepository jdbcChatRepository = new JdbcChatRepository(jdbcTemplate);
        ObjectMapper objectMapper = new ObjectMapper();
        AppProperties properties = new AppProperties();
        RoomService roomService = new RoomService(
                objectMapper,
                event -> {},
                properties,
                new JdbcRoomRepository(jdbcTemplate),
                new JdbcMigrationStateRepository(jdbcTemplate)
        );
        roomService.init();
        RecordingMessagingTemplate messagingTemplate = new RecordingMessagingTemplate();
        ChatService chatService = new ChatService(
                messagingTemplate,
                createUserService(),
                properties,
                new RoomStatePersistenceService(
                        new InMemoryQueueRepository(),
                        new FailingAfterWriteChatRepository(jdbcChatRepository),
                        new InMemoryPlaybackStateRepository()
                ),
                new RoomStateMutationService(new DataSourceTransactionManager(dataSource)),
                new RoomSessionCoordinator(roomService, event -> {}),
                new AfterCommitExecutor(),
                List.of()
        );
        ChatMessage message = new ChatMessage("msg-sqlite-fail", "user-1", "Alice", "boom", 1L, MessageType.CHAT);

        try {
            chatService.addMessage(RoomService.DEFAULT_ROOM_ID, message);
        } catch (IllegalStateException ex) {
            assertThat(ex).hasMessage("db write failed");
        }

        assertThat(chatService.getHistoryFull(RoomService.DEFAULT_ROOM_ID)).isEmpty();
        assertThat(jdbcChatRepository.fetchMessages(RoomService.DEFAULT_ROOM_ID, 0, 10)).isEmpty();
        assertThat(messagingTemplate.destinations).isEmpty();
    }

    private ChatService createChatService() {
        return createChatService(null, new TestTransactionManager(), createUserService());
    }

    private ChatService createChatService(SimpMessagingTemplate messagingTemplate, PlatformTransactionManager transactionManager) {
        return createChatService(messagingTemplate, transactionManager, createUserService(), false);
    }

    private ChatService createChatService(SimpMessagingTemplate messagingTemplate,
                                          PlatformTransactionManager transactionManager,
                                          UserService userService) {
        return createChatService(messagingTemplate, transactionManager, userService, false);
    }

    private ChatService createChatService(SimpMessagingTemplate messagingTemplate,
                                          PlatformTransactionManager transactionManager,
                                          UserService userService,
                                          boolean failOnPersist) {
        AppProperties properties = new AppProperties();
        RoomService roomService = new RoomService(
                new com.fasterxml.jackson.databind.ObjectMapper(),
                event -> {},
                properties,
                new InMemoryRoomRepository(),
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        return new ChatService(
                messagingTemplate,
                userService,
                properties,
                new RoomStatePersistenceService(
                        new InMemoryQueueRepository(),
                        failOnPersist ? new FailingChatRepository() : new InMemoryChatRepository(),
                        new InMemoryPlaybackStateRepository()
                ),
                new RoomStateMutationService(transactionManager),
                new RoomSessionCoordinator(roomService, event -> {}),
                new AfterCommitExecutor(),
                List.of()
        );
    }

    private UserService createUserService() {
        AppProperties properties = new AppProperties();
        RoomService roomService = new RoomService(
                new com.fasterxml.jackson.databind.ObjectMapper(),
                event -> {},
                properties,
                new InMemoryRoomRepository(),
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        return new UserService(
                event -> {},
                roomService,
                new RoomSessionCoordinator(roomService, event -> {}),
                new InMemoryUserProfileRepository()
        );
    }

    private SqliteChatContext createSqliteChatContext(String fileName, boolean failAfterWrite) throws Exception {
        SQLiteDataSource dataSource = createSqliteDataSource(fileName);
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        ObjectMapper objectMapper = new ObjectMapper();
        AppProperties properties = new AppProperties();
        RoomService roomService = new RoomService(
                objectMapper,
                event -> {},
                properties,
                new JdbcRoomRepository(jdbcTemplate),
                new JdbcMigrationStateRepository(jdbcTemplate)
        );
        roomService.init();
        UserService userService = new UserService(
                event -> {},
                roomService,
                new RoomSessionCoordinator(roomService, event -> {}),
                new JdbcUserProfileRepository(jdbcTemplate)
        );
        RecordingMessagingTemplate messagingTemplate = new RecordingMessagingTemplate();
        JdbcChatRepository jdbcChatRepository = new JdbcChatRepository(jdbcTemplate);
        return new SqliteChatContext(
                new ChatService(
                        messagingTemplate,
                        userService,
                        properties,
                        new RoomStatePersistenceService(
                                new InMemoryQueueRepository(),
                                failAfterWrite ? new FailingAfterWriteChatRepository(jdbcChatRepository) : jdbcChatRepository,
                                new InMemoryPlaybackStateRepository()
                        ),
                        new RoomStateMutationService(new DataSourceTransactionManager(dataSource)),
                        new RoomSessionCoordinator(roomService, event -> {}),
                        new AfterCommitExecutor(),
                        List.of()
                ),
                userService,
                jdbcChatRepository,
                messagingTemplate
        );
    }

    private SQLiteDataSource createSqliteDataSource(String fileName) throws Exception {
        Path tempDir = Path.of("target", "tmp", "chat-service-tests");
        Files.createDirectories(tempDir);
        Path dbPath = tempDir.resolve(fileName);
        Files.deleteIfExists(dbPath);
        SQLiteDataSource dataSource = new SQLiteDataSource();
        dataSource.setUrl("jdbc:sqlite:" + dbPath.toAbsolutePath());
        ResourceDatabasePopulator populator = new ResourceDatabasePopulator(
                new org.springframework.core.io.ClassPathResource("db/schema.sql")
        );
        new SqliteSchemaInitializer(dataSource, populator).initialize();
        return dataSource;
    }

    private static final class RecordingMessagingTemplate extends SimpMessagingTemplate {
        private final java.util.LinkedList<String> destinations = new java.util.LinkedList<>();
        private final java.util.LinkedList<Object> payloads = new java.util.LinkedList<>();

        private RecordingMessagingTemplate() {
            super(new org.springframework.messaging.MessageChannel() {
                @Override
                public boolean send(org.springframework.messaging.Message<?> message) {
                    return true;
                }

                @Override
                public boolean send(org.springframework.messaging.Message<?> message, long timeout) {
                    return true;
                }
            });
        }

        @Override
        public void convertAndSend(String destination, Object payload) {
            destinations.add(destination);
            payloads.add(payload);
        }
    }

    private static final class FailingChatRepository extends InMemoryChatRepository {
        @Override
        public void appendMessage(String roomId, ChatMessage message) {
            throw new IllegalStateException("db write failed");
        }
    }

    private record SqliteChatContext(
            ChatService chatService,
            UserService userService,
            JdbcChatRepository jdbcChatRepository,
            RecordingMessagingTemplate messagingTemplate
    ) {
    }

    private static final class FailingAfterWriteChatRepository implements ChatRepository {
        private final JdbcChatRepository delegate;

        private FailingAfterWriteChatRepository(JdbcChatRepository delegate) {
            this.delegate = delegate;
        }

        @Override
        public void appendMessage(String roomId, ChatMessage message) {
            delegate.appendMessage(roomId, message);
            throw new IllegalStateException("db write failed");
        }

        @Override
        public List<ChatMessage> fetchMessages(String roomId, int offset, int limit) {
            return delegate.fetchMessages(roomId, offset, limit);
        }

        @Override
        public void replaceMessages(String roomId, List<ChatMessage> messages) {
            delegate.replaceMessages(roomId, messages);
        }

        @Override
        public void deleteRoomHistory(String roomId) {
            delegate.deleteRoomHistory(roomId);
        }
    }
}
