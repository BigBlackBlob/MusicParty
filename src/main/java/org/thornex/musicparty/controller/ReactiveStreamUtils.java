package org.thornex.musicparty.controller;

import org.springframework.core.io.buffer.DataBuffer;
import org.springframework.core.io.buffer.DataBufferUtils;
import org.springframework.core.io.buffer.DefaultDataBufferFactory;
import org.slf4j.Logger;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.io.InputStream;
import java.io.IOException;
import java.nio.file.Path;
import java.nio.file.Files;

final class ReactiveStreamUtils {
    static final int BUFFER_SIZE = 8192;

    private ReactiveStreamUtils() {
    }

    static Flux<DataBuffer> readInputStream(InputStream inputStream) {
        return DataBufferUtils.readInputStream(() -> inputStream, new DefaultDataBufferFactory(), BUFFER_SIZE);
    }

    /**
     * Wraps a streaming Flux so mid-stream errors are logged and the Flux
     * completes silently instead of propagating to Spring's error handler.
     * This prevents HttpMessageNotWritableException when the response is
     * already committed (e.g. 206 PARTIAL_CONTENT).
     */
    static Flux<DataBuffer> withErrorSuppression(Flux<DataBuffer> body, String context, Logger logger) {
        return body.onErrorResume(e -> {
            logger.warn("Stream error after response committed (suppressing): {}, message={}", context, e.getMessage());
            return Mono.empty();
        });
    }

    static Flux<DataBuffer> readPath(Path path, long position, long count) {
        return DataBufferUtils.readInputStream(() -> {
            InputStream inputStream = Files.newInputStream(path);
            long skipped = inputStream.skip(position);
            while (skipped < position) {
                long next = inputStream.skip(position - skipped);
                if (next <= 0) break;
                skipped += next;
            }
            return new LimitedInputStream(inputStream, count);
        }, new DefaultDataBufferFactory(), BUFFER_SIZE);
    }

    private static final class LimitedInputStream extends InputStream {
        private final InputStream delegate;
        private long remaining;

        private LimitedInputStream(InputStream delegate, long remaining) {
            this.delegate = delegate;
            this.remaining = remaining;
        }

        @Override
        public int read() throws IOException {
            if (remaining <= 0) {
                return -1;
            }
            int value = delegate.read();
            if (value >= 0) {
                remaining--;
            }
            return value;
        }

        @Override
        public int read(byte[] b, int off, int len) throws IOException {
            if (remaining <= 0) {
                return -1;
            }
            int read = delegate.read(b, off, (int) Math.min(len, remaining));
            if (read > 0) {
                remaining -= read;
            }
            return read;
        }

        @Override
        public void close() throws IOException {
            delegate.close();
        }
    }
}
