package org.thornex.musicparty.persistence;

import java.util.Optional;

public interface UserAccountRepository {
    boolean hasAdminAccount();
    boolean usernameExists(String username);
    void create(PersistedUserAccount account);
    void updateLoginTime(String username, long lastLoginAt);
    Optional<PersistedUserAccount> findByUsername(String username);
    Optional<PersistedUserAccount> findByPublicId(String publicId);
}
