package org.thornex.musicparty.controller;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.thornex.musicparty.dto.AccountAuthRequests.ChangePasswordRequest;
import org.thornex.musicparty.dto.AccountAuthRequests.LoginRequest;
import org.thornex.musicparty.dto.AccountAuthRequests.RegisterRequest;
import org.thornex.musicparty.dto.AccountAuthRequests.UpdateProfileRequest;
import org.thornex.musicparty.security.ClientIpResolver;
import org.thornex.musicparty.security.LoginRateLimiter;
import org.thornex.musicparty.security.SessionCookieService;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.AccountSession;

import java.util.Map;

@RestController
@RequestMapping("/api/account")
public class AccountAuthController {
    public static final String SESSION_HEADER = "X-Session-Token";

    private final AccountService accountService;
    private final LoginRateLimiter loginRateLimiter;
    private final ClientIpResolver clientIpResolver;
    private final SessionCookieService sessionCookieService;

    public AccountAuthController(AccountService accountService,
                                  LoginRateLimiter loginRateLimiter,
                                  ClientIpResolver clientIpResolver,
                                  SessionCookieService sessionCookieService) {
        this.accountService = accountService;
        this.loginRateLimiter = loginRateLimiter;
        this.clientIpResolver = clientIpResolver;
        this.sessionCookieService = sessionCookieService;
    }

    @GetMapping("/status")
    public ResponseEntity<?> status() {
        return ResponseEntity.ok(accountService.status());
    }

    @PostMapping("/register")
    public ResponseEntity<?> register(@RequestBody RegisterRequest request, org.springframework.web.server.ServerWebExchange exchange) {
        try {
            AccountSession session = accountService.register(request.username(), request.password());
            sessionCookieService.establish(exchange, session.sessionToken());
            return ResponseEntity.ok(withoutToken(session));
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(Map.of("message", e.getMessage()));
        }
    }

    @PostMapping("/login")
    public ResponseEntity<?> login(@RequestBody LoginRequest request, ServerHttpRequest httpRequest, org.springframework.web.server.ServerWebExchange exchange) {
        String ip = clientIpResolver.resolve(httpRequest);
        if (loginRateLimiter.isBlocked(ip)) {
            long retryAfter = Math.max(1L, (loginRateLimiter.retryAfterMillis(ip) + 999L) / 1000L);
            return ResponseEntity.status(HttpStatus.TOO_MANY_REQUESTS)
                    .header("Retry-After", String.valueOf(retryAfter))
                    .body(Map.of("message", "Too many login attempts. Try again later."));
        }
        try {
            AccountSession session = accountService.login(request.username(), request.password());
            loginRateLimiter.recordSuccess(ip);
            sessionCookieService.establish(exchange, session.sessionToken());
            return ResponseEntity.ok(withoutToken(session));
        } catch (IllegalArgumentException e) {
            loginRateLimiter.recordFailure(ip);
            return ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", e.getMessage()));
        }
    }

    @GetMapping("/me")
    public ResponseEntity<?> me(org.springframework.web.server.ServerWebExchange exchange) {
        String sessionToken = sessionCookieService.sessionToken(exchange);
        return accountService.resolveSession(sessionToken)
                .<ResponseEntity<?>>map(session -> ResponseEntity.ok(withoutToken(session)))
                .orElseGet(() -> ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", "Unknown session token")));
    }

    @PostMapping("/logout")
    public ResponseEntity<AccountSession> logout(org.springframework.web.server.ServerWebExchange exchange) {
        String sessionToken = sessionCookieService.sessionToken(exchange);
        accountService.logout(sessionToken);
        sessionCookieService.clear(exchange);
        return ResponseEntity.noContent().build();
    }

    @PostMapping("/change-password")
    public ResponseEntity<?> changePassword(
            org.springframework.web.server.ServerWebExchange exchange,
            @RequestBody ChangePasswordRequest request
    ) {
        try {
            String sessionToken = sessionCookieService.sessionToken(exchange);
            accountService.changePassword(sessionToken, request.currentPassword(), request.newPassword());
            accountService.logout(sessionToken);
            sessionCookieService.clear(exchange);
            return ResponseEntity.noContent().build();
        } catch (IllegalArgumentException e) {
            HttpStatus status = "Unknown session token".equals(e.getMessage()) ? HttpStatus.UNAUTHORIZED : HttpStatus.BAD_REQUEST;
            return ResponseEntity.status(status).body(Map.of("message", e.getMessage()));
        }
    }

    @PutMapping("/profile")
    public ResponseEntity<?> updateProfile(
            org.springframework.web.server.ServerWebExchange exchange,
            @RequestBody UpdateProfileRequest request
    ) {
        try {
            return ResponseEntity.ok(withoutToken(accountService.updateProfile(sessionCookieService.sessionToken(exchange), request.displayName())));
        } catch (IllegalArgumentException e) {
            HttpStatus status = "Unknown session token".equals(e.getMessage()) ? HttpStatus.UNAUTHORIZED : HttpStatus.BAD_REQUEST;
            return ResponseEntity.status(status).body(Map.of("message", e.getMessage()));
        }
    }

    private AccountSession withoutToken(AccountSession session) {
        return new AccountSession("", session.publicId(), session.username(), session.displayName(), session.role(), session.guest(), session.enabled(), session.lastLoginAt());
    }
}
