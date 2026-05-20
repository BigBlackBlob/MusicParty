package org.thornex.musicparty.service;

import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.persistence.PersistedSession;
import org.thornex.musicparty.persistence.PersistedUserAccount;
import org.thornex.musicparty.persistence.PersistedUserProfile;
import org.thornex.musicparty.persistence.UserAccountRepository;
import org.thornex.musicparty.persistence.UserProfileRepository;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Locale;
import java.util.Optional;
import java.util.UUID;

@Service
public class AccountService {
    private static final int MIN_PASSWORD_LENGTH = 8;

    private final UserAccountRepository accountRepository;
    private final UserProfileRepository userProfileRepository;
    private final BCryptPasswordEncoder passwordEncoder = new BCryptPasswordEncoder();

    public AccountService(UserAccountRepository accountRepository, UserProfileRepository userProfileRepository) {
        this.accountRepository = accountRepository;
        this.userProfileRepository = userProfileRepository;
    }

    public AuthStatus status() {
        return new AuthStatus(!accountRepository.hasAdminAccount());
    }

    public AccountSession register(String username, String password) {
        String normalizedUsername = normalizeUsername(username);
        validatePassword(password);
        if (accountRepository.usernameExists(normalizedUsername)) {
            throw new IllegalArgumentException("username already exists");
        }
        long now = System.currentTimeMillis();
        String publicId = generatePublicId();
        String role = accountRepository.hasAdminAccount() ? "USER" : "ADMIN";
        userProfileRepository.upsertProfile(new PersistedUserProfile(
                publicId,
                normalizedUsername,
                false,
                RoomService.DEFAULT_ROOM_ID,
                now,
                now
        ));
        accountRepository.create(new PersistedUserAccount(
                normalizedUsername,
                publicId,
                passwordEncoder.encode(password),
                role,
                true,
                now,
                now,
                now
        ));
        String sessionToken = issueSession(publicId, now);
        return new AccountSession(sessionToken, publicId, normalizedUsername, role, false);
    }

    public AccountSession login(String username, String password) {
        String normalizedUsername = normalizeUsername(username);
        PersistedUserAccount account = accountRepository.findByUsername(normalizedUsername)
                .filter(PersistedUserAccount::enabled)
                .filter(candidate -> passwordEncoder.matches(password == null ? "" : password, candidate.passwordHash()))
                .orElseThrow(() -> new IllegalArgumentException("Invalid username or password"));
        long now = System.currentTimeMillis();
        accountRepository.updateLoginTime(account.username(), now);
        userProfileRepository.findByPublicId(account.publicId()).ifPresent(profile ->
                userProfileRepository.upsertProfile(new PersistedUserProfile(
                        profile.publicId(),
                        profile.displayName(),
                        false,
                        profile.currentRoomId(),
                        profile.createdAt(),
                        now
                ))
        );
        String sessionToken = issueSession(account.publicId(), now);
        return toSession(account, sessionToken, false);
    }

    public Optional<AccountSession> resolveSession(String sessionToken) {
        if (!StringUtils.hasText(sessionToken)) {
            return Optional.empty();
        }
        return userProfileRepository.findSessionByHash(hashSessionToken(sessionToken))
                .flatMap(session -> accountRepository.findByPublicId(session.publicId()))
                .filter(PersistedUserAccount::enabled)
                .map(account -> toSession(account, sessionToken, false));
    }

    public boolean isAdminSession(String sessionToken) {
        return resolveSession(sessionToken).map(AccountSession::admin).orElse(false);
    }

    public Optional<String> roleForPublicId(String publicId) {
        if (!StringUtils.hasText(publicId)) {
            return Optional.empty();
        }
        return accountRepository.findByPublicId(publicId)
                .filter(PersistedUserAccount::enabled)
                .map(PersistedUserAccount::role);
    }

    public Optional<AccountSession> resolvePublicId(String publicId) {
        if (!StringUtils.hasText(publicId)) {
            return Optional.empty();
        }
        return accountRepository.findByPublicId(publicId)
                .filter(PersistedUserAccount::enabled)
                .map(account -> toSession(account, "", false));
    }

    private AccountSession toSession(PersistedUserAccount account, String sessionToken, boolean guest) {
        return new AccountSession(sessionToken, account.publicId(), account.username(), account.role(), guest);
    }

    private String issueSession(String publicId, long now) {
        String sessionToken = UUID.randomUUID().toString();
        userProfileRepository.upsertSession(new PersistedSession(hashSessionToken(sessionToken), publicId, now, now));
        return sessionToken;
    }

    private String generatePublicId() {
        String publicId;
        do {
            publicId = "u_" + UUID.randomUUID().toString().replace("-", "").substring(0, 12);
        } while (userProfileRepository.findByPublicId(publicId).isPresent());
        return publicId;
    }

    private String normalizeUsername(String username) {
        String normalized = username == null ? "" : username.trim().toLowerCase(Locale.ROOT);
        if (!StringUtils.hasText(normalized) || normalized.length() > 32 || !normalized.matches("[a-z0-9_\\-.]+")) {
            throw new IllegalArgumentException("username must be 1-32 characters and contain only letters, numbers, dot, dash or underscore");
        }
        return normalized;
    }

    private void validatePassword(String password) {
        if (password == null || password.length() < MIN_PASSWORD_LENGTH) {
            throw new IllegalArgumentException("password must be at least 8 characters");
        }
    }

    private String hashSessionToken(String sessionToken) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            byte[] bytes = digest.digest(sessionToken.getBytes(StandardCharsets.UTF_8));
            StringBuilder builder = new StringBuilder(bytes.length * 2);
            for (byte value : bytes) {
                builder.append(String.format("%02x", value));
            }
            return builder.toString();
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is unavailable", e);
        }
    }
}
