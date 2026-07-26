package org.thornex.musicparty.service;

import jakarta.annotation.PreDestroy;
import org.springframework.stereotype.Component;

import java.util.Map;
import java.util.concurrent.Callable;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicBoolean;

@Component
public class RoomCommandCoordinator {
    private final Map<String, QueueState> queues = new ConcurrentHashMap<>();
    private final ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();

    public <T> CompletableFuture<T> executeAsync(String roomId, Callable<T> command) {
        QueueState state = queues.computeIfAbsent(roomId, ignored -> new QueueState());
        CompletableFuture<T> result = new CompletableFuture<>();
        state.pending.add(() -> {
            try { result.complete(command.call()); }
            catch (Throwable error) { result.completeExceptionally(error); }
        });
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
        state.pending.clear();
    }

    private void drain(QueueState state) {
        if (!state.draining.compareAndSet(false, true)) return;
        executor.execute(() -> {
            try {
                Runnable next;
                while (!state.closed.get() && (next = state.pending.poll()) != null) next.run();
            } finally {
                state.draining.set(false);
                if (!state.closed.get() && !state.pending.isEmpty()) drain(state);
            }
        });
    }

    @PreDestroy
    void shutdown() { executor.shutdownNow(); }

    private static final class QueueState {
        final ConcurrentLinkedQueue<Runnable> pending = new ConcurrentLinkedQueue<>();
        final AtomicBoolean draining = new AtomicBoolean();
        final AtomicBoolean closed = new AtomicBoolean();
    }
}
