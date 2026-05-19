package org.thornex.musicparty.service;

import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.persistence.LocalTrackRepository;
import org.thornex.musicparty.security.SecureCompare;

import java.util.LinkedHashSet;
import java.util.Locale;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;
import java.util.stream.Stream;

@Service
public class LocalLibraryAccessService {
    private final AppProperties appProperties;
    private final UserService userService;
    private final LocalTrackRepository repository;

    public LocalLibraryAccessService(AppProperties appProperties, UserService userService, LocalTrackRepository repository) {
        this.appProperties = appProperties;
        this.userService = userService;
        this.repository = repository;
    }

    public boolean isEnabled() {
        return appProperties.getLocalLibrary() != null && appProperties.getLocalLibrary().isEnabled();
    }

    public boolean isAdminPassword(String password) {
        return StringUtils.hasText(appProperties.getAdminPassword())
                && SecureCompare.equals(appProperties.getAdminPassword(), password);
    }

    public boolean canManage(String token, String adminPassword) {
        if (!isEnabled()) return false;
        if (isAdminPassword(adminPassword)) return true;
        return canUploadByToken(token);
    }

    public boolean canUploadByToken(String token) {
        if (!StringUtils.hasText(token)) return false;
        return userService.getUserBySessionToken(token)
                .map(this::canUploadUser)
                .orElse(false);
    }

    public Optional<String> displayNameForToken(String token) {
        if (!StringUtils.hasText(token)) return Optional.empty();
        return userService.getUserBySessionToken(token)
                .map(User::getName)
                .filter(StringUtils::hasText);
    }

    public Set<String> listAllowedUsers() {
        return Stream.concat(parseConfiguredUsers().stream(), repository.findAllowedUploadUsers().stream())
                .filter(StringUtils::hasText)
                .map(LocalLibraryAccessService::normalize)
                .collect(Collectors.toCollection(LinkedHashSet::new));
    }

    public void grant(String userName) {
        String normalized = normalize(userName);
        if (StringUtils.hasText(normalized)) {
            repository.grantUploadUser(normalized, System.currentTimeMillis());
        }
    }

    public void revoke(String userName) {
        String normalized = normalize(userName);
        if (StringUtils.hasText(normalized)) {
            repository.revokeUploadUser(normalized);
        }
    }

    private boolean canUploadUser(User user) {
        if (user == null || user.isGuest() || !StringUtils.hasText(user.getName())) return false;
        Set<String> allowed = listAllowedUsers();
        return allowed.contains("*") || allowed.contains(normalize(user.getName()));
    }

    private Set<String> parseConfiguredUsers() {
        String raw = appProperties.getLocalLibrary() == null ? "" : appProperties.getLocalLibrary().getAllowedUsers();
        if (!StringUtils.hasText(raw)) return Set.of();
        return java.util.Arrays.stream(raw.split(","))
                .map(LocalLibraryAccessService::normalize)
                .filter(StringUtils::hasText)
                .collect(Collectors.toSet());
    }

    public static String normalize(String userName) {
        return userName == null ? "" : userName.trim().toLowerCase(Locale.ROOT);
    }
}
