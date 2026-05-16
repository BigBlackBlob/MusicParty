package org.thornex.musicparty.config;

import lombok.extern.slf4j.Slf4j;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.stereotype.Component;
import org.springframework.web.socket.messaging.SessionConnectEvent;
import org.springframework.web.socket.messaging.SessionDisconnectEvent;
import org.thornex.musicparty.service.WebSocketSessionCoordinator;

@Component
@Slf4j
public class WebSocketEventListener {

    private final WebSocketSessionCoordinator webSocketSessionCoordinator;

    public WebSocketEventListener(WebSocketSessionCoordinator webSocketSessionCoordinator) {
        this.webSocketSessionCoordinator = webSocketSessionCoordinator;
    }

    @EventListener
    public void handleWebSocketConnectListener(SessionConnectEvent event) {
        StompHeaderAccessor headerAccessor = StompHeaderAccessor.wrap(event.getMessage());
        String sessionId = headerAccessor.getSessionId();

        String initialName = headerAccessor.getFirstNativeHeader("user-name");
        String sessionToken = headerAccessor.getFirstNativeHeader("session-token");
        String roomId = headerAccessor.getFirstNativeHeader("room-id");

        log.info("WebSocket Connect Request: Session={}, InitialName={}", sessionId, initialName);

        if (sessionId != null) {
            webSocketSessionCoordinator.handleConnect(sessionId, sessionToken, initialName, roomId);
        }
    }

    @EventListener
    public void handleWebSocketDisconnectListener(SessionDisconnectEvent event) {
        StompHeaderAccessor headerAccessor = StompHeaderAccessor.wrap(event.getMessage());
        String sessionId = headerAccessor.getSessionId();
        if (sessionId != null) {
            webSocketSessionCoordinator.handleDisconnect(sessionId);
        }
    }
}
