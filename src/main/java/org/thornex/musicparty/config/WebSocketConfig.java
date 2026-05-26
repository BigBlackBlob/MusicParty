package org.thornex.musicparty.config;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.HandlerMapping;
import org.springframework.web.reactive.handler.SimpleUrlHandlerMapping;
import org.springframework.web.reactive.socket.WebSocketHandler;
import org.springframework.web.reactive.socket.WebSocketSession;
import org.springframework.web.reactive.socket.server.support.WebSocketHandlerAdapter;
import org.thornex.musicparty.controller.MusicSocketController;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.WebSocketSessionCoordinator;
import org.thornex.musicparty.websocket.ReactiveSocketBroker;
import reactor.core.publisher.Mono;
import reactor.core.scheduler.Schedulers;

import java.net.URI;
import java.util.Map;

@Configuration
@RequiredArgsConstructor
@Slf4j
public class WebSocketConfig {

    private final ReactiveSocketBroker broker;
    private final WebSocketSessionCoordinator coordinator;
    private final MusicSocketController controller;
    private final ObjectMapper objectMapper;
    private final AccountService accountService;
    private final RoomService roomService;
    private final RoomAccessService roomAccessService;

    @Bean
    HandlerMapping webSocketHandlerMapping() {
        SimpleUrlHandlerMapping mapping = new SimpleUrlHandlerMapping(Map.of("/ws", webSocketHandler()));
        mapping.setOrder(-1);
        return mapping;
    }

    @Bean
    WebSocketHandlerAdapter webSocketHandlerAdapter() {
        return new WebSocketHandlerAdapter();
    }

    WebSocketHandler webSocketHandler() {
        return session -> authorize(session)
                .flatMap(authorized -> authorized ? handleAuthorized(session) : closeForbidden(session));
    }

    private Mono<Boolean> authorize(WebSocketSession session) {
        return Mono.fromCallable(() -> {
            String roomId = firstQuery(session, "room-id", "roomId");
            var metadata = roomService.getRoomAccessMetadata(roomId).orElse(null);
            if (metadata == null) {
                return false;
            }
            String sessionToken = firstQuery(session, "session-token", "sessionToken");
            var accountSession = accountService.resolveSession(sessionToken).orElse(null);
            if (accountSession == null) {
                return false;
            }
            if (!metadata.privateRoom()) {
                return true;
            }
            String roomAccessToken = firstQuery(session, "room-access-token", "roomAccessToken");
            return StringUtils.hasText(accountSession.publicId())
                    && roomAccessService.validateAccessToken(metadata.roomId(), accountSession.publicId(), roomAccessToken);
        }).subscribeOn(Schedulers.boundedElastic());
    }

    private Mono<Void> handleAuthorized(WebSocketSession session) {
        String sessionId = session.getId();
        String initialName = firstQuery(session, "user-name", "userName");
        String sessionToken = firstQuery(session, "session-token", "sessionToken");
        String roomId = firstQuery(session, "room-id", "roomId");
        coordinator.handleConnect(sessionId, sessionToken, initialName, roomId);

        Mono<Void> inbound = session.receive()
                .map(message -> message.getPayloadAsText())
                .flatMap(text -> dispatch(sessionId, text))
                .then();
        Mono<Void> outbound = session.send(broker.register(sessionId, roomId).map(session::textMessage));

        return Mono.when(inbound, outbound)
                .doFinally(signal -> {
                    broker.unregister(sessionId);
                    coordinator.handleDisconnect(sessionId);
                });
    }

    private Mono<Void> closeForbidden(WebSocketSession session) {
        return session.close(org.springframework.web.reactive.socket.CloseStatus.POLICY_VIOLATION);
    }

    private Mono<Void> dispatch(String sessionId, String text) {
        return Mono.fromRunnable(() -> {
            try {
                JsonNode root = objectMapper.readTree(text);
                String type = root.path("type").asText("");
                JsonNode payload = root.path("payload");
                controller.dispatch(type, payload, sessionId);
            } catch (Exception ex) {
                log.warn("WebSocket message rejected for session {}: {}", sessionId, ex.getMessage());
            }
        }).subscribeOn(Schedulers.boundedElastic()).then();
    }

    private String firstQuery(WebSocketSession session, String... names) {
        String query = URI.create(session.getHandshakeInfo().getUri().toString()).getRawQuery();
        if (!StringUtils.hasText(query)) {
            return null;
        }
        for (String part : query.split("&")) {
            String[] pair = part.split("=", 2);
            if (pair.length == 0) {
                continue;
            }
            for (String name : names) {
                if (name.equals(pair[0])) {
                    return pair.length > 1 ? java.net.URLDecoder.decode(pair[1], java.nio.charset.StandardCharsets.UTF_8) : "";
                }
            }
        }
        return null;
    }
}
