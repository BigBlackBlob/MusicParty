package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.mock.http.server.reactive.MockServerHttpRequest;
import org.springframework.mock.web.server.MockServerWebExchange;
import org.thornex.musicparty.dto.AccountAuthRequests.LoginRequest;
import org.thornex.musicparty.security.ClientIpResolver;
import org.thornex.musicparty.security.LoginRateLimiter;
import org.thornex.musicparty.security.SessionCookieService;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.AccountSession;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class AccountAuthControllerTests {

    @Test
    void platformAdminCanLoginWhileMemberPasswordLoginIsRejected() {
        AccountService accounts = mock(AccountService.class);
        LoginRateLimiter rateLimiter = mock(LoginRateLimiter.class);
        ClientIpResolver ipResolver = mock(ClientIpResolver.class);
        SessionCookieService cookies = mock(SessionCookieService.class);
        AccountAuthController controller = new AccountAuthController(accounts, rateLimiter, ipResolver, cookies);
        MockServerWebExchange exchange = MockServerWebExchange.from(MockServerHttpRequest.post("/api/account/login"));
        when(ipResolver.resolve(any())).thenReturn("127.0.0.1");

        AccountSession admin = new AccountSession(
                "admin-token", "u_admin", "admin", "Admin", "PLATFORM_ADMIN", false, true, 1L);
        when(accounts.login("admin", "secret-password")).thenReturn(admin);

        var adminResponse = controller.loginPlatformAdmin(
                new LoginRequest("admin", "secret-password"), exchange.getRequest(), exchange);

        assertThat(adminResponse.getStatusCode()).isEqualTo(HttpStatus.OK);
        verify(cookies).establishAdmin(exchange, "admin-token");
        verify(rateLimiter).recordSuccess("127.0.0.1");

        AccountSession member = new AccountSession(
                "member-token", "u_member", "member", "Member", "MEMBER", false, true, 1L);
        when(accounts.login("member", "secret-password")).thenReturn(member);

        var memberResponse = controller.loginPlatformAdmin(
                new LoginRequest("member", "secret-password"), exchange.getRequest(), exchange);

        assertThat(memberResponse.getStatusCode()).isEqualTo(HttpStatus.FORBIDDEN);
        verify(accounts).logout("member-token");
        verify(cookies, never()).establishAdmin(exchange, "member-token");
    }

    @Test
    void passwordRegistrationIsGone() {
        AccountAuthController controller = new AccountAuthController(
                mock(AccountService.class), mock(LoginRateLimiter.class),
                mock(ClientIpResolver.class), mock(SessionCookieService.class));

        assertThat(controller.retiredMemberRegistrationEndpoint().getStatusCode()).isEqualTo(HttpStatus.GONE);
    }
}
