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
                ),
                new SchemaMigration(
                        "schema.subsonic_source.table",
                        jdbc -> !hasTable(jdbc, "subsonic_source"),
                        jdbc -> jdbc.execute("""
                                create table subsonic_source (
                                    id text primary key,
                                    owner_room_id text,
                                    label text not null,
                                    base_url text not null,
                                    username text not null,
                                    password text not null,
                                    client text not null,
                                    api_version text not null,
                                    allowed_users text,
                                    enabled integer not null default 1,
                                    system integer not null default 0,
                                    created_at integer not null,
                                    updated_at integer not null
                                )
                                """)
                ),
                new SchemaMigration(
                        "schema.subsonic_source.owner_room_id",
                        jdbc -> hasTable(jdbc, "subsonic_source") && !hasColumn(jdbc, "subsonic_source", "owner_room_id"),
                        jdbc -> jdbc.execute("alter table subsonic_source add column owner_room_id text")
                ),
                new SchemaMigration(
                        "schema.room_subsonic_source.table",
                        jdbc -> !hasTable(jdbc, "room_subsonic_source"),
                        jdbc -> jdbc.execute("""
                                create table room_subsonic_source (
                                    room_id text not null,
                                    source_id text not null,
                                    enabled integer not null default 1,
                                    display_label text,
                                    allowed_users text,
                                    sort_order integer not null default 0,
                                    created_at integer not null,
                                    updated_at integer not null,
                                    primary key (room_id, source_id),
                                    foreign key (room_id) references room(id),
                                    foreign key (source_id) references subsonic_source(id)
                                )
                                """)
                ),
                new SchemaMigration(
                        "schema.user_playlist.table",
                        jdbc -> !hasTable(jdbc, "user_playlist"),
                        jdbc -> jdbc.execute("""
                                create table user_playlist (
                                    id text primary key,
                                    owner_public_id text not null,
                                    name text not null,
                                    system_key text,
                                    created_at integer not null,
                                    updated_at integer not null,
                                    unique (owner_public_id, system_key),
                                    foreign key (owner_public_id) references user_profile(public_id)
                                )
                                """)
                ),
                new SchemaMigration(
                        "schema.user_playlist.system_key",
                        jdbc -> hasTable(jdbc, "user_playlist") && !hasColumn(jdbc, "user_playlist", "system_key"),
                        jdbc -> {
                            jdbc.execute("alter table user_playlist add column system_key text");
                            jdbc.execute("create unique index if not exists idx_user_playlist_owner_system_key on user_playlist(owner_public_id, system_key)");
                        }
                ),
                new SchemaMigration(
                        "schema.user_playlist_track.table",
                        jdbc -> !hasTable(jdbc, "user_playlist_track"),
                        jdbc -> jdbc.execute("""
                                create table user_playlist_track (
                                    id text primary key,
                                    playlist_id text not null,
                                    music_json text not null,
                                    music_key text not null,
                                    sort_order integer not null,
                                    created_at integer not null,
                                    foreign key (playlist_id) references user_playlist(id),
                                    unique (playlist_id, music_key)
                                )
                                """)
                ),
                new SchemaMigration(
                        "schema.local_track.table",
                        jdbc -> !hasTable(jdbc, "local_track"),
                        jdbc -> jdbc.execute("""
                                create table local_track (
                                    id text primary key,
                                    original_hash text,
                                    original_file_name text,
                                    source_path text,
                                    source_mime_type text,
                                    source_size_bytes integer not null default 0,
                                    title text not null,
                                    artists text not null,
                                    album text,
                                    duration_ms integer not null default 0,
                                    cover_path text,
                                    cover_mime_type text,
                                    ogg_path text,
                                    status text not null,
                                    error_message text,
                                    status_message text,
                                    progress_percent integer,
                                    uploaded_by text,
                                    created_at integer not null,
                                    updated_at integer not null,
                                    started_at integer,
                                    completed_at integer
                                )
                                """)
                ),
                new SchemaMigration(
                        "schema.local_track.product_fields",
                        jdbc -> hasTable(jdbc, "local_track") && !hasColumn(jdbc, "local_track", "original_hash"),
                        jdbc -> {
                            jdbc.execute("alter table local_track add column original_hash text");
                            jdbc.execute("alter table local_track add column original_file_name text");
                            jdbc.execute("alter table local_track add column source_path text");
                            jdbc.execute("alter table local_track add column source_mime_type text");
                            jdbc.execute("alter table local_track add column source_size_bytes integer not null default 0");
                            jdbc.execute("alter table local_track add column cover_mime_type text");
                            jdbc.execute("alter table local_track add column status_message text");
                            jdbc.execute("alter table local_track add column progress_percent integer");
                            jdbc.execute("alter table local_track add column started_at integer");
                            jdbc.execute("alter table local_track add column completed_at integer");
                            jdbc.execute("update local_track set status = 'QUEUED' where status = 'PENDING'");
                            jdbc.execute("update local_track set status = 'PROCESSING' where status = 'TRANSCODING'");
                        }
                ),
                new SchemaMigration(
                        "schema.local_track.original_hash_unique",
                        jdbc -> hasTable(jdbc, "local_track"),
                        jdbc -> jdbc.execute("create unique index if not exists idx_local_track_original_hash_active on local_track(original_hash) where status <> 'DELETED' and original_hash is not null")
                ),
                new SchemaMigration(
                        "schema.local_upload_access.table",
                        jdbc -> !hasTable(jdbc, "local_upload_access"),
                        jdbc -> jdbc.execute("""
                                create table local_upload_access (
                                    user_name text primary key,
                                    created_at integer not null,
                                    updated_at integer not null
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
