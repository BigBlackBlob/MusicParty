package org.thornex.musicparty.controller;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.dto.RoomUpdateRequest;
import org.thornex.musicparty.service.RoomLifecycleService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.RoomAuthorizationService;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.UserService;
import org.thornex.musicparty.security.SessionCookieService;

import java.util.Map;
import java.util.List;

@RestController
@RequestMapping("/api/rooms")
public class RoomController {
    private final RoomService roomService;
    private final AccountService accountService;
    private final RoomLifecycleService roomLifecycleService;
    private final RoomAuthorizationService roomAuthorizationService;
    private final SessionCookieService sessionCookieService;
    private final UserService legacyUserService;

    @Autowired
    public RoomController(RoomService roomService,
                          AccountService accountService,
                          RoomLifecycleService roomLifecycleService,
                          RoomAuthorizationService roomAuthorizationService,
                          SessionCookieService sessionCookieService) {
        this.roomService = roomService;
        this.accountService = accountService;
        this.roomLifecycleService = roomLifecycleService;
        this.roomAuthorizationService = roomAuthorizationService;
        this.sessionCookieService = sessionCookieService;
        this.legacyUserService = null;
    }

    /** Compatibility constructor retained for existing direct controller tests. */
    public RoomController(RoomService roomService,
                          RoomAccessService ignoredRoomAccessService,
                          UserService userService,
                          AccountService accountService,
                          RoomLifecycleService roomLifecycleService) {
        this.roomService = roomService;
        this.accountService = accountService;
        this.roomLifecycleService = roomLifecycleService;
        this.roomAuthorizationService = null;
        this.sessionCookieService = null;
        this.legacyUserService = userService;
    }

    @GetMapping
    public List<RoomInfo> listRooms(org.springframework.web.server.ServerWebExchange exchange) {
        String token = sessionCookieService.sessionToken(exchange);
        String requesterPublicId = accountService.resolveSession(token).map(s -> s.publicId()).orElse(null);
        return roomService.listLobbyRooms(requesterPublicId);
    }

    @PutMapping("/{roomId}")
    public ResponseEntity<?> updateRoom(@PathVariable String roomId, @RequestBody RoomUpdateRequest request, org.springframework.web.server.ServerWebExchange exchange) {
        String token = sessionCookieService.sessionToken(exchange);
        return accountService.resolveSession(token)
                .<ResponseEntity<?>>map(session -> doUpdateRoom(roomId, session.publicId(), token, request))
                .orElseGet(() -> ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", "Unknown session token")));
    }

    private ResponseEntity<?> doUpdateRoom(String roomId, String publicId, String token, RoomUpdateRequest request) {
        try {
            RoomInfo updated = roomService.updateRoomSettings(
                    roomId,
                    publicId,
                    roomAuthorizationService.canManageRoom(roomId, publicId, token),
                    request.name(),
                    false, null, false
            );
            return ResponseEntity.ok(updated);
        } catch (IllegalArgumentException ex) {
            HttpStatus status = "No permission to update room".equals(ex.getMessage()) ? HttpStatus.FORBIDDEN : HttpStatus.BAD_REQUEST;
            return ResponseEntity.status(status).body(Map.of("message", ex.getMessage()));
        }
    }

    @DeleteMapping("/{roomId}")
    public ResponseEntity<?> deleteRoomRequest(@PathVariable String roomId, org.springframework.web.server.ServerWebExchange exchange) {
        String token = sessionCookieService.sessionToken(exchange);
        return accountService.resolveSession(token)
                .<ResponseEntity<?>>map(session -> doDeleteRoom(roomId, session.publicId(), token))
                .orElseGet(() -> ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", "Unknown session token")));
    }

    /** Compatibility entry point for callers that still supply a session token directly. */
    public ResponseEntity<?> deleteRoom(String roomId, String sessionToken) {
        if (legacyUserService == null || sessionToken == null) {
            return ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", "Unknown session token"));
        }
        return legacyUserService.resolvePublicIdBySessionToken(sessionToken)
                .<ResponseEntity<?>>map(publicId -> {
                    boolean deleted = roomLifecycleService.deleteRoom(
                            roomId, publicId, accountService.isAdminSession(sessionToken));
                    if (!deleted) {
                        return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "No permission to delete room"));
                    }
                    return ResponseEntity.ok(Map.of("message", "ROOM DELETED"));
                })
                .orElseGet(() -> ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", "Unknown session token")));
    }

    private ResponseEntity<?> doDeleteRoom(String roomId, String publicId, String sessionToken) {
        boolean deleted = roomLifecycleService.deleteRoom(roomId, publicId, roomAuthorizationService.isPlatformAdmin(sessionToken));
        if (!deleted) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message", "No permission to delete room"));
        }
        return ResponseEntity.ok(Map.of("message", "ROOM DELETED"));
    }
}
