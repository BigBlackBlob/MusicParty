package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcUserAccountRepository implements UserAccountRepository {
    private static final RowMapper<PersistedUserAccount> ROW_MAPPER = (rs, rowNum) -> new PersistedUserAccount(
            rs.getString("username"),
            rs.getString("public_id"),
            rs.getString("password_hash"),
            rs.getString("role"),
            rs.getBoolean("enabled"),
            rs.getLong("created_at"),
            rs.getLong("updated_at"),
            rs.getObject("last_login_at") == null ? null : rs.getLong("last_login_at")
    );

    private final JdbcTemplate jdbcTemplate;

    @Override
    public boolean hasAdminAccount() {
        Integer count = jdbcTemplate.queryForObject("""
                select count(1)
                from user_account
                where role = 'ADMIN' and enabled = 1
                """, Integer.class);
        return count != null && count > 0;
    }

    @Override
    public boolean usernameExists(String username) {
        Integer count = jdbcTemplate.queryForObject("""
                select count(1)
                from user_account
                where username = ?
                """, Integer.class, username);
        return count != null && count > 0;
    }

    @Override
    public void create(PersistedUserAccount account) {
        jdbcTemplate.update("""
                insert into user_account(username, public_id, password_hash, role, enabled, created_at, updated_at, last_login_at)
                values (?, ?, ?, ?, ?, ?, ?, ?)
                """,
                account.username(),
                account.publicId(),
                account.passwordHash(),
                account.role(),
                account.enabled(),
                account.createdAt(),
                account.updatedAt(),
                account.lastLoginAt());
    }

    @Override
    public void updateLoginTime(String username, long lastLoginAt) {
        jdbcTemplate.update("""
                update user_account
                set last_login_at = ?, updated_at = ?
                where username = ?
                """, lastLoginAt, lastLoginAt, username);
    }

    @Override
    public Optional<PersistedUserAccount> findByUsername(String username) {
        List<PersistedUserAccount> rows = jdbcTemplate.query("""
                select username, public_id, password_hash, role, enabled, created_at, updated_at, last_login_at
                from user_account
                where username = ?
                """, ROW_MAPPER, username);
        return rows.stream().findFirst();
    }

    @Override
    public Optional<PersistedUserAccount> findByPublicId(String publicId) {
        List<PersistedUserAccount> rows = jdbcTemplate.query("""
                select username, public_id, password_hash, role, enabled, created_at, updated_at, last_login_at
                from user_account
                where public_id = ?
                """, ROW_MAPPER, publicId);
        return rows.stream().findFirst();
    }
}
