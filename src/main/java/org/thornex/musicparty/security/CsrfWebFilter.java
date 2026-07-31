package org.thornex.musicparty.security;

import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Component;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import reactor.core.publisher.Mono;

@Component
@Order(Ordered.HIGHEST_PRECEDENCE + 20)
public class CsrfWebFilter implements WebFilter {
    private final SessionCookieService cookies;
    public CsrfWebFilter(SessionCookieService cookies) { this.cookies = cookies; }

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        HttpMethod method = exchange.getRequest().getMethod();
        String path = exchange.getRequest().getPath().value();
        if (method == null || method == HttpMethod.GET || method == HttpMethod.HEAD || method == HttpMethod.OPTIONS
                || path.equals("/api/invites/redeem") || path.equals("/api/account/login")
                || path.equals("/api/admin/auth/login")) return chain.filter(exchange);
        if (cookies.sessionToken(exchange) == null || !cookies.csrfMatches(exchange)) {
            exchange.getResponse().setStatusCode(HttpStatus.FORBIDDEN);
            return exchange.getResponse().setComplete();
        }
        return chain.filter(exchange);
    }
}
