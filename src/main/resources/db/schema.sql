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
