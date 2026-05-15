package org.thornex.musicparty.service;

import jakarta.annotation.PostConstruct;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.config.AppProperties;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.util.Base64;

@Service
public class RoomAccessService {
    private static final Base64.Encoder URL_ENCODER = Base64.getUrlEncoder().withoutPadding();
    private static final Base64.Decoder URL_DECODER = Base64.getUrlDecoder();

    private final AppProperties appProperties;
    private final RoomService roomService;
    private volatile byte[] signingSecret;

    public RoomAccessService(AppProperties appProperties, RoomService roomService) {
        this.appProperties = appProperties;
        this.roomService = roomService;
    }

    @PostConstruct
    void init() {
        String configuredSecret = appProperties.getAuth().getRoomAccessTokenSecret();
        if (StringUtils.hasText(configuredSecret)) {
            signingSecret = configuredSecret.getBytes(StandardCharsets.UTF_8);
            return;
        }

        byte[] randomSecret = new byte[32];
        new SecureRandom().nextBytes(randomSecret);
        signingSecret = randomSecret;
    }

    public RoomAccessGrant verifyAccess(String roomId, String publicId, String password) {
        ensureSigningSecret();
        RoomService.RoomAccessMetadata room = roomService.getRoomAccessMetadata(roomId)
                .orElseThrow(() -> new IllegalArgumentException("Room not found"));
        if (!room.privateRoom()) {
            return RoomAccessGrant.allowedWithoutToken();
        }
        if (!StringUtils.hasText(publicId)) {
            return RoomAccessGrant.denied();
        }
        if (roomService.isAdminPassword(password) || RoomPasswordHasher.matches(password, room.passwordHash())) {
            long expiresAt = System.currentTimeMillis() + appProperties.getAuth().getRoomAccessTokenTtlMs();
            return RoomAccessGrant.allowedWithToken(issueToken(room.roomId(), publicId, expiresAt, room.passwordVersion()), expiresAt);
        }
        return RoomAccessGrant.denied();
    }

    public boolean validateAccessToken(String roomId, String publicId, String roomAccessToken) {
        ensureSigningSecret();
        RoomService.RoomAccessMetadata room = roomService.getRoomAccessMetadata(roomId).orElse(null);
        if (room == null) {
            return false;
        }
        if (!room.privateRoom()) {
            return true;
        }
        if (!StringUtils.hasText(roomAccessToken) || !StringUtils.hasText(publicId)) {
            return false;
        }

        String[] parts = roomAccessToken.split("\\.");
        if (parts.length != 2) {
            return false;
        }

        byte[] payloadBytes;
        byte[] actualSignature;
        try {
            payloadBytes = URL_DECODER.decode(parts[0]);
            actualSignature = URL_DECODER.decode(parts[1]);
        } catch (IllegalArgumentException ignored) {
            return false;
        }

        byte[] expectedSignature = hmac(payloadBytes);
        if (!MessageDigest.isEqual(expectedSignature, actualSignature)) {
            return false;
        }

        String[] payload = new String(payloadBytes, StandardCharsets.UTF_8).split("\\|", -1);
        if (payload.length != 4) {
            return false;
        }

        long expiresAt;
        int passwordVersion;
        try {
            expiresAt = Long.parseLong(payload[2]);
            passwordVersion = Integer.parseInt(payload[3]);
        } catch (NumberFormatException ignored) {
            return false;
        }

        return room.roomId().equals(payload[0])
                && publicId.equals(payload[1])
                && expiresAt >= System.currentTimeMillis()
                && passwordVersion == room.passwordVersion();
    }

    private String issueToken(String roomId, String publicId, long expiresAt, int passwordVersion) {
        String payload = roomId + "|" + publicId + "|" + expiresAt + "|" + passwordVersion;
        byte[] payloadBytes = payload.getBytes(StandardCharsets.UTF_8);
        byte[] signatureBytes = hmac(payloadBytes);
        return URL_ENCODER.encodeToString(payloadBytes) + "." + URL_ENCODER.encodeToString(signatureBytes);
    }

    private byte[] hmac(byte[] payload) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(signingSecret, "HmacSHA256"));
            return mac.doFinal(payload);
        } catch (Exception e) {
            throw new IllegalStateException("Unable to sign room access token", e);
        }
    }

    private void ensureSigningSecret() {
        if (signingSecret == null || signingSecret.length == 0) {
            init();
        }
    }
}
