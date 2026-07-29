package org.thornex.musicparty.config;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.springframework.transaction.support.TransactionTemplate;
import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;
import org.sqlite.SQLiteConfig;

import javax.sql.DataSource;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

import lombok.extern.slf4j.Slf4j;

@Configuration
@RequiredArgsConstructor
@Slf4j
@ConditionalOnProperty(prefix = "app.music-api.database", name = "enabled", havingValue = "true", matchIfMissing = true)
public class SqlitePersistenceConfig {

    private final AppProperties appProperties;

    @Bean
    public DataSource sqliteDataSource() throws IOException {
        Path dbPath = Path.of(appProperties.getDatabase().getPath()).toAbsolutePath().normalize();
        Path parent = dbPath.getParent();
        if (parent != null) {
            Files.createDirectories(parent);
        }

        SQLiteConfig config = new SQLiteConfig();
        config.setJournalMode(SQLiteConfig.JournalMode.WAL);
        config.setSynchronous(SQLiteConfig.SynchronousMode.NORMAL);
        config.setBusyTimeout((int) appProperties.getDatabase().getBusyTimeoutMs());
        config.enforceForeignKeys(true);

        HikariConfig hikari = new HikariConfig();
        hikari.setJdbcUrl("jdbc:sqlite:" + dbPath);
        hikari.setDataSourceProperties(config.toProperties());
        if (appProperties.getDatabase().getMaxPoolSize() > 1) {
            log.warn("Ignoring DB_MAX_POOL_SIZE={} for SQLite; using one connection to serialize writes",
                    appProperties.getDatabase().getMaxPoolSize());
        }
        hikari.setMaximumPoolSize(1);
        hikari.setMinimumIdle(1);
        hikari.setConnectionTimeout(appProperties.getDatabase().getConnectionTimeoutMs());
        hikari.setPoolName("musicparty-sqlite");
        return new HikariDataSource(hikari);
    }

    @Bean
    public JdbcTemplate jdbcTemplate(DataSource sqliteDataSource, SqliteSchemaInitializer initializer) {
        return new JdbcTemplate(sqliteDataSource);
    }

    @Bean
    public DataSourceTransactionManager transactionManager(DataSource sqliteDataSource) {
        return new DataSourceTransactionManager(sqliteDataSource);
    }

    @Bean
    public ResourceDatabasePopulator sqliteSchemaPopulator() {
        ResourceDatabasePopulator populator = new ResourceDatabasePopulator();
        if (appProperties.getDatabase().isInitSchema()) {
            populator.addScript(new ClassPathResource("db/schema.sql"));
        }
        populator.setContinueOnError(false);
        populator.setIgnoreFailedDrops(true);
        return populator;
    }

    @Bean
    public SqliteSchemaInitializer sqliteSchemaInitializer(DataSource sqliteDataSource,
                                                            ResourceDatabasePopulator sqliteSchemaPopulator,
                                                            DataSourceTransactionManager transactionManager) {
        return new SqliteSchemaInitializer(sqliteDataSource, sqliteSchemaPopulator, new TransactionTemplate(transactionManager));
    }
}
