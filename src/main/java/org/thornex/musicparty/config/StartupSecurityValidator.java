package org.thornex.musicparty.config;

import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;

import java.net.URI;
import java.util.Arrays;
import java.util.Locale;

@Component
public class StartupSecurityValidator implements ApplicationRunner {

    private final AppProperties appProperties;

    public StartupSecurityValidator(AppProperties appProperties) {
        this.appProperties = appProperties;
    }

    StartupSecurityValidator() {
        this(new AppProperties());
    }

    @Override
    public void run(ApplicationArguments args) {
        if (!"server".equalsIgnoreCase(appProperties.getMode())) {
            return;
        }
        if (!looksLikeProduction()) {
            return;
        }
        if (isLocalhostUrl(appProperties.getBaseUrl())) {
            throw new IllegalStateException("BASE_URL must not point to localhost in server production deployments");
        }
        if (!hasPublicAllowedOrigin()) {
            throw new IllegalStateException("ALLOWED_ORIGINS must explicitly include the public origin in server production deployments");
        }
    }

    private boolean looksLikeProduction() {
        return isProdValue(System.getenv("APP_ENV"))
                || isProdValue(System.getenv("ENV"))
                || isProdValue(System.getenv("NODE_ENV"))
                || profileIncludesProd(System.getenv("SPRING_PROFILES_ACTIVE"))
                || hasPublicAllowedOrigin()
                || (StringUtils.hasText(appProperties.getBaseUrl()) && !isLocalhostUrl(appProperties.getBaseUrl()));
    }

    private boolean hasPublicAllowedOrigin() {
        if (!StringUtils.hasText(appProperties.getAllowedOrigins())) {
            return false;
        }
        return Arrays.stream(appProperties.getAllowedOrigins().split(","))
                .map(String::trim)
                .filter(StringUtils::hasText)
                .anyMatch(origin -> origin.startsWith("http://") || origin.startsWith("https://"))
                && Arrays.stream(appProperties.getAllowedOrigins().split(","))
                .map(String::trim)
                .filter(StringUtils::hasText)
                .anyMatch(origin -> !isLocalhostUrl(origin));
    }

    private boolean profileIncludesProd(String value) {
        if (!StringUtils.hasText(value)) {
            return false;
        }
        return Arrays.stream(value.split(",")).anyMatch(this::isProdValue);
    }

    private boolean isProdValue(String value) {
        return StringUtils.hasText(value) && "prod".equals(value.trim().toLowerCase(Locale.ROOT))
                || StringUtils.hasText(value) && "production".equals(value.trim().toLowerCase(Locale.ROOT));
    }

    private boolean isLocalhostUrl(String value) {
        if (!StringUtils.hasText(value)) {
            return false;
        }
        try {
            String host = URI.create(value.trim()).getHost();
            if (!StringUtils.hasText(host)) {
                return false;
            }
            String normalized = host.toLowerCase(Locale.ROOT);
            return "localhost".equals(normalized)
                    || "127.0.0.1".equals(normalized)
                    || "0.0.0.0".equals(normalized)
                    || "::1".equals(normalized);
        } catch (IllegalArgumentException e) {
            return false;
        }
    }
}
