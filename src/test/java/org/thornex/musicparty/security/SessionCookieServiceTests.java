package org.thornex.musicparty.security;

import org.junit.jupiter.api.Test;
import org.springframework.mock.http.server.reactive.MockServerHttpRequest;
import org.springframework.mock.web.server.MockServerWebExchange;
import org.thornex.musicparty.config.AppProperties;

import static org.assertj.core.api.Assertions.assertThat;

class SessionCookieServiceTests {

    @Test
    void directHttpUsesNonSecureCookiesEvenWhenSecureCookiesAreEnabled() {
        MockServerWebExchange exchange = exchange(MockServerHttpRequest.post("http://203.0.113.8:8848/api/auth/login"));

        new SessionCookieService(properties(true)).establish(exchange, "session-token");

        assertThat(exchange.getResponse().getCookies().getFirst(SessionCookieService.SESSION_COOKIE).isSecure()).isFalse();
        assertThat(exchange.getResponse().getCookies().getFirst(SessionCookieService.CSRF_COOKIE).isSecure()).isFalse();
    }

    @Test
    void tlsProxyUsesSecureCookies() {
        MockServerHttpRequest request = MockServerHttpRequest.post("http://music.example/api/auth/login")
                .header("X-Forwarded-Proto", "https")
                .build();
        MockServerWebExchange exchange = exchange(request);

        new SessionCookieService(properties(true)).establish(exchange, "session-token");

        assertThat(exchange.getResponse().getCookies().getFirst(SessionCookieService.SESSION_COOKIE).isSecure()).isTrue();
        assertThat(exchange.getResponse().getCookies().getFirst(SessionCookieService.CSRF_COOKIE).isSecure()).isTrue();
    }

    @Test
    void secureCookiesCanBeExplicitlyDisabledForLocalDevelopment() {
        MockServerWebExchange exchange = exchange(MockServerHttpRequest.post("https://music.example/api/auth/login"));

        new SessionCookieService(properties(false)).establish(exchange, "session-token");

        assertThat(exchange.getResponse().getCookies().getFirst(SessionCookieService.SESSION_COOKIE).isSecure()).isFalse();
    }

    private static AppProperties properties(boolean secureCookies) {
        AppProperties properties = new AppProperties();
        properties.getAuth().setSecureCookies(secureCookies);
        return properties;
    }

    private static MockServerWebExchange exchange(MockServerHttpRequest.BaseBuilder<?> request) {
        return MockServerWebExchange.from(request.build());
    }

    private static MockServerWebExchange exchange(MockServerHttpRequest request) {
        return MockServerWebExchange.from(request);
    }
}
