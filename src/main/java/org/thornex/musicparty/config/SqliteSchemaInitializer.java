package org.thornex.musicparty.config;

import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
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
        log.info("SQLite schema initialized");
    }
}
