package org.thornex.musicparty.service;

import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.User;

import java.util.Locale;
import java.util.Set;
import java.util.Arrays;
import java.util.stream.Collectors;

@Service
@Slf4j
public class NavidromeAccessService {

    private final AppProperties appProperties;
    private final UserService userService;
    private volatile Set<String> allowedNames;

    public NavidromeAccessService(AppProperties appProperties, UserService userService) {
        this.appProperties = appProperties;
        this.userService = userService;
        refreshAllowedNames();
    }

    public boolean isEnabled() {
        return appProperties.getNavidrome() != null && appProperties.getNavidrome().isEnabled();
    }

    public boolean isConfigured() {
        AppProperties.NavidromeApiConfig config = appProperties.getNavidrome();
        return config != null
                && StringUtils.hasText(config.getBaseUrl())
                && StringUtils.hasText(config.getUsername())
                && StringUtils.hasText(config.getPassword());
    }

    public boolean canUseBySession(String sessionId) {
        if (!isEnabled() || !isConfigured()) return false;
        return userService.getUser(sessionId)
                .map(this::canUseUser)
                .orElse(false);
    }

    public boolean canUseByToken(String token) {
        if (!isEnabled() || !isConfigured()) return false;
        return userService.getUserByToken(token)
                .map(this::canUseUser)
                .orElse(false);
    }

    public boolean canUseUser(User user) {
        if (!isEnabled() || !isConfigured()) return false;
        if (user == null || user.isGuest()) return false;
        if (!StringUtils.hasText(user.getName())) return false;
        Set<String> names = getAllowedUserNames();
        return names.contains(normalize(user.getName()));
    }

    public Set<String> getAllowedUserNames() {
        if (allowedNames == null) {
            refreshAllowedNames();
        }
        return allowedNames;
    }

    public void refreshAllowedNames() {
        String raw = appProperties.getNavidrome() != null ? appProperties.getNavidrome().getAllowedUsers() : "";
        this.allowedNames = parseAllowedNames(raw);
        log.info("Navidrome allowed users refreshed: count={}", this.allowedNames.size());
    }

    static Set<String> parseAllowedNames(String raw) {
        if (!StringUtils.hasText(raw)) {
            return Set.of();
        }
        return Arrays.stream(raw.split(","))
                .map(String::trim)
                .filter(StringUtils::hasText)
                .map(NavidromeAccessService::normalize)
                .collect(Collectors.toUnmodifiableSet());
    }

    public static String normalize(String name) {
        return name.trim().toLowerCase(Locale.ROOT);
    }
}
