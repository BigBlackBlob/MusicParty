package org.thornex.musicparty.config;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.sqlite.SQLiteDataSource;

import javax.sql.DataSource;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

@Configuration
@RequiredArgsConstructor
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

        SQLiteDataSource dataSource = new SQLiteDataSource();
        dataSource.setUrl("jdbc:sqlite:" + dbPath);
        return dataSource;
    }

    @Bean
    public JdbcTemplate jdbcTemplate(DataSource sqliteDataSource) {
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
                                                           ResourceDatabasePopulator sqliteSchemaPopulator) {
        return new SqliteSchemaInitializer(sqliteDataSource, sqliteSchemaPopulator);
    }
}
