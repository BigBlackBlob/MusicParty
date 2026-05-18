package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;
import org.springframework.core.io.ByteArrayResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.sqlite.SQLiteDataSource;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class SqliteSchemaInitializerTests {

    @Test
    void initializeUpgradesLegacySchemaAndRecordsAppliedMigrations() throws Exception {
        SQLiteDataSource dataSource = createLegacyDataSource("legacy-upgrade.db");
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        seedLegacySchema(jdbcTemplate);

        createInitializer(dataSource).initialize();

        assertThat(columnNames(jdbcTemplate, "user_profile")).contains("current_room_id");
        assertThat(columnNames(jdbcTemplate, "room_playback_state")).contains("liked_user_ids_json", "like_markers_json");
        assertThat(tableExists(jdbcTemplate, "user_binding")).isTrue();
        assertThat(tableExists(jdbcTemplate, "user_playlist")).isTrue();
        assertThat(tableExists(jdbcTemplate, "user_playlist_track")).isTrue();
        assertThat(tableExists(jdbcTemplate, "subsonic_source")).isTrue();
        assertThat(tableExists(jdbcTemplate, "room_subsonic_source")).isTrue();
        assertThat(jdbcTemplate.queryForObject("select current_room_id from user_profile where public_id = 'u_legacy'", String.class))
                .isEqualTo("lounge");
        assertThat(jdbcTemplate.queryForList("select migration_key from migration_state order by migration_key", String.class))
                .containsExactly(
                        "schema.room_playback_state.like_markers_json",
                        "schema.room_playback_state.liked_user_ids_json",
                        "schema.room_subsonic_source.table",
                        "schema.subsonic_source.owner_room_id",
                        "schema.subsonic_source.table",
                        "schema.user_binding.table",
                        "schema.user_playlist.table",
                        "schema.user_playlist_track.table",
                        "schema.user_profile.current_room_id"
                );
    }

    @Test
    void initializeIsIdempotentForLegacySchemaUpgrade() throws Exception {
        SQLiteDataSource dataSource = createLegacyDataSource("legacy-idempotent.db");
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        seedLegacySchema(jdbcTemplate);
        SqliteSchemaInitializer initializer = createInitializer(dataSource);

        initializer.initialize();
        initializer.initialize();

        assertThat(jdbcTemplate.queryForObject("select count(1) from migration_state", Integer.class)).isEqualTo(9);
        assertThat(jdbcTemplate.queryForObject("select display_name from user_profile where public_id = 'u_legacy'", String.class))
                .isEqualTo("Legacy User");
    }

    private SqliteSchemaInitializer createInitializer(SQLiteDataSource dataSource) {
        ResourceDatabasePopulator populator = new ResourceDatabasePopulator(
                new org.springframework.core.io.ClassPathResource("db/schema.sql")
        );
        return new SqliteSchemaInitializer(dataSource, populator);
    }

    private SQLiteDataSource createLegacyDataSource(String fileName) throws Exception {
        Path tempDir = Path.of("target", "tmp", "sqlite-schema-initializer-tests");
        Files.createDirectories(tempDir);
        Path dbPath = tempDir.resolve(fileName);
        Files.deleteIfExists(dbPath);
        SQLiteDataSource dataSource = new SQLiteDataSource();
        dataSource.setUrl("jdbc:sqlite:" + dbPath.toAbsolutePath());
        return dataSource;
    }

    private void seedLegacySchema(JdbcTemplate jdbcTemplate) {
        ResourceDatabasePopulator legacyPopulator = new ResourceDatabasePopulator(
                new ByteArrayResource("""
                        create table user_profile (
                            public_id text primary key,
                            display_name text not null,
                            is_guest integer not null,
                            created_at integer not null,
                            last_seen_at integer not null
                        );

                        create table room_playback_state (
                            room_id text primary key,
                            current_music_json text,
                            current_enqueuer_id text,
                            current_enqueuer_name text,
                            position_anchor integer not null,
                            timestamp_anchor integer not null,
                            position_updated_at integer not null,
                            is_shuffle integer not null default 0,
                            is_paused integer not null default 0,
                            is_pause_locked integer not null default 0,
                            is_skip_locked integer not null default 0,
                            is_shuffle_locked integer not null default 0,
                            is_loading integer not null default 0,
                            play_epoch integer not null default 0,
                            state_version integer not null default 0,
                            last_persisted_at integer not null
                        );
                        """.getBytes(StandardCharsets.UTF_8))
        );
        legacyPopulator.execute(jdbcTemplate.getDataSource());
        jdbcTemplate.update("""
                insert into user_profile(public_id, display_name, is_guest, created_at, last_seen_at)
                values ('u_legacy', 'Legacy User', 0, 1, 2)
                """);
        jdbcTemplate.update("""
                insert into room_playback_state(
                    room_id, current_music_json, current_enqueuer_id, current_enqueuer_name,
                    position_anchor, timestamp_anchor, position_updated_at,
                    is_shuffle, is_paused, is_pause_locked, is_skip_locked, is_shuffle_locked,
                    is_loading, play_epoch, state_version, last_persisted_at
                )
                values ('room-1', null, null, null, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)
                """);
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
