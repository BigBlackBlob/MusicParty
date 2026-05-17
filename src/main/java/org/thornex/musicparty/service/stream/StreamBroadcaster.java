package org.thornex.musicparty.service.stream;

import lombok.extern.slf4j.Slf4j;

import java.io.IOException;
import java.io.OutputStream;
import java.util.Arrays;
import java.util.Map;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.ThreadFactory;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.Consumer;

/**
 * 负责将音频数据分发给多个 HTTP 连接
 */
@Slf4j
public class StreamBroadcaster {

    private static final int CLIENT_QUEUE_CAPACITY = 64;
    private final Map<OutputStream, ClientSink> clients = new ConcurrentHashMap<>();
    private final ExecutorService clientWriterExecutor = Executors.newCachedThreadPool(new NamedThreadFactory());
    private Consumer<OutputStream> onClientRemoved;

    public void setOnClientRemoved(Consumer<OutputStream> onClientRemoved) {
        this.onClientRemoved = onClientRemoved;
    }
    
    public void addClient(OutputStream os) {
        ClientSink sink = new ClientSink(os);
        ClientSink existing = clients.put(os, sink);
        if (existing != null) {
            existing.close(false);
        }
        sink.start();
        log.info("Stream client connected. Total: {}", clients.size());
    }

    public void removeClient(OutputStream os) {
        ClientSink sink = clients.remove(os);
        if (sink != null) {
            sink.close(true);
            log.info("Stream client disconnected. Total: {}", clients.size());
            if (onClientRemoved != null) {
                onClientRemoved.accept(os);
            }
        }
    }

    public int getClientCount() {
        return clients.size();
    }

    public void broadcast(byte[] data, int length) {
        if (clients.isEmpty()) return;

        byte[] chunk = Arrays.copyOf(data, length);
        for (ClientSink sink : clients.values()) {
            if (!sink.offer(chunk)) {
                log.warn("Stream client too slow, disconnecting.");
                removeClient(sink.outputStream);
            }
        }
    }

    public void shutdown() {
        for (OutputStream client : clients.keySet()) {
            removeClient(client);
        }
        clientWriterExecutor.shutdownNow();
    }
    
    public void sendSilence() {
        // Implementation dependent:
        // 如果客户端是 VRChat，不发送数据通常会导致它暂停等待缓冲
        // 如果发送全0数据，MP3解码器可能会报错或产生噪音
        // 最佳实践：当没有数据时，什么都不做，让 ffmpeg 进程结束后自然停止写入，
        // 或者让 LiveStreamService 在空闲时挂起一个生成静音的 ffmpeg 进程。
    }

    private class ClientSink {
        private final OutputStream outputStream;
        private final ArrayBlockingQueue<byte[]> queue = new ArrayBlockingQueue<>(CLIENT_QUEUE_CAPACITY);
        private final AtomicBoolean active = new AtomicBoolean(true);
        private Future<?> writerFuture;

        private ClientSink(OutputStream outputStream) {
            this.outputStream = outputStream;
        }

        private void start() {
            writerFuture = clientWriterExecutor.submit(() -> {
                while (active.get() && !Thread.currentThread().isInterrupted()) {
                    try {
                        byte[] chunk = queue.take();
                        outputStream.write(chunk);
                        outputStream.flush();
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    } catch (IOException e) {
                        removeClient(outputStream);
                        return;
                    }
                }
            });
        }

        private boolean offer(byte[] chunk) {
            return active.get() && queue.offer(chunk);
        }

        private void close(boolean closeStream) {
            active.set(false);
            if (writerFuture != null) {
                writerFuture.cancel(true);
            }
            if (closeStream) {
                try {
                    outputStream.close();
                } catch (IOException ignored) {
                }
            }
        }
    }

    private static class NamedThreadFactory implements ThreadFactory {
        private final AtomicInteger counter = new AtomicInteger();

        @Override
        public Thread newThread(Runnable runnable) {
            Thread thread = new Thread(runnable, "stream-client-writer-" + counter.incrementAndGet());
            thread.setDaemon(true);
            return thread;
        }
    }
}
