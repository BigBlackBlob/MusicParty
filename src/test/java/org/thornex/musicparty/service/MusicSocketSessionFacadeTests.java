package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.messaging.Message;
import org.springframework.messaging.MessageHeaders;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.CurrentUserResponse;
import org.thornex.musicparty.dto.PlayerEvent;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;

import java.util.ArrayList;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class MusicSocketSessionFacadeTests {

    @Test
    void renameAndBroadcastPushesUpdatedCurrentUser() {
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                new AppProperties(),
                new InMemoryRoomRepository(),
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        UserService userService = new UserService(
                event -> {},
                roomService,
                new RoomSessionCoordinator(roomService, event -> {}),
                new InMemoryUserProfileRepository()
        );
        var user = userService.handleConnect("session-1", null, "Alice");
        RecordingMusicPlayerService musicPlayerService = new RecordingMusicPlayerService();
        RecordingSimpMessagingTemplate messagingTemplate = new RecordingSimpMessagingTemplate();
        MusicSocketSessionFacade facade = new MusicSocketSessionFacade(musicPlayerService, userService, messagingTemplate);

        boolean renamed = facade.renameAndBroadcast("session-1", "AliceNew");

        assertThat(renamed).isTrue();
        assertThat(musicPlayerService.broadcastOnlineUsersCalled).isTrue();
        assertThat(messagingTemplate.messages).hasSize(1);
        RecordedMessage message = messagingTemplate.messages.getFirst();
        assertThat(message.destination()).isEqualTo("/user/session-1/queue/me");
        assertThat(message.payload()).isInstanceOf(CurrentUserResponse.class);
        assertThat(((CurrentUserResponse) message.payload()).name()).isEqualTo("AliceNew");
        assertThat(((CurrentUserResponse) message.payload()).publicId()).isEqualTo(user.getPublicId());
    }

    @Test
    void sendSeekDeniedPushesTargetedErrorEvent() {
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                new AppProperties(),
                new InMemoryRoomRepository(),
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        UserService userService = new UserService(
                event -> {},
                roomService,
                new RoomSessionCoordinator(roomService, event -> {}),
                new InMemoryUserProfileRepository()
        );
        var user = userService.handleConnect("session-1", null, "Alice");
        RecordingSimpMessagingTemplate messagingTemplate = new RecordingSimpMessagingTemplate();
        MusicSocketSessionFacade facade = new MusicSocketSessionFacade(new RecordingMusicPlayerService(), userService, messagingTemplate);

        facade.sendSeekDenied("session-1", "denied");

        assertThat(messagingTemplate.messages).hasSize(1);
        RecordedMessage message = messagingTemplate.messages.getFirst();
        assertThat(message.destination()).isEqualTo("/user/session-1/queue/events");
        assertThat(message.payload()).isInstanceOf(PlayerEvent.class);
        assertThat(((PlayerEvent) message.payload()).action()).isEqualTo("SEEK_DENIED");
        assertThat(((PlayerEvent) message.payload()).userId()).isEqualTo(user.getPublicId());
        assertThat(((PlayerEvent) message.payload()).message()).isEqualTo("denied");
    }

    @Test
    void sendRoomCreateFailedPushesTargetedErrorEvent() {
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                new AppProperties(),
                new InMemoryRoomRepository(),
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        UserService userService = new UserService(
                event -> {},
                roomService,
                new RoomSessionCoordinator(roomService, event -> {}),
                new InMemoryUserProfileRepository()
        );
        userService.handleConnect("session-1", null, "Alice");
        RecordingSimpMessagingTemplate messagingTemplate = new RecordingSimpMessagingTemplate();
        MusicSocketSessionFacade facade = new MusicSocketSessionFacade(new RecordingMusicPlayerService(), userService, messagingTemplate);

        facade.sendRoomCreateFailed("session-1", "duplicate");

        assertThat(messagingTemplate.messages).hasSize(1);
        RecordedMessage message = messagingTemplate.messages.getFirst();
        assertThat(message.destination()).isEqualTo("/user/session-1/queue/events");
        assertThat(message.payload()).isInstanceOf(PlayerEvent.class);
        assertThat(((PlayerEvent) message.payload()).action()).isEqualTo("ROOM_CREATE_FAILED");
        assertThat(((PlayerEvent) message.payload()).message()).isEqualTo("duplicate");
    }

    @Test
    void sendChatHistoryPushesTargetedHistoryPayload() {
        RecordingSimpMessagingTemplate messagingTemplate = new RecordingSimpMessagingTemplate();
        MusicSocketSessionFacade facade = new MusicSocketSessionFacade(new RecordingMusicPlayerService(), null, messagingTemplate);
        List<ChatMessage> history = List.of(new ChatMessage("chat-1", "u-1", "Alice", "hello", 1L, org.thornex.musicparty.enums.MessageType.CHAT));

        facade.sendRoomChatHistory("session-1", history);
        facade.sendPublicChatHistory("session-1", history);

        assertThat(messagingTemplate.messages).hasSize(2);
        assertThat(messagingTemplate.messages.get(0).destination()).isEqualTo("/user/session-1/queue/chat/history");
        assertThat(messagingTemplate.messages.get(0).payload()).isEqualTo(history);
        assertThat(messagingTemplate.messages.get(1).destination()).isEqualTo("/user/session-1/queue/public-chat/history");
        assertThat(messagingTemplate.messages.get(1).payload()).isEqualTo(history);
    }

    private static final class RecordingMusicPlayerService extends MusicPlayerService {
        private boolean broadcastOnlineUsersCalled;

        private RecordingMusicPlayerService() {
            super(List.of(), null, null, null, event -> {}, new AppProperties(), null, null, null, null, null, null, null, null);
        }

        @Override
        public void broadcastOnlineUsers() {
            this.broadcastOnlineUsersCalled = true;
        }
    }

    private static final class RecordingSimpMessagingTemplate extends SimpMessagingTemplate {
        private final List<RecordedMessage> messages = new ArrayList<>();

        private RecordingSimpMessagingTemplate() {
            super((destination, message) -> true);
            setUserDestinationPrefix("/user");
        }

        @Override
        protected void doSend(String destination, Message<?> message) {
            messages.add(new RecordedMessage(destination, message.getPayload(), message.getHeaders()));
        }
    }

    private record RecordedMessage(String destination, Object payload, MessageHeaders headers) {
    }
}
