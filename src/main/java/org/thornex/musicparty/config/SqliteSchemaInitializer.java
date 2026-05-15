package org.thornex.musicparty.config;

import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;

import javax.sql.DataSource;

@RequiredArgsConstructor
@Slf4j
public class SqliteSchemaInitializer {

    private final DataSource dataSource;
    private final ResourceDatabasePopulator databasePopulator;

    @PostConstruct
    public void initialize() {
        databasePopulator.execute(dataSource);
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        if (!hasColumn(jdbcTemplate, "user_profile", "current_room_id")) {
            jdbcTemplate.execute("alter table user_profile add column current_room_id text not null default 'lounge'");
            log.info("Applied SQLite schema migration: user_profile.current_room_id");
        }
        log.info("SQLite schema initialized");
    }

    private boolean hasColumn(JdbcTemplate jdbcTemplate, String tableName, String columnName) {
        return jdbcTemplate.query("pragma table_info(" + tableName + ")",
                (rs, rowNum) -> rs.getString("name"))
                .stream()
                .anyMatch(columnName::equalsIgnoreCase);
    }
}
