package org.thornex.musicparty.config;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.beans.factory.annotation.Qualifier;
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
import org.thornex.musicparty.security.SessionCookieService;
import org.thornex.musicparty.websocket.ReactiveSocketBroker;
import reactor.core.publisher.Mono;
import reactor.core.scheduler.Scheduler;

import java.net.URI;
import java.util.Arrays;
import java.util.Map;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.atomic.AtomicReference;

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
    private final SessionCookieService sessionCookieService;
    private final AppProperties appProperties;
    @Qualifier("dbReadScheduler")
    private final Scheduler dbReadScheduler;

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
            if (!isAllowedOrigin(session)) {
                return false;
            }
            var metadata = roomService.getRoomAccessMetadata(roomId).orElse(null);
            if (metadata == null) {
                return false;
            }
            String sessionToken = sessionCookieService.sessionToken(session);
            var accountSession = accountService.resolveSession(sessionToken).orElse(null);
            if (accountSession == null) {
                return false;
            }
            if (!metadata.privateRoom()) {
                return true;
            }
            return StringUtils.hasText(accountSession.publicId())
                    && roomAccessService.hasActiveGrant(metadata.roomId(), accountSession.publicId());
        }).subscribeOn(dbReadScheduler);
    }

    private Mono<Void> handleAuthorized(WebSocketSession session) {
        String sessionId = session.getId();
        String initialName = null;
        String sessionToken = sessionCookieService.sessionToken(session);
        String roomId = firstQuery(session, "room-id", "roomId");
        String tokenFingerprint = sessionCookieService.sessionTokenFingerprint(session);
        coordinator.handleConnect(sessionId, sessionToken, initialName, roomId);
        AtomicReference<String> terminalError = new AtomicReference<>();

        Mono<Void> inbound = session.receive()
                .map(message -> message.getPayloadAsText())
                .concatMap(text -> dispatch(sessionId, text))
                .doOnError(error -> {
                    terminalError.compareAndSet(null, "inbound:" + error.getClass().getSimpleName());
                    log.warn("WebSocket inbound failed: sessionId={}, error={}", sessionId, error.toString());
                })
                .then();
        Mono<Void> outbound = session.send(broker.register(sessionId, roomId).map(session::textMessage))
                .doOnError(error -> {
                    terminalError.compareAndSet(null, "outbound:" + error.getClass().getSimpleName());
                    log.warn("WebSocket outbound failed: sessionId={}, error={}", sessionId, error.toString());
                });

        return Mono.when(inbound, outbound)
                .doFinally(signal -> {
                    log.info("WebSocket session finished: sessionId={}, tokenFingerprint={}, signal={}, terminalError={}",
                            sessionId, tokenFingerprint, signal, terminalError.get());
                    broker.unregister(sessionId);
                    coordinator.handleDisconnect(sessionId);
                });
    }

    private Mono<Void> closeForbidden(WebSocketSession session) {
        return session.close(org.springframework.web.reactive.socket.CloseStatus.POLICY_VIOLATION);
    }

    private Mono<Void> dispatch(String sessionId, String text) {
        Mono<Void> command = Mono.defer(() -> {
            try {
                JsonNode root = objectMapper.readTree(text);
                String type = root.path("type").asText("");
                JsonNode payload = root.path("payload");
                Mono<Void> dispatch = Mono.fromFuture(controller.dispatchAsync(type, payload, sessionId));
                // Resync and presence requests only inspect in-memory room/session state.
                // Scheduling each of them on the small blocking DB pool turns a reconnect
                // burst into rejected work and closes otherwise healthy sockets.
                return isInMemorySocketCommand(type) ? dispatch : dispatch.subscribeOn(dbReadScheduler);
            } catch (Exception ex) {
                log.warn("WebSocket message rejected for session {}: {}", sessionId, ex.getMessage());
                return Mono.empty();
            }
        }).onErrorResume(this::isSchedulerRejection, error -> {
            log.warn("WebSocket command deferred because the DB read scheduler is saturated: sessionId={}", sessionId);
            return Mono.empty();
        });
        return command.then();
    }

    private boolean isInMemorySocketCommand(String type) {
        return switch (type) {
            case "player.resync", "/player/resync", "sync.ping", "/sync/ping",
                    "user.me", "/user/me", "users.online", "/users/online" -> true;
            default -> false;
        };
    }

    private boolean isSchedulerRejection(Throwable error) {
        Throwable current = error;
        while (current != null) {
            if (current instanceof RejectedExecutionException) {
                return true;
            }
            current = current.getCause();
        }
        return false;
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

    private boolean isAllowedOrigin(WebSocketSession session) {
        String origin = session.getHandshakeInfo().getHeaders().getOrigin();
        if (!StringUtils.hasText(origin)) return false;
        String socketOrigin = session.getHandshakeInfo().getUri().getScheme() + "://" + session.getHandshakeInfo().getUri().getAuthority();
        if (origin.equals(socketOrigin)) return true;
        return Arrays.stream(appProperties.getAllowedOrigins().split(","))
                .map(String::trim)
                .filter(StringUtils::hasText)
                .anyMatch(origin::equals);
    }
}
