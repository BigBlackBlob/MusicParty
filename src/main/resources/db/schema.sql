create table if not exists room (
    id text primary key,
    name text not null,
    owner_public_id text not null,
    visibility text not null,
    password_hash text,
    password_version integer not null default 0,
    system integer not null default 0,
    created_at integer not null,
    last_active_at integer not null,
    deleted_at integer
);

create index if not exists idx_room_last_active on room(last_active_at desc);
create index if not exists idx_room_deleted_at on room(deleted_at);

create table if not exists user_profile (
    public_id text primary key,
    display_name text not null,
    is_guest integer not null,
    current_room_id text not null default 'lounge',
    created_at integer not null,
    last_seen_at integer not null
);

create table if not exists user_session (
    session_token_hash text primary key,
    public_id text not null,
    created_at integer not null,
    last_seen_at integer not null,
    foreign key (public_id) references user_profile(public_id)
);

create index if not exists idx_user_session_public_id on user_session(public_id);

create table if not exists user_binding (
    public_id text not null,
    platform text not null,
    account_id text not null,
    primary key (public_id, platform),
    foreign key (public_id) references user_profile(public_id)
);

create table if not exists room_queue (
    id text primary key,
    room_id text not null,
    music_json text not null,
    enqueuer_public_id text not null,
    enqueuer_name_snapshot text not null,
    status text not null,
    sort_order integer not null,
    created_at integer not null,
    foreign key (room_id) references room(id)
);

create index if not exists idx_room_queue_room_sort on room_queue(room_id, sort_order);

create table if not exists room_playlist (
    id text primary key,
    room_id text not null,
    name text not null,
    created_at integer not null,
    updated_at integer not null,
    foreign key (room_id) references room(id)
);

create index if not exists idx_room_playlist_room on room_playlist(room_id, created_at);

create table if not exists room_playlist_track (
    id text primary key,
    playlist_id text not null,
    music_json text not null,
    sort_order integer not null,
    created_at integer not null,
    foreign key (playlist_id) references room_playlist(id)
);

create index if not exists idx_room_playlist_track_sort on room_playlist_track(playlist_id, sort_order);

create table if not exists user_playlist (
    id text primary key,
    owner_public_id text not null,
    name text not null,
    created_at integer not null,
    updated_at integer not null,
    foreign key (owner_public_id) references user_profile(public_id)
);

create index if not exists idx_user_playlist_owner on user_playlist(owner_public_id, created_at);

create table if not exists user_playlist_track (
    id text primary key,
    playlist_id text not null,
    music_json text not null,
    music_key text not null,
    sort_order integer not null,
    created_at integer not null,
    foreign key (playlist_id) references user_playlist(id),
    unique (playlist_id, music_key)
);

create index if not exists idx_user_playlist_track_sort on user_playlist_track(playlist_id, sort_order);

create table if not exists room_history (
    id text primary key,
    room_id text not null,
    music_json text not null,
    enqueuer_public_id text,
    played_at integer not null,
    foreign key (room_id) references room(id)
);

create index if not exists idx_room_history_room_played on room_history(room_id, played_at desc);

create table if not exists chat_message (
    id text primary key,
    room_id text,
    user_id text not null,
    user_name text not null,
    content text not null,
    type text not null,
    created_at integer not null,
    foreign key (room_id) references room(id)
);

create index if not exists idx_chat_message_room_created on chat_message(room_id, created_at desc);

create table if not exists room_playback_state (
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
    liked_user_ids_json text,
    like_markers_json text,
    play_epoch integer not null default 0,
    state_version integer not null default 0,
    last_persisted_at integer not null,
    foreign key (room_id) references room(id)
);

create table if not exists migration_state (
    migration_key text primary key,
    completed_at integer not null
);

create table if not exists subsonic_source (
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
);

create index if not exists idx_subsonic_source_enabled on subsonic_source(enabled);
create index if not exists idx_subsonic_source_owner_room on subsonic_source(owner_room_id);

create table if not exists room_subsonic_source (
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
);

create index if not exists idx_room_subsonic_source_source on room_subsonic_source(source_id);
