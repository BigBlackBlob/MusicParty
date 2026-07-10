package org.thornex.musicparty.service;

import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.config.AppProperties;

@Component
public class AdminBootstrapService implements ApplicationRunner {
    private final AccountService accountService;
    private final AppProperties appProperties;

    public AdminBootstrapService(AccountService accountService, AppProperties appProperties) {
        this.accountService = accountService;
        this.appProperties = appProperties;
    }

    @Override
    public void run(ApplicationArguments args) {
        if (!"server".equalsIgnoreCase(appProperties.getMode())) {
            return;
        }
        if (accountService.hasAdminAccount()) {
            return;
        }
        String username = appProperties.getBootstrapAdminUsername();
        String password = appProperties.getBootstrapAdminPassword();
        if (!StringUtils.hasText(username) || !StringUtils.hasText(password)) {
            throw new IllegalStateException("BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD are required before the first server start");
        }
        accountService.bootstrapAdmin(username, password);
    }
}
