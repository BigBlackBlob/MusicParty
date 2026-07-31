package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ServerWebExchange;
import org.thornex.musicparty.persistence.RoomAccessRepository;
import org.thornex.musicparty.security.SessionCookieService;
import org.thornex.musicparty.service.*;
import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/api/rooms/{roomId}")
@RequiredArgsConstructor
public class RoomMembershipController {
    private final AccountService accounts;
    private final RoomAuthorizationService authorization;
    private final RoomAccessRepository access;
    private final InvitationService invitations;
    private final SessionCookieService cookies;

    @PostMapping("/invites")
    public ResponseEntity<?> createInvite(@PathVariable String roomId, @RequestBody(required = false) InviteRequest request, ServerWebExchange exchange) {
        var actor = actor(exchange); if (actor == null || !authorization.canManageRoom(roomId, actor.publicId(), cookies.sessionToken(exchange))) return forbidden();
        var invite = invitations.create(roomId, actor.publicId(), request == null ? null : request.label());
        return ResponseEntity.ok(Map.of("id", invite.id(), "secret", invite.secret(), "expiresAt", invite.expiresAt(), "label", invite.label() == null ? "" : invite.label()));
    }
    @GetMapping("/invites") public ResponseEntity<?> listInvites(@PathVariable String roomId, ServerWebExchange exchange) {
        var actor=actor(exchange); if(actor==null||!authorization.canManageRoom(roomId,actor.publicId(),cookies.sessionToken(exchange)))return forbidden();
        return ResponseEntity.ok(access.listInvites(roomId).stream().map(i -> Map.of("id",i.id(),"label",i.label()==null?"":i.label(),"expiresAt",i.expiresAt(),"usedAt",i.usedAt()==null?0L:i.usedAt(),"revokedAt",i.revokedAt()==null?0L:i.revokedAt())).toList());
    }
    @DeleteMapping("/invites/{inviteId}") public ResponseEntity<?> revokeInvite(@PathVariable String roomId,@PathVariable String inviteId,ServerWebExchange exchange){var actor=actor(exchange);if(actor==null||!authorization.canManageRoom(roomId,actor.publicId(),cookies.sessionToken(exchange)))return forbidden();return invitations.revoke(roomId,inviteId)?ResponseEntity.noContent().build():ResponseEntity.notFound().build();}
    @GetMapping("/members") public ResponseEntity<?> members(@PathVariable String roomId,ServerWebExchange exchange){var actor=actor(exchange);if(actor==null||!authorization.canManageRoom(roomId,actor.publicId(),cookies.sessionToken(exchange)))return forbidden();return ResponseEntity.ok(access.listMemberships(roomId));}
    @DeleteMapping("/members/{publicId}") public ResponseEntity<?> removeMember(@PathVariable String roomId,@PathVariable String publicId,ServerWebExchange exchange){var actor=actor(exchange); if(actor==null||!authorization.canManageRoom(roomId,actor.publicId(),cookies.sessionToken(exchange)))return forbidden();var membership=access.findMembership(roomId,publicId).orElse(null);if(membership==null)return ResponseEntity.notFound().build();if(membership.owner()&&access.countOwners(roomId)<=1)return ResponseEntity.badRequest().body(Map.of("message","A room must retain an owner"));access.deleteMembership(roomId,publicId);return ResponseEntity.noContent().build();}
    @PostMapping("/owners/{publicId}") public ResponseEntity<?> makeOwner(@PathVariable String roomId,@PathVariable String publicId,ServerWebExchange exchange){var actor=actor(exchange);if(actor==null||!authorization.isPlatformAdmin(cookies.sessionToken(exchange)))return forbidden();var m=access.findMembership(roomId,publicId).orElse(null);if(m==null)return ResponseEntity.notFound().build();access.upsertMembership(new org.thornex.musicparty.persistence.PersistedRoomMembership(roomId,publicId,"OWNER",m.createdAt(),System.currentTimeMillis()));return ResponseEntity.noContent().build();}
    @DeleteMapping("/owners/{publicId}") public ResponseEntity<?> removeOwner(@PathVariable String roomId,@PathVariable String publicId,ServerWebExchange exchange){var actor=actor(exchange);if(actor==null||!authorization.isPlatformAdmin(cookies.sessionToken(exchange)))return forbidden();var m=access.findMembership(roomId,publicId).orElse(null);if(m==null)return ResponseEntity.notFound().build();if(access.countOwners(roomId)<=1)return ResponseEntity.badRequest().body(Map.of("message","A room must retain an owner"));access.upsertMembership(new org.thornex.musicparty.persistence.PersistedRoomMembership(roomId,publicId,"MEMBER",m.createdAt(),System.currentTimeMillis()));return ResponseEntity.noContent().build();}
    private AccountSession actor(ServerWebExchange exchange){return accounts.resolveSession(cookies.sessionToken(exchange)).orElse(null);}
    private ResponseEntity<Map<String,String>> forbidden(){return ResponseEntity.status(HttpStatus.FORBIDDEN).body(Map.of("message","ACCESS DENIED"));}
    public record InviteRequest(String label) {}
}
