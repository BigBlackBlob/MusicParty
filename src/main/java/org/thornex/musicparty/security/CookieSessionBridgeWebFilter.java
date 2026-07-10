package org.thornex.musicparty.security;

import org.springframework.core.Ordered;
import org.springframework.core.annotation.Order;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.stereotype.Component;
import org.springframework.web.server.ServerWebExchange;
import org.springframework.web.server.WebFilter;
import org.springframework.web.server.WebFilterChain;
import org.springframework.web.util.UriComponentsBuilder;
import reactor.core.publisher.Mono;

import java.net.URI;

/** Bridges legacy controller request parameters without exposing session tokens to the browser URL. */
@Component
@Order(Ordered.HIGHEST_PRECEDENCE + 10)
public class CookieSessionBridgeWebFilter implements WebFilter {
    private final SessionCookieService cookies;

    public CookieSessionBridgeWebFilter(SessionCookieService cookies) { this.cookies = cookies; }

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, WebFilterChain chain) {
        String token = cookies.sessionToken(exchange);
        if (token == null || !exchange.getRequest().getQueryParams().isEmpty()
                && (exchange.getRequest().getQueryParams().containsKey("sessionToken") || exchange.getRequest().getQueryParams().containsKey("token"))) {
            return chain.filter(exchange);
        }
        URI uri = UriComponentsBuilder.fromUri(exchange.getRequest().getURI())
                .queryParam("sessionToken", token).queryParam("token", token).build(true).toUri();
        ServerHttpRequest request = exchange.getRequest().mutate().uri(uri).build();
        return chain.filter(exchange.mutate().request(request).build());
    }
}
