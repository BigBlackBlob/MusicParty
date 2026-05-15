package org.thornex.musicparty.config;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.messaging.Message;
import org.springframework.messaging.MessageDeliveryException;
import org.springframework.messaging.support.MessageBuilder;
import org.springframework.messaging.simp.stomp.StompCommand;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.service.RoomAccessGrant;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.UserService;

import java.util.HashMap;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class WebSocketAuthInterceptorTests {

    @Test
    void allowsPublicRoomConnectionsWithoutRoomAccessToken() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Open Room", "u_owner", false, null);
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.userService, context.roomService, context.roomAccessService);

        assertThatCode(() -> interceptor.preSend(connectMessage(null, room.roomId(), null), null))
                .doesNotThrowAnyException();
    }

    @Test
    void rejectsPrivateRoomConnectionsWithoutValidRoomAccessToken() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Secret Room", "u_owner", true, "letmein123");
        var user = context.userService.handleConnect("session-1", null, "Alice", RoomService.DEFAULT_ROOM_ID);
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.userService, context.roomService, context.roomAccessService);

        assertThatThrownBy(() -> interceptor.preSend(connectMessage(user.getSessionToken(), room.roomId(), null), null))
                .isInstanceOf(MessageDeliveryException.class)
                .hasMessageContaining("INVALID_ROOM_ACCESS_TOKEN");
    }

    @Test
    void allowsPrivateRoomConnectionsWithValidRoomAccessToken() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Secret Room", "u_owner", true, "letmein123");
        var user = context.userService.handleConnect("session-1", null, "Alice", RoomService.DEFAULT_ROOM_ID);
        RoomAccessGrant grant = context.roomAccessService.verifyAccess(room.roomId(), user.getPublicId(), "letmein123");
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.userService, context.roomService, context.roomAccessService);

        assertThatCode(() -> interceptor.preSend(connectMessage(user.getSessionToken(), room.roomId(), grant.roomAccessToken()), null))
                .doesNotThrowAnyException();
    }

    @Test
    void allowsPublicRoomSubscriptionsWithoutRoomAccessToken() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Open Room", "u_owner", false, null);
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.userService, context.roomService, context.roomAccessService);

        assertThatCode(() -> interceptor.preSend(subscribeMessage("/topic/rooms/" + room.roomId() + "/chat", new HashMap<>()), null))
                .doesNotThrowAnyException();
    }

    @Test
    void allowsPrivateRoomSubscriptionsAfterValidRoomConnect() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Secret Room", "u_owner", true, "letmein123");
        var user = context.userService.handleConnect("session-1", null, "Alice", RoomService.DEFAULT_ROOM_ID);
        RoomAccessGrant grant = context.roomAccessService.verifyAccess(room.roomId(), user.getPublicId(), "letmein123");
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.userService, context.roomService, context.roomAccessService);
        Map<String, Object> sessionAttributes = new HashMap<>();

        interceptor.preSend(connectMessage(user.getSessionToken(), room.roomId(), grant.roomAccessToken(), sessionAttributes), null);

        assertThatCode(() -> interceptor.preSend(subscribeMessage("/topic/rooms/" + room.roomId() + "/player/state", sessionAttributes), null))
                .doesNotThrowAnyException();
    }

    @Test
    void rejectsPrivateRoomSubscriptionsFromAnotherRoom() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Secret Room", "u_owner", true, "letmein123");
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.userService, context.roomService, context.roomAccessService);
        Map<String, Object> sessionAttributes = new HashMap<>();

        interceptor.preSend(connectMessage(null, RoomService.DEFAULT_ROOM_ID, null, sessionAttributes), null);

        assertThatThrownBy(() -> interceptor.preSend(subscribeMessage("/topic/rooms/" + room.roomId() + "/chat", sessionAttributes), null))
                .isInstanceOf(MessageDeliveryException.class)
                .hasMessageContaining("INVALID_ROOM_SUBSCRIPTION");
        assertThatThrownBy(() -> interceptor.preSend(subscribeMessage("/topic/rooms/" + room.roomId() + "/player/state", sessionAttributes), null))
                .isInstanceOf(MessageDeliveryException.class)
                .hasMessageContaining("INVALID_ROOM_SUBSCRIPTION");
        assertThatThrownBy(() -> interceptor.preSend(subscribeMessage("/topic/rooms/" + room.roomId() + "/player/queue", sessionAttributes), null))
                .isInstanceOf(MessageDeliveryException.class)
                .hasMessageContaining("INVALID_ROOM_SUBSCRIPTION");
        assertThatThrownBy(() -> interceptor.preSend(subscribeMessage("/topic/rooms/" + room.roomId() + "/player/events", sessionAttributes), null))
                .isInstanceOf(MessageDeliveryException.class)
                .hasMessageContaining("INVALID_ROOM_SUBSCRIPTION");
    }

    @Test
    void ignoresMalformedRoomTopicSubscriptions() {
        TestContext context = new TestContext();
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.userService, context.roomService, context.roomAccessService);

        assertThatCode(() -> interceptor.preSend(subscribeMessage("/topic/rooms", new HashMap<>()), null))
                .doesNotThrowAnyException();
        assertThatCode(() -> interceptor.preSend(subscribeMessage("/topic/rooms//chat", new HashMap<>()), null))
                .doesNotThrowAnyException();
        assertThatCode(() -> interceptor.preSend(subscribeMessage("/topic/rooms/list", new HashMap<>()), null))
                .doesNotThrowAnyException();
    }

    private static Message<byte[]> connectMessage(String sessionToken, String roomId, String roomAccessToken) {
        return connectMessage(sessionToken, roomId, roomAccessToken, new HashMap<>());
    }

    private static Message<byte[]> connectMessage(String sessionToken, String roomId, String roomAccessToken, Map<String, Object> sessionAttributes) {
        StompHeaderAccessor accessor = StompHeaderAccessor.create(StompCommand.CONNECT);
        if (sessionToken != null) {
            accessor.setNativeHeader("session-token", sessionToken);
        }
        if (roomId != null) {
            accessor.setNativeHeader("room-id", roomId);
        }
        if (roomAccessToken != null) {
            accessor.setNativeHeader("room-access-token", roomAccessToken);
        }
        accessor.setSessionId("ws-test");
        accessor.setSessionAttributes(sessionAttributes);
        accessor.setLeaveMutable(true);
        return MessageBuilder.createMessage(new byte[0], accessor.getMessageHeaders());
    }

    private static Message<byte[]> subscribeMessage(String destination, Map<String, Object> sessionAttributes) {
        StompHeaderAccessor accessor = StompHeaderAccessor.create(StompCommand.SUBSCRIBE);
        accessor.setDestination(destination);
        accessor.setSessionId("ws-test");
        accessor.setSessionAttributes(sessionAttributes);
        accessor.setLeaveMutable(true);
        return MessageBuilder.createMessage(new byte[0], accessor.getMessageHeaders());
    }

    private static final class TestContext {
        private final AppProperties properties = new AppProperties();
        private final RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, new InMemoryRoomRepository(), new InMemoryMigrationStateRepository());
        private final UserService userService = new UserService(event -> {}, roomService, new InMemoryUserProfileRepository());
        private final RoomAccessService roomAccessService = new RoomAccessService(properties, roomService);

        private TestContext() {
            properties.setAdminPassword("admin-password-12345");
            roomService.init();
        }
    }
}
