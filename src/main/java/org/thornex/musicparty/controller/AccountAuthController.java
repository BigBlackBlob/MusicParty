package org.thornex.musicparty.controller;

import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.thornex.musicparty.dto.AccountAuthRequests.LoginRequest;
import org.thornex.musicparty.dto.AccountAuthRequests.RegisterRequest;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.AccountSession;

import java.util.Map;

@RestController
@RequestMapping("/api/account")
public class AccountAuthController {
    public static final String SESSION_HEADER = "X-Session-Token";

    private final AccountService accountService;

    public AccountAuthController(AccountService accountService) {
        this.accountService = accountService;
    }

    @GetMapping("/status")
    public ResponseEntity<?> status() {
        return ResponseEntity.ok(accountService.status());
    }

    @PostMapping("/register")
    public ResponseEntity<?> register(@RequestBody RegisterRequest request) {
        try {
            return ResponseEntity.ok(accountService.register(request.username(), request.password()));
        } catch (IllegalArgumentException e) {
            return ResponseEntity.badRequest().body(Map.of("message", e.getMessage()));
        }
    }

    @PostMapping("/login")
    public ResponseEntity<?> login(@RequestBody LoginRequest request) {
        try {
            return ResponseEntity.ok(accountService.login(request.username(), request.password()));
        } catch (IllegalArgumentException e) {
            return ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", e.getMessage()));
        }
    }

    @GetMapping("/me")
    public ResponseEntity<?> me(@RequestHeader(value = SESSION_HEADER, required = false) String sessionToken) {
        return accountService.resolveSession(sessionToken)
                .<ResponseEntity<?>>map(ResponseEntity::ok)
                .orElseGet(() -> ResponseEntity.status(HttpStatus.UNAUTHORIZED).body(Map.of("message", "Unknown session token")));
    }

    @PostMapping("/logout")
    public ResponseEntity<AccountSession> logout(@RequestHeader(value = SESSION_HEADER, required = false) String sessionToken) {
        return ResponseEntity.noContent().build();
    }
}
