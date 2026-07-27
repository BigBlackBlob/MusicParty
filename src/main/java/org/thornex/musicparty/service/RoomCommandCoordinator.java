package org.thornex.musicparty.service;

import jakarta.annotation.PreDestroy;
import io.micrometer.core.instrument.Gauge;
import io.micrometer.core.instrument.MeterRegistry;
import org.thornex.musicparty.config.AppProperties;
import org.springframework.stereotype.Component;
import org.springframework.beans.factory.annotation.Qualifier;

import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicBoolean;

@Component
public class RoomCommandCoordinator {
    private final Map<String, QueueState> queues = new ConcurrentHashMap<>();
    private final ExecutorService executor;
    private final ScheduledExecutorService timeoutExecutor = Executors.newSingleThreadScheduledExecutor();
    private final MeterRegistry meterRegistry;
    private final int maxPendingCommands;
    private final long commandQueueTimeoutMs;

    public RoomCommandCoordinator(AppProperties appProperties, MeterRegistry meterRegistry,
                                  @Qualifier("dbReadExecutor") ExecutorService executor) {
        this.meterRegistry = meterRegistry;
        this.executor = executor;
        this.maxPendingCommands = appProperties.getPerformance().getRoomCommandQueueCapacity();
        this.commandQueueTimeoutMs = appProperties.getPerformance().getRoomCommandQueueTimeoutMs();
        Gauge.builder("musicparty.room_command_queue.depth", queues,
                        states -> states.values().stream().mapToInt(state -> state.pendingCount.get()).sum())
                .register(meterRegistry);
    }

    public <T> CompletableFuture<T> executeAsync(String roomId, Callable<T> command) {
        QueueState state = queues.computeIfAbsent(roomId, ignored -> new QueueState());
        CompletableFuture<T> result = new CompletableFuture<>();
        if (state.closed.get()) {
            result.completeExceptionally(new CommandRejectedException("ROOM_CLOSED"));
            return result;
        }
        if (state.pendingCount.incrementAndGet() > maxPendingCommands) {
            state.pendingCount.decrementAndGet();
            result.completeExceptionally(new CommandRejectedException("QUEUE_FULL"));
            meterRegistry.counter("musicparty.room_command_queue.rejected", "reason", "full").increment();
            return result;
        }
        PendingCommand<T> pending = new PendingCommand<>(command, result);
        state.pending.add(pending);
        if (state.closed.get() && state.pending.remove(pending)) {
            pending.cancel("ROOM_CLOSED");
            releaseSlot(state, pending);
            return result;
        }
        timeoutExecutor.schedule(() -> {
            if (pending.expire()) {
                releaseSlot(state, pending);
                meterRegistry.counter("musicparty.room_command_queue.rejected", "reason", "timeout").increment();
            }
        }, commandQueueTimeoutMs, TimeUnit.MILLISECONDS);
        drain(state);
        return result;
    }

    public <T> T execute(String roomId, Callable<T> command) {
        return executeAsync(roomId, command).join();
    }

    public void closeRoom(String roomId) {
        QueueState state = queues.remove(roomId);
        if (state == null) return;
        state.closed.set(true);
        PendingCommand<?> pending;
        while ((pending = state.pending.poll()) != null) {
            pending.cancel("ROOM_CLOSED");
            releaseSlot(state, pending);
        }
    }

    private void drain(QueueState state) {
        if (!state.draining.compareAndSet(false, true)) return;
        executor.execute(() -> {
            try {
                PendingCommand<?> next;
                while (!state.closed.get() && (next = state.pending.poll()) != null) {
                    releaseSlot(state, next);
                    meterRegistry.timer("musicparty.room_command_queue.wait").record(System.nanoTime() - next.enqueuedAtNanos, TimeUnit.NANOSECONDS);
                    long executionStart = System.nanoTime();
                    next.run();
                    meterRegistry.timer("musicparty.room_command_queue.execution").record(System.nanoTime() - executionStart, TimeUnit.NANOSECONDS);
                }
            } finally {
                state.draining.set(false);
                if (!state.closed.get() && !state.pending.isEmpty()) drain(state);
            }
        });
    }

    @PreDestroy
    void shutdown() {
        queues.keySet().forEach(this::closeRoom);
        timeoutExecutor.shutdownNow();
    }

    private static final class QueueState {
        final ConcurrentLinkedQueue<PendingCommand<?>> pending = new ConcurrentLinkedQueue<>();
        final AtomicInteger pendingCount = new AtomicInteger();
        final AtomicBoolean draining = new AtomicBoolean();
        final AtomicBoolean closed = new AtomicBoolean();
    }

    private void releaseSlot(QueueState state, PendingCommand<?> pending) {
        if (pending.releaseSlot()) state.pendingCount.decrementAndGet();
    }

    public static final class CommandRejectedException extends RuntimeException {
        public CommandRejectedException(String reason) { super(reason); }
    }

    private static final class PendingCommand<T> {
        private final Callable<T> command;
        private final CompletableFuture<T> result;
        private final AtomicBoolean slotReleased = new AtomicBoolean();
        private final long enqueuedAtNanos = System.nanoTime();

        private PendingCommand(Callable<T> command, CompletableFuture<T> result) {
            this.command = command;
            this.result = result;
        }

        private boolean expire() {
            return result.completeExceptionally(new TimeoutException("Room command queue wait timed out"));
        }

        private void cancel(String reason) {
            result.completeExceptionally(new CommandRejectedException(reason));
        }

        private void run() {
            if (result.isDone()) return;
            try { result.complete(command.call()); }
            catch (Throwable error) { result.completeExceptionally(error); }
        }

        private boolean releaseSlot() {
            return slotReleased.compareAndSet(false, true);
        }
    }
}
