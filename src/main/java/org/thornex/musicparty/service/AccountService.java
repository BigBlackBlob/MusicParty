package org.thornex.musicparty.service;

import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.beans.factory.annotation.Autowired;
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
    private static final String USERNAME_PATTERN = "[\\p{L}\\p{N}_\\-.]+";

    private final UserAccountRepository accountRepository;
    private final UserProfileRepository userProfileRepository;
    private final DatabaseWriteExecutor databaseWriteExecutor;
    private final BCryptPasswordEncoder passwordEncoder = new BCryptPasswordEncoder();

    public AccountService(UserAccountRepository accountRepository, UserProfileRepository userProfileRepository) {
        this(accountRepository, userProfileRepository, null);
    }

    @Autowired
    public AccountService(UserAccountRepository accountRepository,
                          UserProfileRepository userProfileRepository,
                          DatabaseWriteExecutor databaseWriteExecutor) {
        this.accountRepository = accountRepository;
        this.userProfileRepository = userProfileRepository;
        this.databaseWriteExecutor = databaseWriteExecutor;
    }

    public AuthStatus status() {
        return new AuthStatus(!accountRepository.hasAdminAccount());
    }

    public boolean hasAdminAccount() {
        return accountRepository.hasAdminAccount();
    }

    public void bootstrapAdmin(String username, String password) {
        String normalizedUsername = normalizeUsername(username);
        validatePassword(password);
        write(() -> {
            if (accountRepository.hasAdminAccount()) {
                return null;
            }
            if (!accountRepository.claimAdminBootstrap(System.currentTimeMillis())) {
                if (!accountRepository.hasAdminAccount()) {
                    throw new IllegalStateException("Administrator bootstrap is being completed by another instance");
                }
                return null;
            }
            if (accountRepository.usernameExists(normalizedUsername)) {
                throw new IllegalStateException("Bootstrap administrator username already exists");
            }
            createAccount(normalizedUsername, password, "PLATFORM_ADMIN", System.currentTimeMillis());
            return null;
        });
    }

    public AccountSession register(String username, String password) {
        String normalizedUsername = normalizeUsername(username);
        validatePassword(password);
        return write(() -> {
            if (accountRepository.usernameExists(normalizedUsername)) {
                throw new IllegalArgumentException("username already exists");
            }
            long now = System.currentTimeMillis();
            String role = accountRepository.hasAdminAccount() ? "MEMBER" : "PLATFORM_ADMIN";
            return createAccount(normalizedUsername, password, role, now);
        });
    }

    private AccountSession createAccount(String normalizedUsername, String password, String role, long now) {
        String publicId = generatePublicId();
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
        return new AccountSession(sessionToken, publicId, normalizedUsername, normalizedUsername, role, false, true, now);
    }

    public AccountSession login(String username, String password) {
        String normalizedUsername = normalizeUsername(username);
        return write(() -> loginInternal(normalizedUsername, password));
    }

    private AccountSession loginInternal(String normalizedUsername, String password) {
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

    /*
     * All account writes must use the same SQLite writer as queue/chat state.
     * Otherwise a successful password check can still fail while updating the
     * login timestamp, making the browser appear to be immediately logged out.
     */
    private <T> T write(java.util.concurrent.Callable<T> operation) {
        if (databaseWriteExecutor == null) {
            try {
                return operation.call();
            } catch (RuntimeException error) {
                throw error;
            } catch (Exception error) {
                throw new IllegalStateException("Account persistence failed", error);
            }
        }
        return databaseWriteExecutor.call(operation);
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

    public void changePassword(String sessionToken, String currentPassword, String newPassword) {
        validatePassword(newPassword);
        write(() -> {
            AccountSession session = requireSession(sessionToken);
            PersistedUserAccount account = accountRepository.findByPublicId(session.publicId())
                    .filter(PersistedUserAccount::enabled)
                    .orElseThrow(() -> new IllegalArgumentException("Unknown session token"));
            if (!passwordEncoder.matches(currentPassword == null ? "" : currentPassword, account.passwordHash())) {
                throw new IllegalArgumentException("Current password is incorrect");
            }
            accountRepository.updatePasswordHash(account.username(), passwordEncoder.encode(newPassword), System.currentTimeMillis());
            return null;
        });
    }

    public AccountSession updateProfile(String sessionToken, String displayName) {
        String normalizedDisplayName = normalizeDisplayName(displayName);
        return write(() -> {
            AccountSession session = requireSession(sessionToken);
            PersistedUserProfile profile = userProfileRepository.findByPublicId(session.publicId())
                    .orElseThrow(() -> new IllegalArgumentException("Unknown session token"));
            long now = System.currentTimeMillis();
            userProfileRepository.upsertProfile(new PersistedUserProfile(
                    profile.publicId(),
                    normalizedDisplayName,
                    false,
                    profile.currentRoomId(),
                    profile.createdAt(),
                    now
            ));
            return resolveSession(sessionToken).orElseThrow(() -> new IllegalArgumentException("Unknown session token"));
        });
    }

    public void logout(String sessionToken) {
        if (!StringUtils.hasText(sessionToken)) {
            return;
        }
        write(() -> {
            userProfileRepository.deleteSessionByHash(hashSessionToken(sessionToken));
            return null;
        });
    }

    private AccountSession toSession(PersistedUserAccount account, String sessionToken, boolean guest) {
        String displayName = userProfileRepository.findByPublicId(account.publicId())
                .map(PersistedUserProfile::displayName)
                .filter(StringUtils::hasText)
                .orElse(account.username());
        return new AccountSession(
                sessionToken,
                account.publicId(),
                account.username(),
                displayName,
                account.role(),
                guest,
                account.enabled(),
                account.lastLoginAt()
        );
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
        if (!StringUtils.hasText(normalized) || normalized.length() > 32 || !normalized.matches(USERNAME_PATTERN)) {
            throw new IllegalArgumentException("username must be 1-32 characters and contain only letters, numbers, dot, dash or underscore");
        }
        return normalized;
    }

    private void validatePassword(String password) {
        if (password == null || password.length() < MIN_PASSWORD_LENGTH) {
            throw new IllegalArgumentException("password must be at least 8 characters");
        }
    }

    private AccountSession requireSession(String sessionToken) {
        return resolveSession(sessionToken)
                .orElseThrow(() -> new IllegalArgumentException("Unknown session token"));
    }

    private String normalizeDisplayName(String displayName) {
        String normalized = displayName == null ? "" : displayName.trim();
        if (!StringUtils.hasText(normalized) || normalized.length() > 32) {
            throw new IllegalArgumentException("display name must be 1-32 characters");
        }
        return normalized;
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
