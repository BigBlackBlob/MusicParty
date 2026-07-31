package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.persistence.*;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.util.Base64;
import java.util.UUID;

@Service
@RequiredArgsConstructor
public class InvitationService {
    public static final long INVITE_TTL_MS = 7L * 24 * 60 * 60 * 1000;
    private final RoomAccessRepository accessRepository;
    private final UserProfileRepository profileRepository;
    private final UserAccountRepository accountRepository;
    private final RoomStateMutationService mutationService;
    private final SecureRandom random = new SecureRandom();
    private final BCryptPasswordEncoder passwordEncoder = new BCryptPasswordEncoder();

    public CreatedInvite create(String roomId, String actorPublicId, String label) {
        long now = System.currentTimeMillis();
        byte[] bytes = new byte[32]; random.nextBytes(bytes);
        String secret = Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
        String id = "inv_" + UUID.randomUUID().toString().replace("-", "");
        String normalizedLabel = normalizeLabel(label);
        mutationService.runInTransaction(() -> accessRepository.createInvite(new PersistedRoomInvite(
                id, roomId, actorPublicId, sha256(secret), normalizedLabel,
                now + INVITE_TTL_MS, 1, null, null, null, now)));
        return new CreatedInvite(id, secret, now + INVITE_TTL_MS, normalizedLabel);
    }

    public InviteMetadata metadata(String secret) {
        if (!StringUtils.hasText(secret)) {
            return new InviteMetadata(null, 0, false);
        }
        return accessRepository.findInviteBySecretHash(sha256(secret))
                .filter(i -> i.active(System.currentTimeMillis()))
                .map(i -> new InviteMetadata(i.roomId(), i.expiresAt(), true)).orElse(new InviteMetadata(null, 0, false));
    }

    public AccountSession redeem(String secret, String rawDisplayName) {
        String displayName = normalizeName(rawDisplayName);
        long now = System.currentTimeMillis();
        return mutationService.supplyInTransaction(() -> {
            PersistedRoomInvite invite = accessRepository.findInviteBySecretHash(sha256(secret)).filter(i -> i.active(now))
                    .orElseThrow(() -> new IllegalArgumentException("Invitation is invalid or expired"));
            String publicId = "u_" + UUID.randomUUID().toString().replace("-", "").substring(0, 12);
            String username = "member_" + publicId.substring(2);
            if (!accessRepository.consumeInvite(invite.id(), publicId, now)) {
                throw new IllegalArgumentException("Invitation is invalid or expired");
            }
            profileRepository.upsertProfile(new PersistedUserProfile(publicId, displayName, false, invite.roomId(), now, now));
            accountRepository.create(new PersistedUserAccount(username, publicId, passwordEncoder.encode(UUID.randomUUID().toString()), "MEMBER", true, now, now, now));
            accessRepository.upsertMembership(new PersistedRoomMembership(invite.roomId(), publicId, "MEMBER", now, now));
            String token = UUID.randomUUID().toString();
            profileRepository.upsertSession(new PersistedSession(sha256(token), publicId, now, now));
            return new AccountSession(token, publicId, "", displayName, "MEMBER", false, true, now);
        });
    }

    public boolean revoke(String inviteId) { return accessRepository.revokeInvite(inviteId, System.currentTimeMillis()); }
    public boolean revoke(String roomId, String inviteId) { return accessRepository.revokeInvite(roomId, inviteId, System.currentTimeMillis()); }
    private String normalizeName(String name) { String value=name==null?"":name.trim(); if (!StringUtils.hasText(value)||value.length()>32) throw new IllegalArgumentException("display name must be 1-32 characters"); return value; }
    private String normalizeLabel(String label) { String value=label==null?"":label.trim(); return value.length()>64 ? value.substring(0,64) : value; }
    private String sha256(String value) { try { byte[] hash=MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8)); StringBuilder out=new StringBuilder(); for(byte b:hash)out.append(String.format("%02x",b)); return out.toString(); } catch(Exception e) { throw new IllegalStateException(e); } }
    public record CreatedInvite(String id, String secret, long expiresAt, String label) {}
    public record InviteMetadata(String roomId, long expiresAt, boolean valid) {}
}
