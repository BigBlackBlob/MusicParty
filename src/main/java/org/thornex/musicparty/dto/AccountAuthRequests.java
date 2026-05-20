package org.thornex.musicparty.dto;

public final class AccountAuthRequests {
    private AccountAuthRequests() {
    }

    public record RegisterRequest(String username, String password) {
    }

    public record LoginRequest(String username, String password) {
    }
}
