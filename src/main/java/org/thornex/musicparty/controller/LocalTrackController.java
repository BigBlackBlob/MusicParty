package org.thornex.musicparty.controller;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;
import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.dto.LocalTrackUpdateRequest;
import org.thornex.musicparty.dto.LocalUploadAccessRequest;
import org.thornex.musicparty.enums.LocalTrackStatus;
import org.thornex.musicparty.service.LocalLibraryAccessService;
import org.thornex.musicparty.service.LocalLibraryService;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;

import java.io.IOException;
import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;

@RestController
@RequestMapping("/api/local")
@Slf4j
public class LocalTrackController {
    private final LocalLibraryService localLibraryService;
    private final LocalLibraryAccessService accessService;
    private final InternalStreamProxyToken internalStreamProxyToken;

    public LocalTrackController(LocalLibraryService localLibraryService,
                                LocalLibraryAccessService accessService,
                                InternalStreamProxyToken internalStreamProxyToken) {
        this.localLibraryService = localLibraryService;
        this.accessService = accessService;
        this.internalStreamProxyToken = internalStreamProxyToken;
    }

    @GetMapping("/tracks")
    public ResponseEntity<?> listTracks(@RequestParam String adminPassword) {
        if (!accessService.isAdminPassword(adminPassword)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        return ResponseEntity.ok(localLibraryService.listTracks());
    }

    @PostMapping(value = "/tracks/upload", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    public ResponseEntity<?> uploadTrack(@RequestPart("file") MultipartFile file,
                                         @RequestParam(required = false) String token,
                                         @RequestParam(required = false) String adminPassword,
                                         @RequestParam(required = false) String title,
                                         @RequestParam(required = false) String artists,
                                         @RequestParam(required = false) String album) {
        if (!accessService.canManage(token, adminPassword)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        try {
            String uploadedBy = accessService.isAdminPassword(adminPassword)
                    ? "admin"
                    : accessService.displayNameForToken(token).orElse("user");
            return ResponseEntity.ok(localLibraryService.upload(file, uploadedBy, title, artists, album));
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(Map.of("message", e.getMessage()));
        } catch (IOException e) {
            log.warn("Local upload failed", e);
            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(Map.of("message", "Upload failed"));
        }
    }

    @PatchMapping("/tracks/{id}")
    public ResponseEntity<?> updateTrack(@PathVariable String id, @RequestBody LocalTrackUpdateRequest request) {
        if (!accessService.canManage(request.token(), request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        return ResponseEntity.ok(localLibraryService.update(id, request));
    }

    @DeleteMapping("/tracks/{id}")
    public ResponseEntity<?> deleteTrack(@PathVariable String id,
                                         @RequestParam(required = false) String token,
                                         @RequestParam(required = false) String adminPassword) {
        if (!accessService.canManage(token, adminPassword)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        localLibraryService.delete(id);
        return ResponseEntity.ok(Map.of("message", "LOCAL TRACK DELETED"));
    }

    @GetMapping("/upload-access")
    public ResponseEntity<?> listUploadAccess(@RequestParam String adminPassword) {
        if (!accessService.isAdminPassword(adminPassword)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        return ResponseEntity.ok(accessService.listAllowedUsers());
    }

    @PostMapping("/upload-access/grant")
    public ResponseEntity<?> grantUploadAccess(@RequestBody LocalUploadAccessRequest request) {
        if (!accessService.isAdminPassword(request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        accessService.grant(request.userName());
        return ResponseEntity.ok(Map.of("message", "LOCAL UPLOAD ACCESS GRANTED"));
    }

    @PostMapping("/upload-access/revoke")
    public ResponseEntity<?> revokeUploadAccess(@RequestBody LocalUploadAccessRequest request) {
        if (!accessService.isAdminPassword(request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        accessService.revoke(request.userName());
        return ResponseEntity.ok(Map.of("message", "LOCAL UPLOAD ACCESS REVOKED"));
    }

    @GetMapping("/media/{id}")
    public ResponseEntity<StreamingResponseBody> media(@PathVariable String id,
                                                       @RequestParam(required = false) String token,
                                                       @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken,
                                                       @RequestHeader(value = HttpHeaders.RANGE, required = false) String rangeHeader) throws IOException {
        if (!canReadMedia(token, internalToken)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        }
        LocalTrack track = localLibraryService.getTrack(id);
        if (track.status() == LocalTrackStatus.DELETED) {
            return ResponseEntity.status(HttpStatus.GONE).build();
        }
        if (track.status() != LocalTrackStatus.COMPLETED || track.oggPath() == null || track.oggPath().isBlank()) {
            return ResponseEntity.notFound().build();
        }
        Path path = localLibraryService.root().resolve(track.oggPath()).normalize();
        if (!Files.exists(path)) {
            return ResponseEntity.notFound().build();
        }
        return streamFile(path, "audio/ogg", rangeHeader);
    }

    @GetMapping("/cover/{id}")
    public ResponseEntity<StreamingResponseBody> cover(@PathVariable String id) throws IOException {
        LocalTrack track = localLibraryService.getTrack(id);
        if (track.status() == LocalTrackStatus.DELETED) {
            return ResponseEntity.status(HttpStatus.GONE).build();
        }
        if (track.status() != LocalTrackStatus.COMPLETED) {
            return ResponseEntity.notFound().build();
        }
        if (track.coverPath() == null || track.coverPath().isBlank()) {
            return ResponseEntity.notFound().build();
        }
        Path path = localLibraryService.root().resolve(track.coverPath()).normalize();
        if (!Files.exists(path)) {
            return ResponseEntity.notFound().build();
        }
        String contentType = Files.probeContentType(path);
        return streamFile(path, contentType == null ? MediaType.APPLICATION_OCTET_STREAM_VALUE : contentType, null);
    }

    private boolean canReadMedia(String token, String internalToken) {
        return internalStreamProxyToken.matches(internalToken) || accessService.isEnabled();
    }

    private ResponseEntity<StreamingResponseBody> streamFile(Path path, String contentType, String rangeHeader) throws IOException {
        long length = Files.size(path);
        long start = 0;
        long end = length - 1;
        HttpStatus status = HttpStatus.OK;
        if (rangeHeader != null && rangeHeader.startsWith("bytes=")) {
            String[] parts = rangeHeader.substring("bytes=".length()).split("-", 2);
            try {
                start = parts[0].isBlank() ? 0 : Long.parseLong(parts[0]);
                if (parts.length > 1 && !parts[1].isBlank()) {
                    end = Math.min(Long.parseLong(parts[1]), length - 1);
                }
                status = HttpStatus.PARTIAL_CONTENT;
            } catch (NumberFormatException ignored) {
                start = 0;
                end = length - 1;
                status = HttpStatus.OK;
            }
        }
        if (start < 0 || start >= length || end < start) {
            return ResponseEntity.status(HttpStatus.REQUESTED_RANGE_NOT_SATISFIABLE).build();
        }
        long contentLength = end - start + 1;
        HttpHeaders headers = new HttpHeaders();
        headers.set(HttpHeaders.CONTENT_TYPE, contentType);
        headers.set(HttpHeaders.ACCEPT_RANGES, "bytes");
        headers.setContentLength(contentLength);
        headers.set(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "*");
        headers.set(HttpHeaders.ACCESS_CONTROL_EXPOSE_HEADERS, "Content-Length, Content-Range, Accept-Ranges");
        if (status == HttpStatus.PARTIAL_CONTENT) {
            headers.set(HttpHeaders.CONTENT_RANGE, "bytes " + start + "-" + end + "/" + length);
        }
        long finalStart = start;
        StreamingResponseBody body = outputStream -> {
            try (InputStream inputStream = Files.newInputStream(path)) {
                long skipped = inputStream.skip(finalStart);
                while (skipped < finalStart) {
                    long next = inputStream.skip(finalStart - skipped);
                    if (next <= 0) break;
                    skipped += next;
                }
                byte[] buffer = new byte[8192];
                long remaining = contentLength;
                while (remaining > 0) {
                    int read = inputStream.read(buffer, 0, (int) Math.min(buffer.length, remaining));
                    if (read == -1) break;
                    outputStream.write(buffer, 0, read);
                    remaining -= read;
                }
                outputStream.flush();
            }
        };
        return new ResponseEntity<>(body, headers, status);
    }
}
