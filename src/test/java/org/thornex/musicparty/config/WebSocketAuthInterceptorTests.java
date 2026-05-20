package org.thornex.musicparty.config;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.messaging.Message;
import org.springframework.messaging.MessageDeliveryException;
import org.springframework.messaging.support.MessageBuilder;
import org.springframework.messaging.simp.stomp.StompCommand;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserAccountRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.RoomAccessGrant;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.RoomSessionCoordinator;
import org.thornex.musicparty.service.UserService;

import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class WebSocketAuthInterceptorTests {

    @Test
    void rejectsPublicRoomConnectionsWithoutAccountSession() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Open Room", "u_owner", false, null);
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.accountService, context.roomService, context.roomAccessService);

        assertThatThrownBy(() -> interceptor.preSend(connectMessage(null, room.roomId(), null), null))
                .isInstanceOf(MessageDeliveryException.class)
                .hasMessageContaining("INVALID_ACCOUNT_SESSION");
    }

    @Test
    void allowsPublicRoomConnectionsWithAccountSession() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Open Room", "u_owner", false, null);
        var account = context.accountService.register("alice", "correct-horse-battery-staple");
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.accountService, context.roomService, context.roomAccessService);

        assertThatCode(() -> interceptor.preSend(connectMessage(account.sessionToken(), room.roomId(), null), null))
                .doesNotThrowAnyException();
    }

    @Test
    void rejectsPrivateRoomConnectionsWithoutValidRoomAccessToken() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Secret Room", "u_owner", true, "letmein123");
        var account = context.accountService.register("alice", "correct-horse-battery-staple");
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.accountService, context.roomService, context.roomAccessService);

        assertThatThrownBy(() -> interceptor.preSend(connectMessage(account.sessionToken(), room.roomId(), null), null))
                .isInstanceOf(MessageDeliveryException.class)
                .hasMessageContaining("INVALID_ROOM_ACCESS_TOKEN");
    }

    @Test
    void allowsPrivateRoomConnectionsWithValidRoomAccessToken() {
        TestContext context = new TestContext();
        var room = context.roomService.createRoom("Secret Room", "u_owner", true, "letmein123");
        var account = context.accountService.register("alice", "correct-horse-battery-staple");
        RoomAccessGrant grant = context.roomAccessService.verifyAccess(room.roomId(), account.publicId(), "letmein123");
        WebSocketAuthInterceptor interceptor = new WebSocketAuthInterceptor(context.accountService, context.roomService, context.roomAccessService);

        assertThatCode(() -> interceptor.preSend(connectMessage(account.sessionToken(), room.roomId(), grant.roomAccessToken()), null))
                .doesNotThrowAnyException();
    }

    private static Message<byte[]> connectMessage(String sessionToken, String roomId, String roomAccessToken) {
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
        accessor.setLeaveMutable(true);
        return MessageBuilder.createMessage(new byte[0], accessor.getMessageHeaders());
    }

    private static final class TestContext {
        private final AppProperties properties = new AppProperties();
        private final InMemoryUserProfileRepository userProfiles = new InMemoryUserProfileRepository();
        private final AccountService accountService = new AccountService(new InMemoryUserAccountRepository(), userProfiles);
        private final RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, new InMemoryRoomRepository(), new InMemoryMigrationStateRepository());
        private final UserService userService = new UserService(event -> {}, roomService, new RoomSessionCoordinator(roomService, event -> {}), userProfiles);
        private final RoomAccessService roomAccessService = new RoomAccessService(properties, roomService);

        private TestContext() {
            properties.setAdminPassword("admin-password-12345");
            roomService.init();
        }
    }
}
