package org.thornex.musicparty.service;

import lombok.extern.slf4j.Slf4j;
import org.jaudiotagger.audio.AudioFile;
import org.jaudiotagger.audio.AudioFileIO;
import org.jaudiotagger.audio.AudioHeader;
import org.jaudiotagger.tag.FieldKey;
import org.jaudiotagger.tag.Tag;
import org.jaudiotagger.tag.images.Artwork;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.multipart.MultipartFile;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.dto.LocalTrackUpdateRequest;
import org.thornex.musicparty.dto.LocalUploadResult;
import org.thornex.musicparty.enums.LocalTrackStatus;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.persistence.LocalTrackRepository;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.DigestInputStream;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;
import java.util.List;
import java.util.UUID;

@Service
@Slf4j
public class LocalLibraryService {
    private final AppProperties appProperties;
    private final LocalTrackRepository repository;
    private final LocalTranscodeService transcodeService;

    public LocalLibraryService(AppProperties appProperties,
                               LocalTrackRepository repository,
                               LocalTranscodeService transcodeService) {
        this.appProperties = appProperties;
        this.repository = repository;
        this.transcodeService = transcodeService;
    }

    public List<LocalTrack> listTracks() {
        return repository.findAll();
    }

    public List<LocalTrack> search(String keyword, int offset, int limit) {
        return repository.searchCompleted(keyword, offset, limit);
    }

    public LocalTrack getTrack(String id) {
        return repository.findById(id).orElseThrow(() -> new ApiRequestException("Local track not found"));
    }

    public LocalTrack getPlayableTrack(String id) {
        LocalTrack track = getTrack(id);
        if (track.status() == LocalTrackStatus.DELETED) {
            throw new ApiRequestException("Local track is unavailable");
        }
        if (track.status() != LocalTrackStatus.COMPLETED || !StringUtils.hasText(track.oggPath())) {
            throw new ApiRequestException("Local track is not playable");
        }
        return track;
    }

    public LocalUploadResult upload(MultipartFile file, String uploadedBy, String title, String artists, String album) throws IOException {
        if (file == null || file.isEmpty()) {
            throw new IllegalArgumentException("Audio file is required");
        }
        if (file.getSize() > appProperties.getLocalLibrary().getMaxUploadBytes()) {
            throw new IllegalArgumentException("Audio file is too large");
        }

        String id = UUID.randomUUID().toString();
        Path incomingDir = root().resolve("incoming");
        Files.createDirectories(incomingDir);
        String originalName = file.getOriginalFilename() == null ? "upload" : Path.of(file.getOriginalFilename()).getFileName().toString();
        Path input = incomingDir.resolve(id + "-" + originalName.replaceAll("[^A-Za-z0-9._-]", "_"));
        file.transferTo(input);
        String originalHash = sha256(input);
        var existing = repository.findActiveByOriginalHash(originalHash);
        if (existing.isPresent()) {
            Files.deleteIfExists(input);
            return LocalUploadResult.duplicate(existing.get());
        }

        LocalTrackMetadata tag = readAudioTags(input);
        String coverPath = writeCoverIfPresent(id, tag);
        long now = System.currentTimeMillis();
        LocalTrack track = new LocalTrack(
                id,
                originalHash,
                originalName,
                root().relativize(input.toAbsolutePath().normalize()).toString().replace('\\', '/'),
                file.getContentType(),
                file.getSize(),
                firstText(title, tag.title(), stripExtension(originalName)),
                splitArtists(firstText(artists, tag.artist(), "Unknown")),
                firstText(album, tag.album(), ""),
                tag.durationMs(),
                coverPath,
                tag.coverMimeType(),
                null,
                LocalTrackStatus.QUEUED,
                null,
                "Queued for transcoding",
                0,
                uploadedBy,
                now,
                now,
                null,
                null
        );
        repository.upsert(track);
        transcodeService.enqueue(id, input);
        return LocalUploadResult.created(track);
    }

    public LocalTrack update(String id, LocalTrackUpdateRequest request) {
        LocalTrack old = repository.findById(id).orElseThrow(() -> new ApiRequestException("Local track not found"));
        LocalTrack updated = new LocalTrack(
                old.id(),
                old.originalHash(),
                old.originalFileName(),
                old.sourcePath(),
                old.sourceMimeType(),
                old.sourceSizeBytes(),
                StringUtils.hasText(request.title()) ? request.title().trim() : old.title(),
                request.artists() == null || request.artists().isEmpty() ? old.artists() : request.artists().stream().filter(StringUtils::hasText).map(String::trim).toList(),
                request.album() == null ? old.album() : request.album().trim(),
                old.durationMs(),
                old.coverPath(),
                old.coverMimeType(),
                old.oggPath(),
                old.status(),
                old.errorMessage(),
                old.statusMessage(),
                old.progressPercent(),
                old.uploadedBy(),
                old.createdAt(),
                System.currentTimeMillis(),
                old.startedAt(),
                old.completedAt()
        );
        repository.upsert(updated);
        return updated;
    }

    public void delete(String id) {
        LocalTrack track = repository.findById(id).orElse(null);
        if (track == null) return;
        deleteQuietly(track.oggPath());
        deleteQuietly(track.coverPath());
        deleteQuietly(track.sourcePath());
        repository.deleteLocalReferences(id);
        repository.markDeleted(id, System.currentTimeMillis());
    }

    public Path root() {
        return Path.of(appProperties.getLocalLibrary().getPath()).toAbsolutePath().normalize();
    }

    private void deleteQuietly(String value) {
        if (!StringUtils.hasText(value)) return;
        try {
            Files.deleteIfExists(root().resolve(value).normalize());
        } catch (IOException e) {
            log.warn("Failed to delete local library file {}", value, e);
        }
    }

    private LocalTrackMetadata readAudioTags(Path input) {
        try {
            AudioFile audioFile = AudioFileIO.read(input.toFile());
            AudioHeader header = audioFile.getAudioHeader();
            Tag tag = audioFile.getTag();
            Artwork artwork = tag == null ? null : tag.getFirstArtwork();
            byte[] cover = artwork == null ? null : artwork.getBinaryData();
            return new LocalTrackMetadata(
                    text(tag, FieldKey.TITLE),
                    text(tag, FieldKey.ARTIST),
                    text(tag, FieldKey.ALBUM),
                    header == null ? 0 : Math.max(0, header.getTrackLength()) * 1000L,
                    cover,
                    artwork == null ? null : artwork.getMimeType()
            );
        } catch (Exception e) {
            log.debug("Failed to read local track tags from {}", input, e);
            return LocalTrackMetadata.empty();
        }
    }

    private String writeCoverIfPresent(String id, LocalTrackMetadata metadata) {
        if (metadata.coverBytes() == null || metadata.coverBytes().length == 0) return null;
        String extension = metadata.coverMimeType() != null && metadata.coverMimeType().toLowerCase().contains("png") ? ".png" : ".jpg";
        Path cover = root().resolve("covers").resolve(id + extension);
        try {
            Files.createDirectories(cover.getParent());
            Files.write(cover, metadata.coverBytes());
            return root().relativize(cover).toString().replace('\\', '/');
        } catch (IOException e) {
            log.warn("Failed to write embedded cover for local track {}", id, e);
            return null;
        }
    }

    private String text(Tag tag, FieldKey key) {
        if (tag == null) return "";
        try {
            return tag.getFirst(key);
        } catch (Exception e) {
            return "";
        }
    }

    private String sha256(Path input) throws IOException {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            try (DigestInputStream stream = new DigestInputStream(Files.newInputStream(input), digest)) {
                stream.transferTo(java.io.OutputStream.nullOutputStream());
            }
            return HexFormat.of().formatHex(digest.digest());
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is not available", e);
        }
    }

    private String firstText(String... values) {
        for (String value : values) {
            if (StringUtils.hasText(value)) return value.trim();
        }
        return "";
    }

    private List<String> splitArtists(String value) {
        if (!StringUtils.hasText(value)) return List.of("Unknown");
        List<String> result = java.util.Arrays.stream(value.split("[,;/]"))
                .map(String::trim)
                .filter(StringUtils::hasText)
                .toList();
        return result.isEmpty() ? List.of("Unknown") : result;
    }

    private String stripExtension(String name) {
        int dot = name.lastIndexOf('.');
        return dot > 0 ? name.substring(0, dot) : name;
    }

    private record LocalTrackMetadata(String title, String artist, String album, long durationMs, byte[] coverBytes, String coverMimeType) {
        static LocalTrackMetadata empty() {
            return new LocalTrackMetadata("", "", "", 0, null, null);
        }
    }
}
