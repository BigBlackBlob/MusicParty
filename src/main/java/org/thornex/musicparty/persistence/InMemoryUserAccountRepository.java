package org.thornex.musicparty.persistence;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Repository;

import java.util.Locale;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

@Repository
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "false")
public class InMemoryUserAccountRepository implements UserAccountRepository {
    private final Map<String, PersistedUserAccount> accounts = new ConcurrentHashMap<>();
    private final java.util.concurrent.atomic.AtomicBoolean bootstrapClaimed = new java.util.concurrent.atomic.AtomicBoolean();

    @Override
    public boolean hasAdminAccount() {
        return accounts.values().stream().anyMatch(account -> "ADMIN".equals(account.role()) && account.enabled());
    }

    @Override
    public boolean claimAdminBootstrap(long claimedAt) {
        return bootstrapClaimed.compareAndSet(false, true);
    }

    @Override
    public boolean usernameExists(String username) {
        return accounts.containsKey(normalize(username));
    }

    @Override
    public void create(PersistedUserAccount account) {
        accounts.put(normalize(account.username()), account);
    }

    @Override
    public void updateLoginTime(String username, long lastLoginAt) {
        accounts.computeIfPresent(normalize(username), (key, account) -> new PersistedUserAccount(
                account.username(),
                account.publicId(),
                account.passwordHash(),
                account.role(),
                account.enabled(),
                account.createdAt(),
                lastLoginAt,
                lastLoginAt
        ));
    }

    @Override
    public void updatePasswordHash(String username, String passwordHash, long updatedAt) {
        accounts.computeIfPresent(normalize(username), (key, account) -> new PersistedUserAccount(
                account.username(),
                account.publicId(),
                passwordHash,
                account.role(),
                account.enabled(),
                account.createdAt(),
                updatedAt,
                account.lastLoginAt()
        ));
    }

    @Override
    public Optional<PersistedUserAccount> findByUsername(String username) {
        return Optional.ofNullable(accounts.get(normalize(username)));
    }

    @Override
    public Optional<PersistedUserAccount> findByPublicId(String publicId) {
        return accounts.values().stream()
                .filter(account -> account.publicId().equals(publicId))
                .findFirst();
    }

    private String normalize(String username) {
        return username == null ? "" : username.trim().toLowerCase(Locale.ROOT);
    }
}
