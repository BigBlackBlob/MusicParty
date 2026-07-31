package org.thornex.musicparty.security;

import org.springframework.http.ResponseCookie;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.reactive.socket.WebSocketSession;
import org.thornex.musicparty.config.AppProperties;

import java.security.SecureRandom;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Base64;

@Component
public class SessionCookieService {
    public static final String SESSION_COOKIE = "MP_SESSION";
    public static final String CSRF_COOKIE = "MP_CSRF";
    public static final String CSRF_HEADER = "X-CSRF-Token";
    public static final String ADMIN_ELEVATION_COOKIE = "MP_ADMIN_ELEVATION";
    private static final long MEMBER_SESSION_SECONDS = 90L * 24 * 60 * 60;
    private static final long ADMIN_SESSION_SECONDS = 12L * 60 * 60;
    private static final long ELEVATION_SECONDS = 10L * 60;

    private final AppProperties appProperties;
    private final SecureRandom random = new SecureRandom();

    public SessionCookieService(AppProperties appProperties) { this.appProperties = appProperties; }

    public String sessionToken(ServerWebExchange exchange) {
        var cookie = exchange.getRequest().getCookies().getFirst(SESSION_COOKIE);
        return cookie == null ? null : cookie.getValue();
    }

    public String sessionToken(WebSocketSession session) {
        var cookie = session.getHandshakeInfo().getCookies().getFirst(SESSION_COOKIE);
        return cookie == null ? null : cookie.getValue();
    }

    /** Logs can correlate a handshake without exposing the MP_SESSION credential. */
    public String sessionTokenFingerprint(WebSocketSession session) {
        String token = sessionToken(session);
        if (!StringUtils.hasText(token)) {
            return "none";
        }
        try {
            byte[] digest = MessageDigest.getInstance("SHA-256").digest(token.getBytes(java.nio.charset.StandardCharsets.UTF_8));
            return java.util.HexFormat.of().formatHex(digest, 0, 6);
        } catch (NoSuchAlgorithmException ex) {
            throw new IllegalStateException("SHA-256 is unavailable", ex);
        }
    }

    public void establish(ServerWebExchange exchange, String sessionToken) {
        boolean secure = shouldUseSecureCookie(exchange);
        long maxAge =  MEMBER_SESSION_SECONDS;
        exchange.getResponse().addCookie(ResponseCookie.from(SESSION_COOKIE, sessionToken).httpOnly(true)
                .secure(secure).sameSite("Lax").path("/").maxAge(maxAge).build());
        exchange.getResponse().addCookie(ResponseCookie.from(CSRF_COOKIE, randomToken()).httpOnly(false)
                .secure(secure).sameSite("Lax").path("/").maxAge(maxAge).build());
    }

    public void establishAdmin(ServerWebExchange exchange, String sessionToken) {
        boolean secure = shouldUseSecureCookie(exchange);
        exchange.getResponse().addCookie(ResponseCookie.from(SESSION_COOKIE, sessionToken).httpOnly(true)
                .secure(secure).sameSite("Lax").path("/").maxAge(ADMIN_SESSION_SECONDS).build());
        exchange.getResponse().addCookie(ResponseCookie.from(CSRF_COOKIE, randomToken()).httpOnly(false)
                .secure(secure).sameSite("Lax").path("/").maxAge(ADMIN_SESSION_SECONDS).build());
    }

    public void establishElevation(ServerWebExchange exchange, String value) {
        exchange.getResponse().addCookie(ResponseCookie.from(ADMIN_ELEVATION_COOKIE, value).httpOnly(true)
                .secure(shouldUseSecureCookie(exchange)).sameSite("Strict").path("/api/admin").maxAge(ELEVATION_SECONDS).build());
    }

    public void clear(ServerWebExchange exchange) {
        boolean secure = shouldUseSecureCookie(exchange);
        exchange.getResponse().addCookie(expired(SESSION_COOKIE, true, secure));
        exchange.getResponse().addCookie(expired(CSRF_COOKIE, false, secure));
        exchange.getResponse().addCookie(ResponseCookie.from(ADMIN_ELEVATION_COOKIE, "").httpOnly(true).secure(secure)
                .sameSite("Strict").path("/api/admin").maxAge(0).build());
    }

    public boolean csrfMatches(ServerWebExchange exchange) {
        var cookie = exchange.getRequest().getCookies().getFirst(CSRF_COOKIE);
        String header = exchange.getRequest().getHeaders().getFirst(CSRF_HEADER);
        return cookie != null && StringUtils.hasText(header) && SecureCompare.equals(cookie.getValue(), header);
    }

    private boolean shouldUseSecureCookie(ServerWebExchange exchange) {
        if (!appProperties.getAuth().isSecureCookies()) return false;
        String forwardedProto = exchange.getRequest().getHeaders().getFirst("X-Forwarded-Proto");
        if (StringUtils.hasText(forwardedProto)) {
            return "https".equalsIgnoreCase(forwardedProto.split(",", 2)[0].trim());
        }
        return "https".equalsIgnoreCase(exchange.getRequest().getURI().getScheme());
    }

    private ResponseCookie expired(String name, boolean httpOnly, boolean secure) {
        return ResponseCookie.from(name, "").httpOnly(httpOnly).secure(secure)
                .sameSite("Strict").path("/").maxAge(0).build();
    }

    private String randomToken() {
        byte[] bytes = new byte[32];
        random.nextBytes(bytes);
        return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
    }
}
