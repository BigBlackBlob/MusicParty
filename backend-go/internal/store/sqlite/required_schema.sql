-- Generated from the frozen Java SQLite initializer. Do not edit by hand.
PRAGMA foreign_keys = ON;

CREATE TABLE admin_bootstrap_claim (
    claim_key text primary key,
    claimed_at integer not null
);

CREATE TABLE chat_message ( id text primary key, room_id text, user_id text not null, user_name text not null, content text not null, type text not null, created_at integer not null, foreign key (room_id) references room(id) );

CREATE TABLE local_track ( id text primary key, original_hash text, original_file_name text, source_path text, source_mime_type text, source_size_bytes integer not null default 0, title text not null, artists text not null, album text, duration_ms integer not null default 0, cover_path text, cover_mime_type text, ogg_path text, status text not null, error_message text, status_message text, progress_percent integer, uploaded_by text, created_at integer not null, updated_at integer not null, started_at integer, completed_at integer );

CREATE TABLE local_upload_access ( user_name text primary key, created_at integer not null, updated_at integer not null );

CREATE TABLE migration_state ( migration_key text primary key, completed_at integer not null );

CREATE TABLE room ( id text primary key, name text not null, owner_public_id text not null, visibility text not null, password_hash text, password_version integer not null default 0, system integer not null default 0, created_at integer not null, last_active_at integer not null, deleted_at integer );

CREATE TABLE room_history ( id text primary key, room_id text not null, music_json text not null, enqueuer_public_id text, played_at integer not null, foreign key (room_id) references room(id) );

CREATE TABLE room_history_track ( room_id text not null, platform text not null, music_id text not null, music_json text not null, play_count integer not null, first_played_at integer not null, last_played_at integer not null, primary key (room_id, platform, music_id), foreign key (room_id) references room(id) );

CREATE TABLE room_invite ( id text primary key, room_id text not null, created_by_public_id text not null, secret_hash text not null unique, label text, expires_at integer not null, max_uses integer not null default 1, used_at integer, used_by_public_id text, revoked_at integer, created_at integer not null, foreign key (room_id) references room(id) );

CREATE TABLE room_membership ( room_id text not null, public_id text not null, role text not null, created_at integer not null, updated_at integer not null, primary key (room_id, public_id), foreign key (room_id) references room(id), foreign key (public_id) references user_profile(public_id) );

CREATE TABLE room_playback_state ( room_id text primary key, current_music_json text, current_enqueuer_id text, current_enqueuer_name text, position_anchor integer not null, timestamp_anchor integer not null, position_updated_at integer not null, is_shuffle integer not null default 0, is_paused integer not null default 0, is_pause_locked integer not null default 0, is_skip_locked integer not null default 0, is_shuffle_locked integer not null default 0, is_loading integer not null default 0, liked_user_ids_json text, like_markers_json text, play_epoch integer not null default 0, state_version integer not null default 0, last_persisted_at integer not null, foreign key (room_id) references room(id) );

CREATE TABLE room_playlist ( id text primary key, room_id text not null, name text not null, created_at integer not null, updated_at integer not null, foreign key (room_id) references room(id) );

CREATE TABLE room_playlist_track ( id text primary key, playlist_id text not null, music_json text not null, sort_order integer not null, created_at integer not null, foreign key (playlist_id) references room_playlist(id) );

CREATE TABLE room_queue ( id text primary key, room_id text not null, music_json text not null, enqueuer_public_id text not null, enqueuer_name_snapshot text not null, status text not null, sort_order integer not null, created_at integer not null, foreign key (room_id) references room(id) );

CREATE TABLE room_subsonic_source ( room_id text not null, source_id text not null, enabled integer not null default 1, display_label text, allowed_users text, sort_order integer not null default 0, created_at integer not null, updated_at integer not null, primary key (room_id, source_id), foreign key (room_id) references room(id), foreign key (source_id) references subsonic_source(id) );

CREATE TABLE site_setting ( setting_key text primary key, setting_value text, secret integer not null default 0, updated_at integer not null );

CREATE TABLE subsonic_source ( id text primary key, owner_room_id text, label text not null, base_url text not null, username text not null, password text not null, client text not null, api_version text not null, allowed_users text, enabled integer not null default 1, system integer not null default 0, created_at integer not null, updated_at integer not null );

CREATE TABLE user_account ( username text primary key, public_id text not null unique, password_hash text not null, role text not null, enabled integer not null default 1, created_at integer not null, updated_at integer not null, last_login_at integer, foreign key (public_id) references user_profile(public_id) );

CREATE TABLE user_binding ( public_id text not null, platform text not null, account_id text not null, primary key (public_id, platform), foreign key (public_id) references user_profile(public_id) );

CREATE TABLE user_playlist ( id text primary key, owner_public_id text not null, name text not null, system_key text, created_at integer not null, updated_at integer not null, unique (owner_public_id, system_key), foreign key (owner_public_id) references user_profile(public_id) );

CREATE TABLE user_playlist_track ( id text primary key, playlist_id text not null, music_json text not null, music_key text not null, sort_order integer not null, created_at integer not null, foreign key (playlist_id) references user_playlist(id), unique (playlist_id, music_key) );

CREATE TABLE user_profile ( public_id text primary key, display_name text not null, is_guest integer not null, current_room_id text not null default 'lounge', created_at integer not null, last_seen_at integer not null );

CREATE TABLE user_session ( session_token_hash text primary key, public_id text not null, created_at integer not null, last_seen_at integer not null, foreign key (public_id) references user_profile(public_id) );

CREATE INDEX idx_chat_message_room_created on chat_message(room_id, created_at desc);

CREATE UNIQUE INDEX idx_local_track_original_hash_active on local_track(original_hash) where status <> 'DELETED' and original_hash is not null;

CREATE INDEX idx_local_track_status on local_track(status);

CREATE INDEX idx_local_track_updated on local_track(updated_at desc);

CREATE INDEX idx_room_deleted_at on room(deleted_at);

CREATE INDEX idx_room_history_room_played on room_history(room_id, played_at desc);

CREATE INDEX idx_room_history_track_room_last on room_history_track(room_id, last_played_at desc);

CREATE INDEX idx_room_invite_room on room_invite(room_id, created_at desc);

CREATE INDEX idx_room_last_active on room(last_active_at desc);

CREATE INDEX idx_room_membership_public on room_membership(public_id);

CREATE INDEX idx_room_playlist_room on room_playlist(room_id, created_at);

CREATE INDEX idx_room_playlist_track_sort on room_playlist_track(playlist_id, sort_order);

CREATE INDEX idx_room_queue_room_sort on room_queue(room_id, sort_order);

CREATE INDEX idx_room_subsonic_source_source on room_subsonic_source(source_id);

CREATE INDEX idx_subsonic_source_enabled on subsonic_source(enabled);

CREATE INDEX idx_subsonic_source_owner_room on subsonic_source(owner_room_id);

CREATE INDEX idx_user_account_public_id on user_account(public_id);

CREATE INDEX idx_user_account_role on user_account(role);

CREATE INDEX idx_user_playlist_owner on user_playlist(owner_public_id, created_at);

CREATE INDEX idx_user_playlist_track_sort on user_playlist_track(playlist_id, sort_order);

CREATE INDEX idx_user_session_public_id on user_session(public_id);

CREATE UNIQUE INDEX uq_room_queue_room_id on room_queue(room_id, id);

