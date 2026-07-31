package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ServerWebExchange;
import org.thornex.musicparty.security.SessionCookieService;
import org.thornex.musicparty.service.InvitationService;
import org.thornex.musicparty.service.RoomService;
import java.util.Map;

@RestController
@RequiredArgsConstructor
public class InvitationController {
    private final InvitationService invitations;
    private final RoomService rooms;
    private final SessionCookieService cookies;

    @GetMapping("/api/join/{secret}/metadata")
    public ResponseEntity<?> metadata(@PathVariable String secret) {
        var metadata = invitations.metadata(secret);
        if (!metadata.valid()) return ResponseEntity.status(HttpStatus.NOT_FOUND).body(Map.of("valid", false));
        return ResponseEntity.ok(Map.of("valid", true, "roomId", metadata.roomId(), "roomName", rooms.getRoom(metadata.roomId()).name(), "expiresAt", metadata.expiresAt()));
    }

    @PostMapping("/api/invites/redeem")
    public ResponseEntity<?> redeem(@RequestBody RedeemInviteRequest request, ServerWebExchange exchange) {
        try {
            var session = invitations.redeem(request.secret(), request.displayName());
            cookies.establish(exchange, session.sessionToken());
            return ResponseEntity.ok(new SessionView(session.publicId(), session.displayName(), session.role(), session.guest(), session.enabled(), session.lastLoginAt()));
        } catch (IllegalArgumentException ex) {
            return ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", "Invitation is invalid or expired"));
        }
    }
    public record RedeemInviteRequest(String secret, String displayName) {}
    public record SessionView(String publicId, String displayName, String role, boolean guest, boolean enabled, Long lastLoginAt) {}
}
