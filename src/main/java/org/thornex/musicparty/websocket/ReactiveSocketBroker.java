package org.thornex.musicparty.websocket;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import org.thornex.musicparty.service.UserService;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Sinks;

import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

@Component
@RequiredArgsConstructor
@Slf4j
public class ReactiveSocketBroker {

    private final ObjectMapper objectMapper;
    private final UserService userService;
    private final Map<String, Sinks.Many<String>> sessions = new ConcurrentHashMap<>();
    private final Map<String, Set<String>> roomSessions = new ConcurrentHashMap<>();

    public Flux<String> register(String sessionId, String roomId) {
        Sinks.Many<String> sink = Sinks.many().multicast().onBackpressureBuffer();
        sessions.put(sessionId, sink);
        subscribeRoom(sessionId, roomId);
        return sink.asFlux();
    }

    public void unregister(String sessionId) {
        Sinks.Many<String> sink = sessions.remove(sessionId);
        if (sink != null) {
            sink.tryEmitComplete();
        }
        roomSessions.values().forEach(sessions -> sessions.remove(sessionId));
    }

    public void subscribeRoom(String sessionId, String roomId) {
        String normalizedRoomId = roomId == null || roomId.isBlank() ? userService.getRoomIdForSession(sessionId) : roomId;
        roomSessions.computeIfAbsent(normalizedRoomId, ignored -> ConcurrentHashMap.newKeySet()).add(sessionId);
    }

    public void sendToSession(String sessionId, String type, Object payload) {
        sendToSession(sessionId, type, null, payload);
    }

    public void sendToSession(String sessionId, String type, String roomId, Object payload) {
        Sinks.Many<String> sink = sessions.get(sessionId);
        if (sink == null) {
            return;
        }
        emit(sink, new SocketEnvelope(type, null, roomId, payload));
    }

    public void broadcastRoom(String roomId, String type, Object payload) {
        Set<String> subscribers = roomSessions.get(roomId);
        if (subscribers == null || subscribers.isEmpty()) {
            return;
        }
        SocketEnvelope envelope = new SocketEnvelope(type, null, roomId, payload);
        subscribers.forEach(sessionId -> {
            Sinks.Many<String> sink = sessions.get(sessionId);
            if (sink != null) {
                emit(sink, envelope);
            }
        });
    }

    public void broadcastAll(String type, Object payload) {
        SocketEnvelope envelope = new SocketEnvelope(type, null, null, payload);
        sessions.values().forEach(sink -> emit(sink, envelope));
    }

    private void emit(Sinks.Many<String> sink, SocketEnvelope envelope) {
        try {
            sink.tryEmitNext(objectMapper.writeValueAsString(envelope));
        } catch (JsonProcessingException ex) {
            log.warn("Failed to serialize websocket envelope type={}", envelope.type(), ex);
        }
    }
}
