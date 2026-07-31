package org.thornex.musicparty.security;

import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.stereotype.Component;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;

/**
 * Deliberately does not bridge URL/header tokens. Controllers resolve identity only
 * from the HttpOnly session cookie; keeping this filter as a no-op avoids a
 * compatibility bean silently reintroducing browser-visible credentials.
 */
@Component
@Order(Ordered.HIGHEST_PRECEDENCE + 10)
public class CookieSessionBridgeWebFilter implements WebFilter {
    private final SessionCookieService cookies;

    public CookieSessionBridgeWebFilter(SessionCookieService cookies) { this.cookies = cookies; }

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        return chain.filter(exchange);
    }
}
