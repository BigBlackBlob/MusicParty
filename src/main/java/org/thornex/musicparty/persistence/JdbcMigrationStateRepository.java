package org.thornex.musicparty.persistence;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnBean;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

@Repository
@RequiredArgsConstructor
@ConditionalOnBean(JdbcTemplate.class)
public class JdbcMigrationStateRepository implements MigrationStateRepository {

    private final JdbcTemplate jdbcTemplate;

    @Override
    public boolean isCompleted(String migrationKey) {
        Integer count = jdbcTemplate.queryForObject(
                "select count(1) from migration_state where migration_key = ? and completed_at is not null",
                Integer.class,
                migrationKey
        );
        return count != null && count > 0;
    }

    @Override
    public void markCompleted(String migrationKey) {
        jdbcTemplate.update("""
                insert into migration_state(migration_key, completed_at)
                values (?, ?)
                on conflict(migration_key) do update set completed_at = excluded.completed_at
                """,
                migrationKey,
                System.currentTimeMillis()
        );
    }
}
