package org.thornex.musicparty.config;

import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;

import java.util.Locale;
import java.util.Set;

@Component
public class StartupSecurityValidator implements ApplicationRunner {

    private static final Set<String> INSECURE_ADMIN_PASSWORDS = Set.of(
            "admin",
            "admin123",
            "password",
            "change-me",
            "changeme"
    );

    private final AppProperties appProperties;

    public StartupSecurityValidator(AppProperties appProperties) {
        this.appProperties = appProperties;
    }

    @Override
    public void run(ApplicationArguments args) {
        String password = appProperties.getAdminPassword();
        if (!StringUtils.hasText(password)) {
            throw new IllegalStateException("ADMIN_PASSWORD must be set before starting MusicParty.");
        }
        if (INSECURE_ADMIN_PASSWORDS.contains(password.trim().toLowerCase(Locale.ROOT))) {
            throw new IllegalStateException("ADMIN_PASSWORD is using an insecure default value.");
        }
    }
}
