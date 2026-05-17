package org.thornex.musicparty.service.stream;

import org.springframework.stereotype.Component;

import java.security.SecureRandom;
import java.util.Base64;

@Component
public class InternalStreamProxyToken {
    public static final String HEADER_NAME = "X-MusicParty-Internal-Stream-Token";

    private final String token;

    public InternalStreamProxyToken() {
        byte[] bytes = new byte[32];
        new SecureRandom().nextBytes(bytes);
        this.token = Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
    }

    public String value() {
        return token;
    }

    public boolean matches(String candidate) {
        return token.equals(candidate);
    }
}
