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
        assertThat(tableExists(jdbcTemplate, "user_account")).isTrue();
        assertThat(tableExists(jdbcTemplate, "site_setting")).isTrue();
        assertThat(tableExists(jdbcTemplate, "user_playlist")).isTrue();
        assertThat(columnNames(jdbcTemplate, "user_playlist")).contains("system_key");
        assertThat(tableExists(jdbcTemplate, "user_playlist_track")).isTrue();
        assertThat(tableExists(jdbcTemplate, "room_history_track")).isTrue();
        assertThat(tableExists(jdbcTemplate, "subsonic_source")).isTrue();
        assertThat(tableExists(jdbcTemplate, "room_subsonic_source")).isTrue();
        assertThat(tableExists(jdbcTemplate, "local_track")).isTrue();
        assertThat(columnNames(jdbcTemplate, "local_track")).contains(
                "original_hash",
                "original_file_name",
                "source_path",
                "source_mime_type",
                "source_size_bytes",
                "cover_mime_type",
                "status_message",
                "progress_percent",
                "started_at",
                "completed_at"
        );
        assertThat(tableExists(jdbcTemplate, "local_upload_access")).isTrue();
        assertThat(jdbcTemplate.queryForObject("select current_room_id from user_profile where public_id = 'u_legacy'", String.class))
                .isEqualTo("lounge");
        assertThat(jdbcTemplate.queryForList("select migration_key from migration_state order by migration_key", String.class))
                .containsExactly(
                        "schema.admin_bootstrap_claim.table",
                        "schema.local_track.original_hash_unique",
                        "schema.local_track.product_fields",
                        "schema.local_track.table",
                        "schema.local_upload_access.table",
                        "schema.room_history_track.table",
                        "schema.room_invite.table",
                        "schema.room_membership.table",
                        "schema.room_playback_state.like_markers_json",
                        "schema.room_playback_state.liked_user_ids_json",
                        "schema.room_subsonic_source.table",
                        "schema.site_setting.table",
                        "schema.subsonic_source.owner_room_id",
                        "schema.subsonic_source.table",
                        "schema.user_account.platform_admin_role",
                        "schema.user_account.table",
                        "schema.user_binding.table",
                        "schema.user_playlist.system_key",
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

        assertThat(jdbcTemplate.queryForObject("select count(1) from migration_state", Integer.class)).isEqualTo(21);
        assertThat(jdbcTemplate.queryForObject("select display_name from user_profile where public_id = 'u_legacy'", String.class))
                .isEqualTo("Legacy User");
    }

    @Test
    void initializeBackfillsRoomHistoryTrackFromExistingRoomHistory() throws Exception {
        SQLiteDataSource dataSource = createLegacyDataSource("legacy-history-backfill.db");
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        seedLegacyHistory(jdbcTemplate);

        createInitializer(dataSource).initialize();

        assertThat(jdbcTemplate.query("""
                select room_id, platform, music_id, play_count, first_played_at, last_played_at
                from room_history_track
                order by room_id, platform, music_id
                """, (rs, rowNum) -> new HistoryTrackRow(
                rs.getString("room_id"),
                rs.getString("platform"),
                rs.getString("music_id"),
                rs.getInt("play_count"),
                rs.getLong("first_played_at"),
                rs.getLong("last_played_at")
        ))).containsExactly(
                new HistoryTrackRow("room-a", "netease", "song-1", 2, 100, 300),
                new HistoryTrackRow("room-b", "netease", "song-1", 1, 200, 200)
        );
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

    private void seedLegacyHistory(JdbcTemplate jdbcTemplate) {
        ResourceDatabasePopulator legacyPopulator = new ResourceDatabasePopulator(
                new ByteArrayResource("""
                        create table room_history (
                            id text primary key,
                            room_id text not null,
                            music_json text not null,
                            enqueuer_public_id text,
                            played_at integer not null
                        );
                        """.getBytes(StandardCharsets.UTF_8))
        );
        legacyPopulator.execute(jdbcTemplate.getDataSource());
        jdbcTemplate.update("""
                insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at)
                values ('h1', 'room-a', '{"id":"song-1","name":"Old","artists":["A"],"duration":1,"platform":"netease","coverUrl":"old"}', null, 100)
                """);
        jdbcTemplate.update("""
                insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at)
                values ('h2', 'room-a', '{"id":"song-1","name":"New","artists":["A"],"duration":1,"platform":"netease","coverUrl":"new"}', null, 300)
                """);
        jdbcTemplate.update("""
                insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at)
                values ('h3', 'room-b', '{"id":"song-1","name":"Other Room","artists":["A"],"duration":1,"platform":"netease","coverUrl":"other"}', null, 200)
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

    private record HistoryTrackRow(
            String roomId,
            String platform,
            String musicId,
            int playCount,
            long firstPlayedAt,
            long lastPlayedAt
    ) {}
}
