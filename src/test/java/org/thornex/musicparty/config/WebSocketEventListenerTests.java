package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;
import org.springframework.messaging.Message;
import org.springframework.messaging.support.MessageBuilder;
import org.springframework.messaging.simp.stomp.StompCommand;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.web.socket.messaging.SessionConnectEvent;
import org.springframework.web.socket.messaging.SessionDisconnectEvent;
import org.thornex.musicparty.service.WebSocketSessionCoordinator;

import static org.assertj.core.api.Assertions.assertThat;

class WebSocketEventListenerTests {

    @Test
    void delegatesConnectHeadersToSessionCoordinator() {
        RecordingWebSocketSessionCoordinator coordinator = new RecordingWebSocketSessionCoordinator();
        WebSocketEventListener listener = new WebSocketEventListener(coordinator);

        listener.handleWebSocketConnectListener(new SessionConnectEvent(this, connectMessage()));

        assertThat(coordinator.connectedSessionId).isEqualTo("ws-session-1");
        assertThat(coordinator.connectedSessionToken).isEqualTo("session-token-1");
        assertThat(coordinator.connectedInitialName).isEqualTo("Alice");
        assertThat(coordinator.connectedRoomId).isEqualTo("room-1");
    }

    @Test
    void delegatesDisconnectSessionToSessionCoordinator() {
        RecordingWebSocketSessionCoordinator coordinator = new RecordingWebSocketSessionCoordinator();
        WebSocketEventListener listener = new WebSocketEventListener(coordinator);

        listener.handleWebSocketDisconnectListener(new SessionDisconnectEvent(this, disconnectMessage(), "ws-session-2", null, null));

        assertThat(coordinator.disconnectedSessionId).isEqualTo("ws-session-2");
    }

    private static Message<byte[]> connectMessage() {
        StompHeaderAccessor accessor = StompHeaderAccessor.create(StompCommand.CONNECT);
        accessor.setSessionId("ws-session-1");
        accessor.setNativeHeader("session-token", "session-token-1");
        accessor.setNativeHeader("user-name", "Alice");
        accessor.setNativeHeader("room-id", "room-1");
        accessor.setLeaveMutable(true);
        return MessageBuilder.createMessage(new byte[0], accessor.getMessageHeaders());
    }

    private static Message<byte[]> disconnectMessage() {
        StompHeaderAccessor accessor = StompHeaderAccessor.create(StompCommand.DISCONNECT);
        accessor.setSessionId("ws-session-2");
        accessor.setLeaveMutable(true);
        return MessageBuilder.createMessage(new byte[0], accessor.getMessageHeaders());
    }

    private static final class RecordingWebSocketSessionCoordinator extends WebSocketSessionCoordinator {
        private String connectedSessionId;
        private String connectedSessionToken;
        private String connectedInitialName;
        private String connectedRoomId;
        private String disconnectedSessionId;

        private RecordingWebSocketSessionCoordinator() {
            super(null, null);
        }

        @Override
        public void handleConnect(String sessionId, String sessionToken, String initialName, String roomId) {
            this.connectedSessionId = sessionId;
            this.connectedSessionToken = sessionToken;
            this.connectedInitialName = initialName;
            this.connectedRoomId = roomId;
        }

        @Override
        public void handleDisconnect(String sessionId) {
            this.disconnectedSessionId = sessionId;
        }
    }
}
