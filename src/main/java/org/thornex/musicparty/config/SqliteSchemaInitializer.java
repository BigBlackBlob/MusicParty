package org.thornex.musicparty.config;

import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;

import javax.sql.DataSource;
import java.util.List;

@RequiredArgsConstructor
@Slf4j
public class SqliteSchemaInitializer {

    private final DataSource dataSource;
    private final ResourceDatabasePopulator databasePopulator;

    @PostConstruct
    public void initialize() {
        databasePopulator.execute(dataSource);
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        migrations().forEach(migration -> applyMigration(jdbcTemplate, migration));
        log.info("SQLite schema initialized");
    }

    private void applyMigration(JdbcTemplate jdbcTemplate, SchemaMigration migration) {
        if (isMigrationCompleted(jdbcTemplate, migration.key())) {
            return;
        }
        if (migration.needsApply().test(jdbcTemplate)) {
            migration.apply().accept(jdbcTemplate);
            log.info("Applied SQLite schema migration: {}", migration.key());
        }
        markMigrationCompleted(jdbcTemplate, migration.key());
    }

    private List<SchemaMigration> migrations() {
        return List.of(
                new SchemaMigration(
                        "schema.user_profile.current_room_id",
                        jdbc -> !hasColumn(jdbc, "user_profile", "current_room_id"),
                        jdbc -> jdbc.execute("alter table user_profile add column current_room_id text not null default 'lounge'")
                ),
                new SchemaMigration(
                        "schema.room_playback_state.liked_user_ids_json",
                        jdbc -> !hasColumn(jdbc, "room_playback_state", "liked_user_ids_json"),
                        jdbc -> jdbc.execute("alter table room_playback_state add column liked_user_ids_json text")
                ),
                new SchemaMigration(
                        "schema.room_playback_state.like_markers_json",
                        jdbc -> !hasColumn(jdbc, "room_playback_state", "like_markers_json"),
                        jdbc -> jdbc.execute("alter table room_playback_state add column like_markers_json text")
                ),
                new SchemaMigration(
                        "schema.user_binding.table",
                        jdbc -> !hasTable(jdbc, "user_binding"),
                        jdbc -> jdbc.execute("""
                                create table user_binding (
                                    public_id text not null,
                                    platform text not null,
                                    account_id text not null,
                                    primary key (public_id, platform),
                                    foreign key (public_id) references user_profile(public_id)
                                )
                                """)
                )
        );
    }

    private boolean hasColumn(JdbcTemplate jdbcTemplate, String tableName, String columnName) {
        return jdbcTemplate.query("pragma table_info(" + tableName + ")",
                (rs, rowNum) -> rs.getString("name"))
                .stream()
                .anyMatch(columnName::equalsIgnoreCase);
    }

    private boolean hasTable(JdbcTemplate jdbcTemplate, String tableName) {
        Integer count = jdbcTemplate.queryForObject("""
                select count(1)
                from sqlite_master
                where type = 'table' and lower(name) = lower(?)
                """, Integer.class, tableName);
        return count != null && count > 0;
    }

    private boolean isMigrationCompleted(JdbcTemplate jdbcTemplate, String migrationKey) {
        Integer count = jdbcTemplate.queryForObject("""
                select count(1)
                from migration_state
                where migration_key = ? and completed_at is not null
                """, Integer.class, migrationKey);
        return count != null && count > 0;
    }

    private void markMigrationCompleted(JdbcTemplate jdbcTemplate, String migrationKey) {
        jdbcTemplate.update("""
                insert into migration_state(migration_key, completed_at)
                values (?, ?)
                on conflict(migration_key) do update set completed_at = excluded.completed_at
                """, migrationKey, System.currentTimeMillis());
    }

    private record SchemaMigration(
            String key,
            java.util.function.Predicate<JdbcTemplate> needsApply,
            java.util.function.Consumer<JdbcTemplate> apply
    ) {
    }
}
