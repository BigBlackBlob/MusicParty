package org.thornex.musicparty.service;

import jakarta.annotation.PostConstruct;
import jakarta.annotation.PreDestroy;
import lombok.Data;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.core.io.buffer.DataBuffer;
import org.springframework.core.io.buffer.DataBufferUtils;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientRequestException;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Gauge;
import org.thornex.musicparty.config.LocalResourceConfig;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.enums.CacheStatus;
import org.thornex.musicparty.event.DownloadStatusEvent;
import reactor.core.Disposable;
import reactor.core.publisher.Mono;
import reactor.core.publisher.Sinks;
import reactor.core.scheduler.Scheduler;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.AtomicMoveNotSupportedException;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardCopyOption;
import java.nio.file.StandardOpenOption;
import java.time.Duration;
import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.Semaphore;

@Service
@Slf4j
public class LocalCacheService {

    private final WebClient webClient;
    private static final long DOWNLOAD_COOLDOWN_SECONDS = 3;

    // 内存中维护缓存文件的元数据
    private final Map<String, CacheEntry> cacheIndex = new ConcurrentHashMap<>();
    private final AtomicLong currentTotalSize = new AtomicLong(0);
    private final ApplicationEventPublisher eventPublisher;
    private final AppProperties appProperties;
    private final Scheduler fileIoScheduler;
    private final Scheduler subprocessScheduler;
    private final java.util.concurrent.Executor subprocessExecutor;
    private final MeterRegistry meterRegistry;
    private final Sinks.Many<DownloadTask> downloadQueue;
    private final AtomicInteger pendingDownloadTasks = new AtomicInteger(0);
    private final AtomicLong reservedDownloadBytes = new AtomicLong(0);
    private final AtomicInteger activeDownloadTasks = new AtomicInteger(0);
    private final Map<String, Semaphore> platformPermits = new ConcurrentHashMap<>();
    private final Map<String, Process> runningProcesses = new ConcurrentHashMap<>();
    private final Object capacityLock = new Object();
    private Disposable queueSubscription;
    private static final int COMMAND_OUTPUT_LIMIT = 16 * 1024;

    private record DownloadTask(
            String musicId,
            String platform,
            Mono<DownloadSource> sourceProvider,
            AtomicBoolean started
    ) {}

    public enum DownloadSubmission { QUEUED, ALREADY_PRESENT, REJECTED_QUEUE_FULL, REJECTED_CAPACITY, REJECTED_EMIT }

    public record DownloadSource(
            String url,
            Map<String, String> headers,
            String extension,
            List<String> command
    ) {
        public DownloadSource(String url, Map<String, String> headers, String extension) {
            this(url, headers, extension, List.of());
        }

        public static DownloadSource command(String url, String extension, List<String> command) {
            return new DownloadSource(url, Map.of(), extension, command);
        }

        public boolean usesCommand() {
            return command != null && !command.isEmpty();
        }
    }

    public LocalCacheService(WebClient webClient, ApplicationEventPublisher eventPublisher, AppProperties appProperties) {
        this(webClient, eventPublisher, appProperties, reactor.core.scheduler.Schedulers.boundedElastic(),
                reactor.core.scheduler.Schedulers.boundedElastic(), CompletableFuture.delayedExecutor(0, TimeUnit.MILLISECONDS),
                new io.micrometer.core.instrument.simple.SimpleMeterRegistry());
    }

    @Autowired
    public LocalCacheService(WebClient webClient, ApplicationEventPublisher eventPublisher, AppProperties appProperties,
                             @Qualifier("fileIoScheduler") Scheduler fileIoScheduler,
                             @Qualifier("subprocessScheduler") Scheduler subprocessScheduler,
                             @Qualifier("subprocessExecutor") java.util.concurrent.Executor subprocessExecutor,
                             MeterRegistry meterRegistry) {
        this.webClient = webClient;
        this.eventPublisher = eventPublisher;
        this.appProperties = appProperties;
        this.fileIoScheduler = fileIoScheduler;
        this.subprocessScheduler = subprocessScheduler;
        this.subprocessExecutor = subprocessExecutor;
        this.meterRegistry = meterRegistry;
        this.downloadQueue = Sinks.many().unicast().onBackpressureBuffer(
                new ArrayBlockingQueue<>(Math.max(1, appProperties.getPerformance().getDownloadMaxQueuedTasks())));
        Gauge.builder("musicparty.download.queued", pendingDownloadTasks, AtomicInteger::get).register(meterRegistry);
        Gauge.builder("musicparty.download.active", activeDownloadTasks, AtomicInteger::get).register(meterRegistry);
        Gauge.builder("musicparty.download.reserved_bytes", reservedDownloadBytes, AtomicLong::get).register(meterRegistry);
    }

    @Data
    public static class CacheEntry {
        private String id;
        private volatile String fileName;
        private volatile CacheStatus status;
        private volatile long size;
        private volatile long lastAccessTime;
        private volatile String originalUrl; // 用于重试或记录
        private volatile long reservedBytes;
    }

    @PostConstruct
    public void init() {
        // 初始化时扫描目录，重建索引和计算大小
        File dir = new File(LocalResourceConfig.CACHE_DIR);
        if (!dir.exists()) dir.mkdirs();

        File[] files = dir.listFiles();
        if (files != null) {
            for (File f : files) {
                if (f.getName().endsWith(".part")) {
                    try {
                        Files.deleteIfExists(f.toPath());
                    } catch (IOException e) {
                        log.warn("Failed to delete stale partial cache file: {}", f.getName(), e);
                    }
                    continue;
                }
                String id = f.getName().split("\\.")[0]; // 假设文件名是 id.mp3
                CacheEntry entry = new CacheEntry();
                entry.setId(id);
                entry.setFileName(f.getName());
                entry.setStatus(CacheStatus.COMPLETED);
                entry.setSize(f.length());
                entry.setLastAccessTime(System.currentTimeMillis());

                cacheIndex.put(id, entry);
                currentTotalSize.addAndGet(f.length());
            }
        }
        log.info("LocalCacheService initialized. Current cache size: {} bytes", currentTotalSize.get());

        this.queueSubscription = downloadQueue.asFlux()
                .flatMap(task ->
                        processTask(task)
                                .onErrorResume(e -> {
                                    log.error("Download queue processor failed: phase=dispatch", e);
                                    return Mono.empty();
                                }), appProperties.getPerformance().getDownloadMaxActiveTasks())
                .subscribe();
    }

    @PreDestroy
    public void cleanup() {
        if (queueSubscription != null && !queueSubscription.isDisposed()) {
            queueSubscription.dispose();
        }
        runningProcesses.values().forEach(process -> process.destroyForcibly());
        runningProcesses.clear();
    }

    /**
     * 提交下载任务
     * @param musicId 音乐ID（作为文件名）
     * @param urlProvider 提供下载链接的 Mono（因为链接可能是动态获取的）
     * @param headers 下载需要的请求头（Referer, Cookie等）
     * @param extension 文件扩展名 (如 .m4a, .mp3)
     */
    public void submitDownload(String musicId, Mono<String> urlProvider, Map<String, String> headers, String extension) {
        submitDownload("unknown", musicId, urlProvider, headers, extension);
    }

    public DownloadSubmission submitDownload(String platform, String musicId, Mono<String> urlProvider, Map<String, String> headers, String extension) {
        return submitDynamicDownload(platform, musicId, urlProvider.map(url -> new DownloadSource(url, headers, extension)));
    }

    public void submitDynamicDownload(String musicId, Mono<DownloadSource> sourceProvider) {
        submitDynamicDownload("unknown", musicId, sourceProvider);
    }

    public DownloadSubmission submitDynamicDownload(String platform, String musicId, Mono<DownloadSource> sourceProvider) {
        CacheEntry entry;
        long reservation = appProperties.getPerformance().getDownloadReservationBytes();
        synchronized (capacityLock) {
            CacheEntry existing = cacheIndex.get(musicId);
            if (existing != null && (existing.getStatus() == CacheStatus.COMPLETED || isInFlight(existing.getStatus()))) {
                if (existing.getStatus() == CacheStatus.COMPLETED) touch(musicId);
                meterRegistry.counter("musicparty.download.submissions", "result", "deduplicated").increment();
                return DownloadSubmission.ALREADY_PRESENT;
            }
            if (!reserveSlotAndCapacity(reservation)) {
                meterRegistry.counter("musicparty.download.submissions", "result", "rejected").increment();
                return pendingDownloadTasks.get() >= appProperties.getPerformance().getDownloadMaxQueuedTasks()
                        ? DownloadSubmission.REJECTED_QUEUE_FULL : DownloadSubmission.REJECTED_CAPACITY;
            }
            entry = new CacheEntry();
            entry.setId(musicId);
            entry.setStatus(CacheStatus.PENDING);
            entry.setLastAccessTime(System.currentTimeMillis());
            entry.setReservedBytes(reservation);
            cacheIndex.put(musicId, entry);
        }

        eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
        Sinks.EmitResult result = downloadQueue.tryEmitNext(new DownloadTask(musicId, platform, sourceProvider, new AtomicBoolean()));
        if (result.isFailure()) {
            synchronized (capacityLock) {
                pendingDownloadTasks.decrementAndGet();
                releaseReservation(entry);
                entry.setStatus(CacheStatus.REJECTED);
            }
            eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
            meterRegistry.counter("musicparty.download.submissions", "result", "emit_failed").increment();
            return DownloadSubmission.REJECTED_EMIT;
        }
        meterRegistry.counter("musicparty.download.submissions", "result", "queued", "platform", platform).increment();
        return DownloadSubmission.QUEUED;
    }

    private Mono<Void> processTask(DownloadTask task) {
        if (task.started().compareAndSet(false, true)) {
            pendingDownloadTasks.updateAndGet(value -> Math.max(0, value - 1));
        }
        String musicId = task.musicId();
        CacheEntry entry = cacheIndex.get(musicId);

        // 双重检查：如果任务在排队期间被移除了，就跳过
        if (entry == null) return Mono.empty();

        Semaphore platformPermit = platformPermits.computeIfAbsent(task.platform(), this::platformPermit);
        if (!platformPermit.tryAcquire()) {
            entry.setStatus(CacheStatus.RETRY_WAIT);
            eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
            return Mono.delay(Duration.ofMillis(100)).then(processTask(task));
        }
        activeDownloadTasks.incrementAndGet();
        entry.setStatus(CacheStatus.RESOLVING);
        eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
        log.info("Processing download: platform={}, musicId={}, phase=resolving", task.platform(), musicId);

        return task.sourceProvider()
                .flatMap(source -> {
                    entry.setStatus(CacheStatus.DOWNLOADING);
                    eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
                    entry.setOriginalUrl(source.url());
                    String fileName = musicId + source.extension();
                    entry.setFileName(fileName);
                    Path destPath = Paths.get(LocalResourceConfig.CACHE_DIR, fileName);
                    Path partPath = Paths.get(LocalResourceConfig.CACHE_DIR, fileName + ".part");

                    log.info("Processing download: platform={}, musicId={}, phase={}", task.platform(), musicId,
                            source.usesCommand() ? "subprocess" : "downloading");
                    Mono<Void> download = source.usesCommand()
                            ? runDownloadCommand(musicId, source.command(), partPath)
                            : downloadWithRetries(source, partPath, task.platform(), musicId, entry, 0);

                    return download.publishOn(fileIoScheduler)
                            .doOnSuccess(unused -> completeDownload(entry, musicId, fileName, partPath, destPath));
                })
                // 错误处理
                .doOnError(error -> {
                    log.error("Download failed: platform={}, musicId={}, phase=download, error={}",
                            task.platform(), musicId, error.getClass().getSimpleName(), error);
                    try {
                        if (entry.getFileName() != null) {
                            Files.deleteIfExists(Paths.get(LocalResourceConfig.CACHE_DIR, entry.getFileName() + ".part"));
                        }
                    } catch (IOException cleanupError) {
                        log.warn("Failed to delete partial cache for {}", musicId, cleanupError);
                    }
                    entry.setStatus(CacheStatus.FAILED);
                    releaseReservation(entry);
                    eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
                })
                // 这里的 onErrorResume 保证即使这个任务失败，Flux 链也不会断，会继续执行 delay 和下一个任务
                .onErrorResume(e -> Mono.empty())
                .doFinally(ignored -> {
                    activeDownloadTasks.decrementAndGet();
                    platformPermit.release();
                })
                .then();
    }

    private Mono<Void> downloadWithWebClient(DownloadSource source, Path partPath) {
        return DataBufferUtils.write(
                webClient.get()
                        .uri(source.url())
                        .headers(httpHeaders -> {
                            if (source.headers() != null) source.headers().forEach(httpHeaders::add);
                        })
                        .retrieve()
                        .bodyToFlux(DataBuffer.class),
                partPath,
                StandardOpenOption.CREATE,
                StandardOpenOption.TRUNCATE_EXISTING,
                StandardOpenOption.WRITE
        );
    }

    private Mono<Void> downloadWithRetries(DownloadSource source, Path partPath, String platform,
                                           String musicId, CacheEntry entry, int retryCount) {
        return downloadWithWebClient(source, partPath)
                .onErrorResume(error -> {
                    Duration delay = retryDelay(error, retryCount);
                    if (delay == null) return Mono.error(error);
                    entry.setStatus(CacheStatus.RETRY_WAIT);
                    eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
                    meterRegistry.counter("musicparty.download.retries", "platform", platform,
                            "reason", retryReason(error)).increment();
                    log.warn("Retrying download: platform={}, musicId={}, retry={}, delayMs={}, reason={}",
                            platform, musicId, retryCount + 1, delay.toMillis(), retryReason(error));
                    return Mono.delay(delay)
                            .then(Mono.fromRunnable(() -> {
                                entry.setStatus(CacheStatus.DOWNLOADING);
                                eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
                            }))
                            .then(downloadWithRetries(source, partPath, platform, musicId, entry, retryCount + 1));
                });
    }

    private Duration retryDelay(Throwable error, int retryCount) {
        if (retryCount >= appProperties.getPerformance().getDownloadMaxRetries()) return null;
        if (error instanceof WebClientResponseException response) {
            if (response.getStatusCode().value() == 429) return retryAfter(response, retryCount);
            if (!response.getStatusCode().is5xxServerError()) return null;
        } else if (!(error instanceof WebClientRequestException)
                && !(error instanceof java.util.concurrent.TimeoutException)
                && !(error.getCause() instanceof java.util.concurrent.TimeoutException)) {
            return null;
        }
        long initial = Math.max(1, appProperties.getPerformance().getDownloadRetryInitialDelayMs());
        long capped = Math.min(appProperties.getPerformance().getDownloadRetryMaxDelayMs(),
                initial * (1L << Math.min(retryCount, 20)));
        return Duration.ofMillis(Math.max(1, (long) (capped * (0.8d + Math.random() * 0.4d))));
    }

    private Duration retryAfter(WebClientResponseException response, int retryCount) {
        String value = response.getHeaders().getFirst("Retry-After");
        if (value != null) {
            try {
                return boundedRetryDelay(Duration.ofSeconds(Math.max(0, Long.parseLong(value.trim()))));
            } catch (NumberFormatException ignored) {
                try {
                    long milliseconds = Duration.between(ZonedDateTime.now(),
                            ZonedDateTime.parse(value, DateTimeFormatter.RFC_1123_DATE_TIME)).toMillis();
                    return boundedRetryDelay(Duration.ofMillis(Math.max(0, milliseconds)));
                } catch (RuntimeException invalidDate) {
                    log.debug("Ignoring invalid Retry-After header: {}", value);
                }
            }
        }
        long initial = Math.max(1, appProperties.getPerformance().getDownloadRetryInitialDelayMs());
        long capped = Math.min(appProperties.getPerformance().getDownloadRetryMaxDelayMs(),
                initial * (1L << Math.min(retryCount, 20)));
        return Duration.ofMillis(Math.max(1, (long) (capped * (0.8d + Math.random() * 0.4d))));
    }

    private Duration boundedRetryDelay(Duration delay) {
        return delay.compareTo(Duration.ofMillis(appProperties.getPerformance().getDownloadRetryMaxDelayMs())) > 0
                ? Duration.ofMillis(appProperties.getPerformance().getDownloadRetryMaxDelayMs())
                : delay;
    }

    private String retryReason(Throwable error) {
        if (error instanceof WebClientResponseException response) return "http_" + response.getStatusCode().value();
        if (error instanceof WebClientRequestException) return "network";
        return "timeout";
    }

    private Mono<Void> runDownloadCommand(String musicId, List<String> command, Path outputPath) {
        return Mono.fromRunnable(() -> {
                    List<String> resolvedCommand = command.stream()
                            .map(arg -> arg.replace("{output}", outputPath.toString()))
                            .toList();
                    ProcessBuilder pb = new ProcessBuilder(resolvedCommand);
                    pb.redirectErrorStream(true);
                    try {
                        Process process = pb.start();
                        runningProcesses.put(musicId, process);
                        CompletableFuture<String> outputFuture = CompletableFuture.supplyAsync(() -> {
                            try {
                                return readProcessOutput(process.getInputStream());
                            } catch (IOException e) {
                                return "Failed to read command output: " + e.getMessage();
                            }
                        }, subprocessExecutor);
                        boolean finished = process.waitFor(15, TimeUnit.MINUTES);
                        if (!finished) {
                            process.destroyForcibly();
                            throw new RuntimeException("Download command timed out");
                        }
                        String output = outputFuture.get(5, TimeUnit.SECONDS);
                        if (process.exitValue() != 0) {
                            throw new RuntimeException("Download command failed: " + output);
                        }
                        runningProcesses.remove(musicId, process);
                    } catch (IOException e) {
                        throw new RuntimeException("Download command failed", e);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        throw new RuntimeException("Download command interrupted", e);
                    } catch (Exception e) {
                        throw new RuntimeException("Download command failed", e);
                    } finally {
                        runningProcesses.remove(musicId);
                    }
                })
                .subscribeOn(subprocessScheduler)
                .then();
    }

    private String readProcessOutput(java.io.InputStream inputStream) throws IOException {
        byte[] buffer = new byte[1024];
        byte[] captured = new byte[COMMAND_OUTPUT_LIMIT];
        int capturedLength = 0;
        int totalRead = 0;
        int read;
        while ((read = inputStream.read(buffer)) != -1) {
            totalRead += read;
            int copy = Math.min(read, COMMAND_OUTPUT_LIMIT - capturedLength);
            if (copy > 0) {
                System.arraycopy(buffer, 0, captured, capturedLength, copy);
                capturedLength += copy;
            }
        }
        String output = new String(captured, 0, capturedLength, java.nio.charset.StandardCharsets.UTF_8);
        if (totalRead > COMMAND_OUTPUT_LIMIT) {
            output += "\n... output truncated after " + COMMAND_OUTPUT_LIMIT + " bytes";
        }
        return output;
    }

    private void completeDownload(CacheEntry entry, String musicId, String fileName, Path partPath, Path destPath) {
        try {
            Path completedPath = resolveCompletedPath(fileName, partPath, destPath);
            if (!completedPath.equals(destPath)) {
                moveCompletedDownload(completedPath, destPath);
            }
            long size = Files.size(destPath);
            entry.setSize(size);
            entry.setStatus(CacheStatus.COMPLETED);
            releaseReservation(entry);
            currentTotalSize.addAndGet(size);
            log.info("Download completed: {}", fileName);
            eventPublisher.publishEvent(new DownloadStatusEvent(this, musicId));
            ensureCapacity();
        } catch (IOException e) {
            throw new RuntimeException("File write error: " + e.getMessage(), e);
        }
    }

    private Path resolveCompletedPath(String fileName, Path partPath, Path destPath) throws IOException {
        if (Files.exists(partPath)) return partPath;
        if (Files.exists(destPath)) return destPath;

        Path parent = partPath.getParent();
        if (parent != null && Files.isDirectory(parent)) {
            String partPrefix = partPath.getFileName().toString();
            try (var stream = Files.list(parent)) {
                return stream
                        .filter(path -> {
                            String name = path.getFileName().toString();
                            return name.equals(fileName) || name.startsWith(partPrefix);
                        })
                        .findFirst()
                        .orElseThrow(() -> new IOException("Downloaded file not found: expected " + partPath));
            }
        }
        throw new IOException("Downloaded file not found: expected " + partPath);
    }

    private void moveCompletedDownload(Path partPath, Path destPath) throws IOException {
        try {
            Files.move(partPath, destPath, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (AtomicMoveNotSupportedException e) {
            Files.move(partPath, destPath, StandardCopyOption.REPLACE_EXISTING);
        }
    }

    /**
     * LRU 清理策略
     */
    private synchronized void ensureCapacity() {
        if (currentTotalSize.get() <= appProperties.getCache().getMaxSize().toBytes()) return;

        log.info("Cache limit exceeded. Cleaning up...");

        // 按最后访问时间排序
        cacheIndex.values().stream()
                .filter(e -> e.getStatus() == CacheStatus.COMPLETED) // 只删已完成的
                .sorted(Comparator.comparingLong(CacheEntry::getLastAccessTime))
                .forEach(entry -> {
                    if (currentTotalSize.get() <= appProperties.getCache().getMaxSize().toBytes()) return; // 容量够了就停

                    try {
                        Path path = Paths.get(LocalResourceConfig.CACHE_DIR, entry.getFileName());
                        Files.deleteIfExists(path);
                        currentTotalSize.addAndGet(-entry.getSize());
                        cacheIndex.remove(entry.getId());
                        log.info("Evicted: {}", entry.getFileName());
                    } catch (IOException e) {
                        log.error("Failed to delete {}", entry.getFileName(), e);
                    }
                });
    }

    private boolean reserveSlotAndCapacity(long reservation) {
        if (pendingDownloadTasks.get() >= appProperties.getPerformance().getDownloadMaxQueuedTasks()) return false;
        long maxSize = appProperties.getCache().getMaxSize().toBytes();
        if (currentTotalSize.get() + reservedDownloadBytes.get() + reservation > maxSize) {
            ensureCapacity();
        }
        if (currentTotalSize.get() + reservedDownloadBytes.get() + reservation > maxSize) return false;
        pendingDownloadTasks.incrementAndGet();
        reservedDownloadBytes.addAndGet(reservation);
        return true;
    }

    private void releaseReservation(CacheEntry entry) {
        long reservation = entry.getReservedBytes();
        if (reservation > 0) {
            entry.setReservedBytes(0);
            reservedDownloadBytes.addAndGet(-reservation);
        }
    }

    private boolean isInFlight(CacheStatus status) {
        return status == CacheStatus.PENDING || status == CacheStatus.RESOLVING || status == CacheStatus.DOWNLOADING
                || status == CacheStatus.TRANSCODING || status == CacheStatus.RETRY_WAIT;
    }

    private Semaphore platformPermit(String platform) {
        int permits = switch (platform) {
            case "netease" -> appProperties.getPerformance().getNeteaseDownloadConcurrency();
            case "bilibili" -> appProperties.getPerformance().getBilibiliDownloadConcurrency();
            case "youtube" -> appProperties.getPerformance().getYoutubeDownloadConcurrency();
            case "local", "navidrome" -> appProperties.getPerformance().getLocalDownloadConcurrency();
            default -> 1;
        };
        return new Semaphore(Math.max(1, permits));
    }

    /**
     * 获取文件访问 URL
     * 返回: /media/id.ext
     */
    public String getLocalUrl(String musicId) {
        CacheEntry entry = cacheIndex.get(musicId);
        if (entry != null && entry.getStatus() == CacheStatus.COMPLETED) {
            touch(musicId);
            return "/media/" + entry.getFileName();
        }
        return null;
    }

    public CacheStatus getStatus(String musicId) {
        if (!cacheIndex.containsKey(musicId)) return null;
        return cacheIndex.get(musicId).getStatus();
    }
    
    public CacheEntry getCacheEntry(String musicId) {
        return cacheIndex.get(musicId);
    }

    private void touch(String musicId) {
        CacheEntry entry = cacheIndex.get(musicId);
        if (entry != null) {
            entry.setLastAccessTime(System.currentTimeMillis());
        }
    }

    @Scheduled(fixedDelay = 60000)
    void cleanupStaleTasks() {
        cleanupStaleTasks(System.currentTimeMillis());
        // 定期 LRU 巡检：之前只在下载完成时触发 ensureCapacity，
        // 长期不下载新歌时，缓存可能超限却没人清理。这里每分钟补一次
        ensureCapacity();
    }

    void cleanupStaleTasks(long now) {
        long ttl = appProperties.getPerformance().getDownloadTaskTtlMs();
        cacheIndex.entrySet().removeIf(entry -> {
            CacheStatus status = entry.getValue().getStatus();
            boolean staleStatus = status == CacheStatus.PENDING || status == CacheStatus.RESOLVING
                    || status == CacheStatus.RETRY_WAIT || status == CacheStatus.FAILED || status == CacheStatus.REJECTED;
            if (!staleStatus || now - entry.getValue().getLastAccessTime() <= ttl) {
                return false;
            }
            // 清理孤儿 .part 文件（之前只删索引条目，不删磁盘上的半成品文件）
            String fileName = entry.getValue().getFileName();
            if (fileName != null) {
                try {
                    Path partPath = Paths.get(LocalResourceConfig.CACHE_DIR, fileName + ".part");
                    Files.deleteIfExists(partPath);
                } catch (IOException e) {
                    log.warn("Failed to delete stale .part file for {}", entry.getValue().getId(), e);
                }
            }
            releaseReservation(entry.getValue());
            return true;
        });
    }

    public int getTrackedCacheEntryCount() {
        return cacheIndex.size();
    }

    public int getPendingDownloadTaskCount() {
        return pendingDownloadTasks.get();
    }

    public int getActiveDownloadTaskCount() {
        return activeDownloadTasks.get();
    }

    void trackForTest(String musicId, CacheStatus status, long lastAccessTime) {
        CacheEntry entry = new CacheEntry();
        entry.setId(musicId);
        entry.setStatus(status);
        entry.setLastAccessTime(lastAccessTime);
        cacheIndex.put(musicId, entry);
    }
}
