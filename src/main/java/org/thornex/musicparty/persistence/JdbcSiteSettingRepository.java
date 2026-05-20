package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class JdbcSiteSettingRepository implements SiteSettingRepository {
    private final JdbcTemplate jdbcTemplate;

    @Override
    public Optional<String> findValue(String key) {
        List<String> rows = jdbcTemplate.query("""
                select setting_value
                from site_setting
                where setting_key = ?
                """, (rs, rowNum) -> rs.getString("setting_value"), key);
        return rows.stream().findFirst();
    }

    @Override
    public void upsert(String key, String value, boolean secret, long updatedAt) {
        jdbcTemplate.update("""
                insert into site_setting(setting_key, setting_value, secret, updated_at)
                values (?, ?, ?, ?)
                on conflict(setting_key) do update set
                  setting_value = excluded.setting_value,
                  secret = excluded.secret,
                  updated_at = excluded.updated_at
                """, key, value, secret, updatedAt);
    }
}
