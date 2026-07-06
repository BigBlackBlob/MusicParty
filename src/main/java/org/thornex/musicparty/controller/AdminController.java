package org.thornex.musicparty.controller;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.util.StringUtils;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.thornex.musicparty.dto.AdminCommandRequest;
import org.thornex.musicparty.dto.AdminNavidromeAccessRequest;
import org.thornex.musicparty.dto.AdminSubsonicSourceRequest;
import org.thornex.musicparty.dto.AdminSubsonicSourceView;
import org.thornex.musicparty.dto.SubsonicSourceRequest;
import org.thornex.musicparty.service.ChatService;
import org.thornex.musicparty.service.AdminAuthorizationService;
import org.thornex.musicparty.service.MusicPlayerService;
import org.thornex.musicparty.service.NavidromeAccessService;
import org.thornex.musicparty.service.RuntimeMetricsService;
import org.thornex.musicparty.service.RoomSubsonicSource;
import org.thornex.musicparty.service.SubsonicSourceRegistry;
import org.thornex.musicparty.service.api.BilibiliMusicApiService;
import org.thornex.musicparty.service.api.NeteaseMusicApiService;
import reactor.core.publisher.Mono;

import java.util.Map;
import java.util.Set;

@RestController
@RequestMapping("/api/admin")
public class AdminController {

    private final MusicPlayerService musicPlayerService;
    private final ChatService chatService;
    private final NeteaseMusicApiService neteaseMusicApiService;
    private final BilibiliMusicApiService bilibiliMusicApiService;
    private final org.thornex.musicparty.service.stream.LiveStreamService liveStreamService;
    private final SubsonicSourceRegistry subsonicSourceRegistry;
    private final NavidromeAccessService navidromeAccessService;
    private final AdminAuthorizationService adminAuthorizationService;
    private final RuntimeMetricsService runtimeMetricsService;

    public AdminController(MusicPlayerService musicPlayerService,
                           ChatService chatService,
                           NeteaseMusicApiService neteaseMusicApiService,
                           BilibiliMusicApiService bilibiliMusicApiService,
                           org.thornex.musicparty.service.stream.LiveStreamService liveStreamService,
                           SubsonicSourceRegistry subsonicSourceRegistry,
                           NavidromeAccessService navidromeAccessService,
                           AdminAuthorizationService adminAuthorizationService,
                           RuntimeMetricsService runtimeMetricsService) {
        this.musicPlayerService = musicPlayerService;
        this.chatService = chatService;
        this.neteaseMusicApiService = neteaseMusicApiService;
        this.bilibiliMusicApiService = bilibiliMusicApiService;
        this.liveStreamService = liveStreamService;
        this.subsonicSourceRegistry = subsonicSourceRegistry;
        this.navidromeAccessService = navidromeAccessService;
        this.adminAuthorizationService = adminAuthorizationService;
        this.runtimeMetricsService = runtimeMetricsService;
    }

    @PostMapping("/command")
    public Mono<ResponseEntity<?>> handleAdminCommand(@RequestBody AdminCommandRequest request) {
        if (!isValidAdmin(request.sessionToken(), request.password())) {
            return just(ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED")));
        }

        String command = request.command().trim();
        if (!command.startsWith("//")) {
            return just(ResponseEntity.badRequest().body(Map.of("message", "Invalid command format.")));
        }

        String[] parts = command.split("\\s+", 3);
        String action = parts[0].toUpperCase();

        switch (action) {
            case "//STREAM":
                if (parts.length < 2) {
                    return just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //STREAM <ON/OFF>")));
                }
                String subCmd = parts[1].toUpperCase();
                if ("ON".equals(subCmd)) {
                    liveStreamService.setEnabled(true);
                    return just(ResponseEntity.ok(Map.of("message", "STREAM SERVICE ENABLED")));
                } else if ("OFF".equals(subCmd)) {
                    liveStreamService.setEnabled(false);
                    return just(ResponseEntity.ok(Map.of("message", "STREAM SERVICE DISABLED")));
                } else {
                    return just(ResponseEntity.badRequest().body(Map.of("message", "Invalid stream command")));
                }

            case "//LOCK":
                if (parts.length < 3) {
                    if (parts.length < 2) {
                        return just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //LOCK <TYPE> <ON/OFF>. TYPE: PAUSE, SKIP, SHUFFLE, ALL")));
                    }
                    return just(ResponseEntity.badRequest().body(Map.of("message", "Missing ON/OFF. Usage: //LOCK <TYPE> <ON/OFF>")));
                }
                String type = parts[1].toUpperCase();
                String state = parts[2].toUpperCase();
                boolean locked = "ON".equals(state);

                if ("ALL".equals(type)) {
                    musicPlayerService.setAllLocks(locked);
                    return just(ResponseEntity.ok(Map.of("message", "ALL LOCKS SET TO " + locked)));
                } else if (Set.of("PAUSE", "SKIP", "SHUFFLE").contains(type)) {
                    musicPlayerService.setLock(type, locked);
                    return just(ResponseEntity.ok(Map.of("message", type + " LOCK SET TO " + locked)));
                } else {
                    return just(ResponseEntity.badRequest().body(Map.of("message", "Invalid lock type: " + type)));
                }

            case "//PAUSE":
                musicPlayerService.togglePause("SYSTEM");
                return just(ResponseEntity.ok(Map.of("message", "TOGGLE PAUSE (SYSTEM OVERRIDE)")));

            case "//SKIP":
                musicPlayerService.skipToNext("SYSTEM");
                return just(ResponseEntity.ok(Map.of("message", "SKIP TO NEXT (SYSTEM OVERRIDE)")));

            case "//SHUFFLE":
                musicPlayerService.toggleShuffle("SYSTEM");
                return just(ResponseEntity.ok(Map.of("message", "TOGGLE SHUFFLE (SYSTEM OVERRIDE)")));

            case "//RESET":
                musicPlayerService.resetSystem();
                return just(ResponseEntity.ok(Map.of("message", "SYSTEM PURGED")));

            case "//CLEAR":
                if (parts.length < 2 || "QUEUE".equalsIgnoreCase(parts[1])) {
                    musicPlayerService.clearQueue();
                    return just(ResponseEntity.ok(Map.of("message", "QUEUE CLEARED")));
                } else if ("CHAT".equalsIgnoreCase(parts[1])) {
                    chatService.clearHistoryAndNotify();
                    return just(ResponseEntity.ok(Map.of("message", "CHAT HISTORY CLEARED")));
                } else {
                    return just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //CLEAR <QUEUE/CHAT>")));
                }

            case "//PASS":
                return just(ResponseEntity.status(HttpStatus.GONE).body(Map.of("message", "Legacy global password commands were removed. Use account login and room privacy settings.")));

            case "//OPEN":
                return just(ResponseEntity.status(HttpStatus.GONE).body(Map.of("message", "Legacy global password commands were removed. Use account login and room privacy settings.")));

            case "//COOKIE":
                if (parts.length < 3) {
                    return just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //COOKIE <platform> <cookie_string>")));
                }
                String platform = parts[1].toLowerCase();
                String cookie = parts[2];

                if ("netease".equals(platform)) {
                    neteaseMusicApiService.updateCookie(cookie);
                    return just(ResponseEntity.ok(Map.of("message", "Netease cookie updated.")));
                } else if ("bilibili".equals(platform)) {
                    bilibiliMusicApiService.updateSessdata(cookie);
                    return just(ResponseEntity.ok(Map.of("message", "Bilibili SESSDATA updated.")));
                } else {
                    return just(ResponseEntity.badRequest().body(Map.of("message", "Unsupported platform: " + platform)));
                }

            case "//SUBSONIC":
                return handleSubsonicCommand(SubsonicSourceRegistry.normalizeRoomId(request.roomId()), parts);

            case "//NAVIDROME":
                return just(handleNavidromeCommand(parts));

            default:
                return just(ResponseEntity.badRequest().body(Map.of("message", "Unknown command: " + action)));
        }
    }

    @GetMapping("/runtime-metrics")
    public ResponseEntity<?> runtimeMetrics(String adminPassword, String sessionToken) {
        if (!isValidAdmin(sessionToken, adminPassword)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        return ResponseEntity.ok(runtimeMetricsService.snapshot());
    }

    @PostMapping("/subsonic-source")
    public ResponseEntity<?> upsertSubsonicSource(@RequestBody AdminSubsonicSourceRequest request) {
        if (!isValidAdmin(request.sessionToken(), request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        try {
            var source = subsonicSourceRegistry.upsert(
                    SubsonicSourceRegistry.normalizeRoomId(request.roomId()),
                    new SubsonicSourceRequest(
                            request.id(),
                            request.label(),
                            request.baseUrl(),
                            request.username(),
                            request.password(),
                            request.allowedUsers(),
                            request.enabled()
                    )
            );
            return ResponseEntity.ok(Map.of("message", "SOURCE SAVED", "platformId", source.platformId()));
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(Map.of("message", e.getMessage()));
        }
    }

    @PostMapping("/navidrome-access/grant")
    public ResponseEntity<?> grantNavidromeAccess(@RequestBody AdminNavidromeAccessRequest request) {
        if (!isValidAdmin(request.sessionToken(), request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        return updateNavidromeAccess(request.userName(), true);
    }

    @PostMapping("/navidrome-access/revoke")
    public ResponseEntity<?> revokeNavidromeAccess(@RequestBody AdminNavidromeAccessRequest request) {
        if (!isValidAdmin(request.sessionToken(), request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        return updateNavidromeAccess(request.userName(), false);
    }

    @GetMapping("/subsonic-sources")
    public ResponseEntity<?> listSubsonicSources(String adminPassword, String sessionToken, String roomId) {
        if (!isValidAdmin(sessionToken, adminPassword)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        return ResponseEntity.ok(subsonicSourceRegistry.list(SubsonicSourceRegistry.normalizeRoomId(roomId))
                .stream()
                .map(this::toSourceView)
                .toList());
    }

    @PostMapping("/subsonic-source/test")
    public Mono<ResponseEntity<Map<String, String>>> testSubsonicSource(@RequestBody AdminSubsonicSourceRequest request) {
        if (!isValidAdmin(request.sessionToken(), request.adminPassword())) {
            return Mono.just(ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED")));
        }
        RoomSubsonicSource source = subsonicSourceRegistry
                .findRoomSource(SubsonicSourceRegistry.normalizeRoomId(request.roomId()), request.id())
                .orElse(null);
        if (source == null) {
            return Mono.just(ResponseEntity.badRequest().body(Map.of("message", "Source not found")));
        }
        return subsonicSourceRegistry.test(source)
                .timeout(java.time.Duration.ofSeconds(12))
                .thenReturn(ResponseEntity.ok(Map.of("message", "SUBSONIC SOURCE OK: " + source.label())))
                .onErrorResume(e -> Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY)
                        .body(Map.of("message", "Subsonic test failed: " + e.getMessage()))));
    }

    @PostMapping("/subsonic-source/order")
    public ResponseEntity<?> reorderSubsonicSource(@RequestBody AdminSubsonicSourceRequest request) {
        if (!isValidAdmin(request.sessionToken(), request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        if (request.sortOrder() == null) {
            return ResponseEntity.badRequest().body(Map.of("message", "sortOrder is required"));
        }
        try {
            RoomSubsonicSource source = subsonicSourceRegistry.updateSortOrder(
                    SubsonicSourceRegistry.normalizeRoomId(request.roomId()),
                    request.id(),
                    request.sortOrder()
            );
            return ResponseEntity.ok(Map.of("message", "SOURCE ORDER UPDATED", "source", toSourceView(source)));
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(Map.of("message", e.getMessage()));
        }
    }

    @PostMapping("/subsonic-source/remove")
    public ResponseEntity<?> removeSubsonicSource(@RequestBody AdminSubsonicSourceRequest request) {
        if (!isValidAdmin(request.sessionToken(), request.adminPassword())) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "ACCESS DENIED"));
        }
        boolean removed = subsonicSourceRegistry.remove(SubsonicSourceRegistry.normalizeRoomId(request.roomId()), request.id());
        return removed
                ? ResponseEntity.ok(Map.of("message", "SOURCE REMOVED"))
                : ResponseEntity.badRequest().body(Map.of("message", "Source not found in this Lounge"));
    }

    private boolean isValidAdmin(String sessionToken, String legacyPassword) {
        return adminAuthorizationService.isAuthorized(sessionToken, legacyPassword);
    }

    private AdminSubsonicSourceView toSourceView(RoomSubsonicSource source) {
        return new AdminSubsonicSourceView(
                source.id(),
                source.platformId(),
                source.label(),
                source.source().baseUrl(),
                source.source().username(),
                StringUtils.hasText(source.allowedUsers()) ? source.allowedUsers() : source.source().allowedUsers(),
                source.enabled(),
                source.active(),
                source.system(),
                source.sortOrder(),
                source.updatedAt()
        );
    }

    private ResponseEntity<?> handleNavidromeCommand(String[] parts) {
        if (parts.length < 3) {
            return ResponseEntity.badRequest().body(Map.of("message", "Usage: //NAVIDROME <GRANT/REVOKE> <user_name>"));
        }
        String subCommand = parts[1].toUpperCase();
        String userName = parts[2].trim();
        if (userName.isBlank()) {
            return ResponseEntity.badRequest().body(Map.of("message", "User name cannot be empty"));
        }
        return switch (subCommand) {
            case "GRANT" -> updateNavidromeAccess(userName, true);
            case "REVOKE" -> updateNavidromeAccess(userName, false);
            default -> ResponseEntity.badRequest().body(Map.of("message", "Usage: //NAVIDROME <GRANT/REVOKE> <user_name>"));
        };
    }

    private ResponseEntity<?> updateNavidromeAccess(String userName, boolean grant) {
        if (!StringUtils.hasText(userName)) {
            return ResponseEntity.badRequest().body(Map.of("message", "User name cannot be empty"));
        }
        boolean updated = grant
                ? navidromeAccessService.grantUserName(userName.trim())
                : navidromeAccessService.revokeUserName(userName.trim());
        if (updated) {
            return ResponseEntity.ok(Map.of("message", grant ? "NAVIDROME ACCESS GRANTED" : "NAVIDROME ACCESS REVOKED"));
        }
        return ResponseEntity.badRequest().body(Map.of("message", grant ? "Unable to grant Navidrome access" : "Unable to revoke Navidrome access"));
    }

    private Mono<ResponseEntity<?>> handleSubsonicCommand(String roomId, String[] parts) {
        if (parts.length < 2) {
            return just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //SUBSONIC <LIST/TEST/REMOVE/ADD>")));
        }
        String subCommand = parts[1].toUpperCase();
        return switch (subCommand) {
            case "LIST" -> just(ResponseEntity.ok(Map.of(
                    "message",
                    subsonicSourceRegistry.list(roomId).stream()
                            .map(source -> source.id() + "=" + (source.enabled() ? "ON" : "OFF") + " " + source.label())
                            .toList()
                            .toString()
            )));
            case "REMOVE" -> {
                if (parts.length < 3) {
                    yield just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //SUBSONIC REMOVE <id>")));
                }
                boolean removed = subsonicSourceRegistry.remove(roomId, parts[2].trim());
                yield just(removed
                        ? ResponseEntity.ok(Map.of("message", "SUBSONIC SOURCE REMOVED"))
                        : ResponseEntity.badRequest().body(Map.of("message", "Source not found in this Lounge")));
            }
            case "TEST" -> {
                if (parts.length < 3) {
                    yield just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //SUBSONIC TEST <id>")));
                }
                RoomSubsonicSource source = subsonicSourceRegistry.findRoomSource(roomId, parts[2].trim()).orElse(null);
                if (source == null) {
                    yield just(ResponseEntity.badRequest().body(Map.of("message", "Source not found")));
                }
                yield subsonicSourceRegistry.test(source)
                        .timeout(java.time.Duration.ofSeconds(12))
                        .then(Mono.<ResponseEntity<?>>just(ResponseEntity.ok(Map.of("message", "SUBSONIC SOURCE OK: " + source.label()))))
                        .onErrorResume(e -> Mono.<ResponseEntity<?>>just(ResponseEntity.status(HttpStatus.BAD_GATEWAY)
                                .body(Map.of("message", "Subsonic test failed: " + e.getMessage()))));
            }
            case "ADD" -> {
                if (parts.length < 3) {
                    yield just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //SUBSONIC ADD id=<id> label=<name> baseUrl=<url> username=<user> password=<pass> [allowedUsers=*]")));
                }
                try {
                    SubsonicSourceRequest request = parseSubsonicRequest(parts[2]);
                    var source = subsonicSourceRegistry.upsert(roomId, request);
                    yield just(ResponseEntity.ok(Map.of("message", "SUBSONIC SOURCE SAVED: " + source.platformId())));
                } catch (IllegalArgumentException e) {
                    yield just(ResponseEntity.badRequest().body(Map.of("message", e.getMessage())));
                }
            }
            default -> just(ResponseEntity.badRequest().body(Map.of("message", "Usage: //SUBSONIC <LIST/TEST/REMOVE/ADD>")));
        };
    }

    private Mono<ResponseEntity<?>> just(ResponseEntity<?> response) {
        return Mono.just(response);
    }

    private SubsonicSourceRequest parseSubsonicRequest(String raw) {
        Map<String, String> params = java.util.Arrays.stream(raw.split("\\s+"))
                .map(part -> part.split("=", 2))
                .filter(pair -> pair.length == 2)
                .collect(java.util.stream.Collectors.toMap(pair -> pair[0].toLowerCase(), pair -> pair[1], (left, right) -> right));
        return new SubsonicSourceRequest(
                params.get("id"),
                params.get("label"),
                params.get("baseurl"),
                params.get("username"),
                params.get("password"),
                params.getOrDefault("allowedusers", "*"),
                !"false".equalsIgnoreCase(params.getOrDefault("enabled", "true"))
        );
    }
}
