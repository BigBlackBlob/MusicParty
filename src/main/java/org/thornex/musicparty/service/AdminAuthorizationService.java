package org.thornex.musicparty.service;

import org.springframework.stereotype.Service;
@Service
public class AdminAuthorizationService {
    private final AccountService accountService;

    public AdminAuthorizationService(AccountService accountService) {
        this.accountService = accountService;
    }

    public boolean isAdminSession(String sessionToken) {
        return accountService.resolveSession(sessionToken).map(AccountSession::admin).orElse(false);
    }

    public boolean isAuthorized(String sessionToken, String legacyAdminPassword) {
        return isAdminSession(sessionToken);
    }

    public boolean isLegacyAdminPassword(String value) {
        return false;
    }
}
