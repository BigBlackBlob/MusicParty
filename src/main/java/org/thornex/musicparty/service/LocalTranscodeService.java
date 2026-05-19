package org.thornex.musicparty.service;

import jakarta.annotation.PreDestroy;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.enums.LocalTrackStatus;
import org.thornex.musicparty.persistence.LocalTrackRepository;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

@Service
@Slf4j
public class LocalTranscodeService {
    private final AppProperties appProperties;
    private final LocalTrackRepository repository;
    private final ExecutorService executor = Executors.newSingleThreadExecutor();
    private static final Pattern TIME_PATTERN = Pattern.compile("time=(\\d+):(\\d+):(\\d+(?:\\.\\d+)?)");

    public LocalTranscodeService(AppProperties appProperties, LocalTrackRepository repository) {
        this.appProperties = appProperties;
        this.repository = repository;
    }

    public void enqueue(String trackId, Path input) {
        executor.submit(() -> transcode(trackId, input));
    }

    private void transcode(String trackId, Path input) {
        long now = System.currentTimeMillis();
        repository.updateStatus(trackId, LocalTrackStatus.PROCESSING, null, null, "Transcoding audio", 0, now, null, now);
        Path root = Path.of(appProperties.getLocalLibrary().getPath()).toAbsolutePath().normalize();
        Path audioDir = root.resolve("audio");
        Path output = audioDir.resolve(trackId + ".ogg");
        try {
            Files.createDirectories(audioDir);
            long durationMs = repository.findById(trackId).map(org.thornex.musicparty.dto.LocalTrack::durationMs).orElse(0L);
            List<String> command = List.of(
                    appProperties.getFfmpegPath(),
                    "-y",
                    "-i", input.toAbsolutePath().toString(),
                    "-vn",
                    "-c:a", "libvorbis",
                    "-b:a", "320k",
                    "-progress", "pipe:1",
                    output.toAbsolutePath().toString()
            );
            Process process = new ProcessBuilder(command)
                    .redirectErrorStream(true)
                    .start();
            try (java.io.BufferedReader reader = process.inputReader()) {
                String line;
                while ((line = reader.readLine()) != null) {
                    Integer progress = parseProgress(line, durationMs);
                    if (progress != null) {
                        repository.updateStatus(trackId, LocalTrackStatus.PROCESSING, null, null,
                                "Transcoding audio", Math.min(progress, 99), null, null, System.currentTimeMillis());
                    }
                }
            }
            int exitCode = process.waitFor();
            if (exitCode != 0 || !Files.exists(output) || Files.size(output) == 0) {
                throw new IllegalStateException("ffmpeg exited with code " + exitCode);
            }
            Files.deleteIfExists(input);
            long completedAt = System.currentTimeMillis();
            repository.updateStatus(trackId, LocalTrackStatus.COMPLETED, "audio/" + trackId + ".ogg", null,
                    "Transcode completed", 100, null, completedAt, completedAt);
        } catch (Exception e) {
            log.warn("Local track transcode failed: {}", trackId, e);
            repository.updateStatus(trackId, LocalTrackStatus.FAILED, null, e.getMessage(),
                    "Transcode failed", null, null, null, System.currentTimeMillis());
        }
    }

    private Integer parseProgress(String line, long durationMs) {
        if (durationMs <= 0 || line == null) return null;
        if (line.startsWith("out_time_ms=")) {
            try {
                long outMicros = Long.parseLong(line.substring("out_time_ms=".length()).trim());
                return (int) Math.max(0, Math.min(99, (outMicros / 1000L) * 100 / durationMs));
            } catch (NumberFormatException ignored) {
                return null;
            }
        }
        Matcher matcher = TIME_PATTERN.matcher(line);
        if (!matcher.find()) return null;
        double seconds = Integer.parseInt(matcher.group(1)) * 3600
                + Integer.parseInt(matcher.group(2)) * 60
                + Double.parseDouble(matcher.group(3));
        return (int) Math.max(0, Math.min(99, seconds * 1000 * 100 / durationMs));
    }

    @PreDestroy
    public void shutdown() {
        executor.shutdownNow();
    }
}
