package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.sqlite.SQLiteDataSource;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class SqliteAccountSchemaTests {

    @Test
    void initializerCreatesAccountAndSiteSettingTables() throws Exception {
        Path tempDir = Path.of("target", "tmp", "sqlite-account-schema-tests");
        Files.createDirectories(tempDir);
        SQLiteDataSource dataSource = new SQLiteDataSource();
        dataSource.setUrl("jdbc:sqlite:" + tempDir.resolve("account-schema.db").toAbsolutePath());

        ResourceDatabasePopulator populator = new ResourceDatabasePopulator(
                new org.springframework.core.io.ClassPathResource("db/schema.sql")
        );
        new SqliteSchemaInitializer(dataSource, populator).initialize();

        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        assertThat(tableExists(jdbcTemplate, "user_account")).isTrue();
        assertThat(columnNames(jdbcTemplate, "user_account"))
                .contains("username", "public_id", "password_hash", "role", "enabled", "last_login_at");
        assertThat(tableExists(jdbcTemplate, "site_setting")).isTrue();
        assertThat(columnNames(jdbcTemplate, "site_setting"))
                .contains("setting_key", "setting_value", "secret", "updated_at");
    }

    private List<String> columnNames(JdbcTemplate jdbcTemplate, String tableName) {
        return jdbcTemplate.query("pragma table_info(" + tableName + ")",
                (rs, rowNum) -> rs.getString("name"));
    }

    private boolean tableExists(JdbcTemplate jdbcTemplate, String tableName) {
        Integer count = jdbcTemplate.queryForObject("""
                select count(1)
                from sqlite_master
                where type = 'table' and lower(name) = lower(?)
                """, Integer.class, tableName);
        return count != null && count > 0;
    }
}
