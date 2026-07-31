package org.thornex.musicparty.websocket;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import jakarta.annotation.PostConstruct;
import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.Gauge;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import org.thornex.musicparty.config.AppProperties;
import org.springframework.stereotype.Component;
import org.thornex.musicparty.service.UserService;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Sinks;

import java.util.Map;
import java.util.AbstractQueue;
import java.util.ArrayDeque;
import java.util.Iterator;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

@Component
@RequiredArgsConstructor
@Slf4j
public class ReactiveSocketBroker {

    private final ObjectMapper objectMapper;
    private final UserService userService;
    private final AppProperties appProperties;
    private final MeterRegistry meterRegistry;
    private static final Set<String> COALESCIBLE_TYPES = Set.of("player.state", "player.queue", "users.online", "rooms.list", "player.progress");
    private final Map<String, Sinks.Many<OutboundMessage>> sessions = new ConcurrentHashMap<>();
    private final Map<String, Set<String>> roomSessions = new ConcurrentHashMap<>();

    @PostConstruct
    void registerMetrics() {
        Gauge.builder("musicparty.websocket.connections.active", sessions, Map::size).register(meterRegistry);
        Gauge.builder("musicparty.websocket.rooms.subscribed", roomSessions, Map::size).register(meterRegistry);
    }

    public Flux<String> register(String sessionId, String roomId) {
        int capacity = appProperties.getPerformance().getWebsocketClientQueueCapacity();
        Sinks.Many<OutboundMessage> sink = Sinks.many().unicast().onBackpressureBuffer(new CoalescingQueue(capacity));
        sessions.put(sessionId, sink);
        meterRegistry.counter("musicparty.websocket.connections", "event", "connected").increment();
        subscribeRoom(sessionId, roomId);
        return sink.asFlux().map(OutboundMessage::serialized);
    }

    public void unregister(String sessionId) {
        Sinks.Many<OutboundMessage> sink = sessions.remove(sessionId);
        if (sink != null) {
            synchronized (sink) {
                sink.tryEmitComplete();
            }
            meterRegistry.counter("musicparty.websocket.connections", "event", "disconnected").increment();
        }
        roomSessions.values().forEach(sessions -> sessions.remove(sessionId));
        roomSessions.entrySet().removeIf(entry -> entry.getValue().isEmpty());
    }

    public void subscribeRoom(String sessionId, String roomId) {
        String normalizedRoomId = roomId == null || roomId.isBlank() ? userService.getRoomIdForSession(sessionId) : roomId;
        roomSessions.computeIfAbsent(normalizedRoomId, ignored -> ConcurrentHashMap.newKeySet()).add(sessionId);
    }

    public void sendToSession(String sessionId, String type, Object payload) {
        sendToSession(sessionId, type, null, payload);
    }

    public void sendToSession(String sessionId, String type, String roomId, Object payload) {
        Sinks.Many<OutboundMessage> sink = sessions.get(sessionId);
        if (sink == null) {
            return;
        }
        emit(sessionId, sink, type, new SocketEnvelope(type, null, roomId, payload));
    }

    public void broadcastRoom(String roomId, String type, Object payload) {
        Set<String> subscribers = roomSessions.get(roomId);
        if (subscribers == null || subscribers.isEmpty()) {
            return;
        }
        String serialized = serialize(type, new SocketEnvelope(type, null, roomId, payload));
        if (serialized == null) return;
        recordBroadcast(type, serialized);
        subscribers.forEach(sessionId -> {
            Sinks.Many<OutboundMessage> sink = sessions.get(sessionId);
            if (sink != null) {
                emitSerialized(sessionId, sink, type, serialized);
            }
        });
    }

    public void broadcastAll(String type, Object payload) {
        String serialized = serialize(type, new SocketEnvelope(type, null, null, payload));
        if (serialized == null) return;
        recordBroadcast(type, serialized);
        sessions.forEach((sessionId, sink) -> emitSerialized(sessionId, sink, type, serialized));
    }

    public int getSessionCount() {
        return sessions.size();
    }

    public int getSubscribedRoomCount() {
        return roomSessions.size();
    }

    private void emit(String sessionId, Sinks.Many<OutboundMessage> sink, String type, SocketEnvelope envelope) {
        String serialized = serialize(type, envelope);
        if (serialized != null) emitSerialized(sessionId, sink, type, serialized);
    }

    private String serialize(String type, SocketEnvelope envelope) {
        Timer.Sample sample = Timer.start(meterRegistry);
        try {
            String serialized = objectMapper.writeValueAsString(envelope);
            meterRegistry.summary("musicparty.websocket.serialization.bytes", "type", type).record(serialized.length());
            return serialized;
        } catch (JsonProcessingException ex) {
            meterRegistry.counter("musicparty.websocket.serialization.failures", "type", type).increment();
            log.warn("Failed to serialize websocket envelope type={}", type, ex);
            return null;
        } finally {
            sample.stop(meterRegistry.timer("musicparty.websocket.serialization", "type", type));
        }
    }

    private void emitSerialized(String sessionId, Sinks.Many<OutboundMessage> sink, String type, String serialized) {
        // A room broadcast and a direct resync can arrive on different Netty threads.
        // Unicast sinks reject concurrent producers with FAIL_NON_SERIALIZED; serialize
        // only the tiny enqueue operation, never the actual socket write.
        Sinks.EmitResult result;
        synchronized (sink) {
            result = sink.tryEmitNext(new OutboundMessage(type, serialized));
        }
        if (result == Sinks.EmitResult.FAIL_OVERFLOW) {
            // A bounded sink prevents a stalled client from retaining an unbounded backlog.
            log.info("Disconnecting slow websocket client: sessionId={}, queueDepth={}, messageType={}", sessionId,
                    appProperties.getPerformance().getWebsocketClientQueueCapacity(), type);
            meterRegistry.counter("musicparty.websocket.slow_client_disconnects", "type", type).increment();
            synchronized (sink) {
                sink.tryEmitError(new IllegalStateException("WebSocket client queue is full"));
            }
        } else if (result != Sinks.EmitResult.OK && result != Sinks.EmitResult.FAIL_ZERO_SUBSCRIBER) {
            meterRegistry.counter("musicparty.websocket.send_failures", "type", type, "result", result.name()).increment();
            log.debug("Websocket send failed: messageType={}, result={}", type, result);
        }
    }

    private void recordBroadcast(String type, String serialized) {
        Counter.builder("musicparty.websocket.broadcasts").tag("type", type).register(meterRegistry).increment();
        meterRegistry.summary("musicparty.websocket.broadcast.bytes", "type", type).record(serialized.length());
    }

    private record OutboundMessage(String type, String serialized) {}

    /**
     * State updates are replaceable, while reliable events retain FIFO order and cannot be displaced.
     */
    private static final class CoalescingQueue extends AbstractQueue<OutboundMessage> {
        private final ArrayDeque<OutboundMessage> messages = new ArrayDeque<>();
        private final int capacity;

        private CoalescingQueue(int capacity) {
            this.capacity = Math.max(1, capacity);
        }

        @Override
        public synchronized boolean offer(OutboundMessage message) {
            if (COALESCIBLE_TYPES.contains(message.type())) {
                messages.removeIf(existing -> existing.type().equals(message.type()));
            }
            if (messages.size() >= capacity) return false;
            messages.addLast(message);
            return true;
        }

        @Override
        public synchronized OutboundMessage poll() {
            return messages.pollFirst();
        }

        @Override
        public synchronized OutboundMessage peek() {
            return messages.peekFirst();
        }

        @Override
        public synchronized Iterator<OutboundMessage> iterator() {
            return new ArrayDeque<>(messages).iterator();
        }

        @Override
        public synchronized int size() {
            return messages.size();
        }
    }
}
