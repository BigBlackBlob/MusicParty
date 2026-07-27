package org.thornex.musicparty.service;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;

import java.util.concurrent.ExecutionException;
import java.util.concurrent.ThreadPoolExecutor;

/**
 * The single admission point for SQLite mutations.  SQLite WAL allows readers
 * alongside a writer, but a read transaction cannot safely be upgraded while
 * another connection commits.  Keeping each application write transaction on
 * this executor prevents those upgrade races.
 */
@Service
public class DatabaseWriteExecutor {
    private static final String WRITE_THREAD_PREFIX = "mp-db-write";

    private final ThreadPoolExecutor executor;

    public DatabaseWriteExecutor(@Qualifier("dbWriteExecutor") ThreadPoolExecutor executor) {
        this.executor = executor;
    }

    public void execute(Runnable operation) {
        call(() -> {
            operation.run();
            return null;
        });
    }

    public <T> T call(java.util.concurrent.Callable<T> operation) {
        if (Thread.currentThread().getName().startsWith(WRITE_THREAD_PREFIX)) {
            try {
                return operation.call();
            } catch (Exception ex) {
                throw propagate(ex);
            }
        }
        try {
            return executor.submit(operation).get();
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException("Interrupted while waiting for SQLite write", ex);
        } catch (ExecutionException ex) {
            throw propagate(ex.getCause());
        }
    }

    private RuntimeException propagate(Throwable error) {
        if (error instanceof RuntimeException runtimeException) {
            return runtimeException;
        }
        return new IllegalStateException("SQLite write failed", error);
    }
}
