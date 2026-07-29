package org.thornex.musicparty.service;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;

import java.util.concurrent.ExecutionException;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;

/**
 * The single admission point for SQLite mutations.  SQLite WAL allows readers
 * alongside a writer, but a read transaction cannot safely be upgraded while
 * another connection commits.  Keeping each application write transaction on
 * this executor prevents those upgrade races.
 */
@Service
@Slf4j
public class DatabaseWriteExecutor {
    private static final String WRITE_THREAD_PREFIX = "mp-db-write";

    private final ThreadPoolExecutor executor;
    private final Timer waitTimer;
    private final Timer executionTimer;
    private final Counter successCounter;
    private final Counter failureCounter;

    public DatabaseWriteExecutor(@Qualifier("dbWriteExecutor") ThreadPoolExecutor executor) {
        this(executor, null);
    }

    @Autowired
    public DatabaseWriteExecutor(@Qualifier("dbWriteExecutor") ThreadPoolExecutor executor,
                                 MeterRegistry meterRegistry) {
        this.executor = executor;
        this.waitTimer = meterRegistry == null ? null : meterRegistry.timer("musicparty.db.write.wait");
        this.executionTimer = meterRegistry == null ? null : meterRegistry.timer("musicparty.db.write.duration");
        this.successCounter = meterRegistry == null ? null : meterRegistry.counter("musicparty.db.write", "outcome", "success");
        this.failureCounter = meterRegistry == null ? null : meterRegistry.counter("musicparty.db.write", "outcome", "failure");
    }

    public void execute(Runnable operation) {
        call(() -> {
            operation.run();
            return null;
        });
    }

    public <T> T call(java.util.concurrent.Callable<T> operation) {
        if (Thread.currentThread().getName().startsWith(WRITE_THREAD_PREFIX)) {
            return executeMeasured(operation);
        }
        long waitStarted = System.nanoTime();
        try {
            return executor.submit(() -> {
                record(waitTimer, waitStarted);
                return executeMeasured(operation);
            }).get();
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("Interrupted while waiting for SQLite write", ex);
        } catch (ExecutionException ex) {
            throw propagate(ex.getCause());
        }
    }

    private <T> T executeMeasured(java.util.concurrent.Callable<T> operation) {
        long started = System.nanoTime();
        try {
            T result = operation.call();
            if (successCounter != null) {
                successCounter.increment();
            }
            return result;
        } catch (Exception ex) {
            if (failureCounter != null) {
                failureCounter.increment();
            }
            log.warn("SQLite writer operation failed: thread={}, queued={}",
                    Thread.currentThread().getName(), executor.getQueue().size(), ex);
            throw propagate(ex);
        } finally {
            record(executionTimer, started);
        }
    }

    private void record(Timer timer, long startedNanos) {
        if (timer != null) {
            timer.record(System.nanoTime() - startedNanos, TimeUnit.NANOSECONDS);
        }
    }

    private RuntimeException propagate(Throwable error) {
        if (error instanceof RuntimeException runtimeException) {
            return runtimeException;
        }
        return new IllegalStateException("SQLite write failed", error);
    }
}
