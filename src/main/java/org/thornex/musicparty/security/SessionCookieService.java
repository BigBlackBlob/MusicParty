package org.thornex.musicparty.security;

import org.springframework.http.ResponseCookie;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.reactive.socket.WebSocketSession;
import org.thornex.musicparty.config.AppProperties;

import java.security.SecureRandom;
import java.util.Base64;

@Component
public class SessionCookieService {
    public static final String SESSION_COOKIE = "MP_SESSION";
    public static final String CSRF_COOKIE = "MP_CSRF";
    public static final String CSRF_HEADER = "X-CSRF-Token";

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

    public void establish(ServerWebExchange exchange, String sessionToken) {
        exchange.getResponse().addCookie(ResponseCookie.from(SESSION_COOKIE, sessionToken).httpOnly(true)
                .secure(appProperties.getAuth().isSecureCookies()).sameSite("Strict").path("/").build());
        exchange.getResponse().addCookie(ResponseCookie.from(CSRF_COOKIE, randomToken()).httpOnly(false)
                .secure(appProperties.getAuth().isSecureCookies()).sameSite("Strict").path("/").build());
    }

    public void clear(ServerWebExchange exchange) {
        exchange.getResponse().addCookie(expired(SESSION_COOKIE, true));
        exchange.getResponse().addCookie(expired(CSRF_COOKIE, false));
    }

    public boolean csrfMatches(ServerWebExchange exchange) {
        var cookie = exchange.getRequest().getCookies().getFirst(CSRF_COOKIE);
        String header = exchange.getRequest().getHeaders().getFirst(CSRF_HEADER);
        return cookie != null && StringUtils.hasText(header) && SecureCompare.equals(cookie.getValue(), header);
    }

    private ResponseCookie expired(String name, boolean httpOnly) {
        return ResponseCookie.from(name, "").httpOnly(httpOnly).secure(appProperties.getAuth().isSecureCookies())
                .sameSite("Strict").path("/").maxAge(0).build();
    }

    private String randomToken() {
        byte[] bytes = new byte[32];
        random.nextBytes(bytes);
        return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
    }
}
