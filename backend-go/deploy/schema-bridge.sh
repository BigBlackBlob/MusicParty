#!/usr/bin/env sh
set -eu

APPROVED_GO_IMAGE='ghcr.io/bigblackblob/musicparty-go@sha256:f16d4a26b94d28ca42e47c0d1bee8043f5f2434d2a982f375290ef49c390e759'
APPROVED_JAVA_BRIDGE_IMAGE='ghcr.io/bigblackblob/musicparty@sha256:1f08392fa699bcb655e3fd2a6eb2347432378804412b2e5521f955e48cf299e2'
BRIDGE_CONTAINER=''
BRIDGE_TOKEN=''

usage() {
  cat <<'EOF'
Usage: schema-bridge.sh preflight|apply|verify [snapshot-path]

Required environment:
  MUSIC_PARTY_IMAGE
  MUSIC_PARTY_JAVA_BRIDGE_IMAGE
  MUSIC_PARTY_ORIGINAL_JAVA_IMAGE
  MUSIC_PARTY_RUNTIME_UID
  MUSIC_PARTY_RUNTIME_GID

Optional environment:
  DATA_DIR           Host data directory (default: ./music_party/data)
  COMPOSE_FILE       Base compose file (default: ./docker-compose.yml)
  GO_COMPOSE_FILE    Go override (default: ./backend-go/deploy/compose.go.yml)
  CONTAINER_NAME     Application container (default: music-party-app)
  BACKUP_DIR         Snapshot directory (default: ./music_party/backups/go-cutover)

apply also requires MUSICPARTY_MAINTENANCE_CONFIRMED=YES and a stopped
application container. It never starts, stops, or restores a Compose service.
EOF
}

fail() {
  printf '%s\n' "error: $*" >&2
  exit 1
}

env_value() {
  printenv "$1" 2>/dev/null || true
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

absolute_dir() {
  [ -d "$1" ] || fail "directory does not exist"
  case $(uname -s) in
    MINGW*|MSYS*) (cd "$1" && pwd -W) ;;
    *) (cd "$1" && pwd -P) ;;
  esac
}

docker_cmd() {
  case $(uname -s) in
    MINGW*|MSYS*) MSYS_NO_PATHCONV=1 docker "$@" ;;
    *) docker "$@" ;;
  esac
}

require_image() {
  name=$1
  value=$(env_value "$name")
  case $value in *@sha256:*) ;; *) fail "$name must use an immutable @sha256: digest" ;; esac
  digest=$(printf '%s' "$value" | sed 's/.*@sha256://')
  [ "$(printf '%s' "$digest" | wc -c | tr -d ' ')" -eq 64 ] ||
    fail "$name must use a full SHA-256 digest"
  case $digest in *[!0123456789abcdef]*) fail "$name must use a lowercase hexadecimal SHA-256 digest" ;; esac
  printf '%s\n' "$value"
}

require_images() {
  MUSIC_PARTY_IMAGE=$(require_image MUSIC_PARTY_IMAGE)
  MUSIC_PARTY_JAVA_BRIDGE_IMAGE=$(require_image MUSIC_PARTY_JAVA_BRIDGE_IMAGE)
  MUSIC_PARTY_ORIGINAL_JAVA_IMAGE=$(require_image MUSIC_PARTY_ORIGINAL_JAVA_IMAGE)
  [ "$MUSIC_PARTY_IMAGE" = "$APPROVED_GO_IMAGE" ] ||
    fail "MUSIC_PARTY_IMAGE must equal the approved Go candidate digest"
  [ "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" = "$APPROVED_JAVA_BRIDGE_IMAGE" ] ||
    fail "MUSIC_PARTY_JAVA_BRIDGE_IMAGE must equal the approved Java bridge candidate digest"
}

require_runtime_ids() {
  MUSIC_PARTY_RUNTIME_UID=$(env_value MUSIC_PARTY_RUNTIME_UID)
  MUSIC_PARTY_RUNTIME_GID=$(env_value MUSIC_PARTY_RUNTIME_GID)
  case $MUSIC_PARTY_RUNTIME_UID in ''|*[!0-9]*) fail "MUSIC_PARTY_RUNTIME_UID must be a numeric id" ;; esac
  case $MUSIC_PARTY_RUNTIME_GID in ''|*[!0-9]*) fail "MUSIC_PARTY_RUNTIME_GID must be a numeric id" ;; esac
}

require_local_images() {
  docker_cmd image inspect "$MUSIC_PARTY_IMAGE" >/dev/null 2>&1 ||
    fail "approved Go image is not present locally; pull and inspect it first"
  docker_cmd image inspect "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" >/dev/null 2>&1 ||
    fail "approved Java bridge image is not present locally; pull and inspect it first"
  docker_cmd image inspect "$MUSIC_PARTY_ORIGINAL_JAVA_IMAGE" >/dev/null 2>&1 ||
    fail "original Java image is not present locally; pull and inspect it first"
}

require_original_java_container() {
  current=$(docker_cmd inspect --format '{{.Image}}' "$CONTAINER_NAME" 2>/dev/null) ||
    fail "application container cannot be inspected"
  recorded=$(docker_cmd image inspect --format '{{.Id}}' "$MUSIC_PARTY_ORIGINAL_JAVA_IMAGE" 2>/dev/null) ||
    fail "original Java image cannot be inspected"
  [ "$current" = "$recorded" ] ||
    fail "application container does not match MUSIC_PARTY_ORIGINAL_JAVA_IMAGE"
}

python_table_count() {
  access_mode=$1
  source_dir=$2
  source_name=$3
  mount="type=bind,src=$source_dir,dst=/db"
  case $access_mode in
    live) ;;
    snapshot) mount="$mount,readonly" ;;
    *) fail "invalid SQLite access mode" ;;
  esac
  docker_cmd run --pull=never --rm --interactive --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "$mount" --entrypoint python3 "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" - "$source_name" "$access_mode" 2>/dev/null <<'PY'
import sqlite3, sys
from urllib.parse import quote
suffix = "?mode=ro&immutable=1" if sys.argv[2] == "snapshot" else "?mode=ro"
db = sqlite3.connect("file:/db/" + quote(sys.argv[1]) + suffix, uri=True)
try:
    db.execute("pragma query_only=on")
    print(db.execute("select count(*) from sqlite_master where type = ? and name not like ?", ("table", "sqlite_%")).fetchone()[0])
finally:
    db.close()
PY
}

table_count_or_fail() {
  if ! count=$(python_table_count "$1" "$2" "$3"); then
    fail "cannot count application tables"
  fi
  case $count in ''|*[!0-9]*) fail "application table count was not numeric" ;; esac
  printf '%s\n' "$count"
}

require_table_count() {
  actual=$(table_count_or_fail "$2" "$3" "$4")
  [ "$actual" = "$1" ] || fail "expected $1 application tables, found $actual"
}

prebridge_shape() {
  access_mode=$1
  source_dir=$2
  source_name=$3
  mount="type=bind,src=$source_dir,dst=/db"
  case $access_mode in
    live) ;;
    snapshot) mount="$mount,readonly" ;;
    *) fail "invalid SQLite access mode" ;;
  esac
  docker_cmd run --pull=never --rm --interactive --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "$mount" --entrypoint python3 "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" - "$source_name" "$access_mode" 2>/dev/null <<'PY'
import sqlite3, sys
from urllib.parse import quote
suffix = "?mode=ro&immutable=1" if sys.argv[2] == "snapshot" else "?mode=ro"
db = sqlite3.connect("file:/db/" + quote(sys.argv[1]) + suffix, uri=True)
try:
    db.execute("pragma query_only=on")
    tables = {row[0] for row in db.execute("select name from sqlite_master where type = 'table' and name not like 'sqlite_%'")}
    expected = {
        "admin_bootstrap_claim", "chat_message", "local_track", "local_upload_access",
        "migration_state", "room", "room_history", "room_history_track", "room_invite",
        "room_membership", "room_playback_state", "room_playlist", "room_playlist_track",
        "room_queue", "room_subsonic_source", "site_setting", "subsonic_source",
        "user_account", "user_binding", "user_playlist", "user_playlist_track",
        "user_profile", "user_session",
    }
    missing = expected - tables
    expected_migration_keys = {
        "legacy.queue-data.json",
        "legacy.rooms.json",
        "schema.admin_bootstrap_claim.table",
        "schema.local_track.original_hash_unique",
        "schema.local_track.product_fields",
        "schema.local_track.table",
        "schema.local_upload_access.table",
        "schema.room_history_track.table",
        "schema.room_playback_state.like_markers_json",
        "schema.room_playback_state.liked_user_ids_json",
        "schema.room_subsonic_source.table",
        "schema.site_setting.table",
        "schema.subsonic_source.owner_room_id",
        "schema.subsonic_source.table",
        "schema.user_account.table",
        "schema.user_binding.table",
        "schema.user_playlist.system_key",
        "schema.user_playlist.table",
        "schema.user_playlist_track.table",
        "schema.user_profile.current_room_id",
    }
    ledger_rows = list(db.execute("select migration_key, completed_at from migration_state"))
    valid = (
        tables == expected - {"room_membership", "room_invite"}
        and missing == {"room_membership", "room_invite"}
        and len(ledger_rows) == len(expected_migration_keys)
        and {migration_key for migration_key, _ in ledger_rows} == expected_migration_keys
        and all(completed_at is not None for _, completed_at in ledger_rows)
    )
    print("valid" if valid else "invalid")
finally:
    db.close()
PY
}

require_prebridge_shape() {
  if ! shape=$(prebridge_shape "$1" "$2" "$3"); then
    fail "cannot inspect the 21-table pre-bridge schema"
  fi
  [ "$shape" = valid ] ||
    fail "pre-bridge schema must lack only room_membership and room_invite with the expected migration ledger"
}

initializer_complete() {
  source_dir=$1
  source_name=$2
  docker_cmd run --pull=never --rm --interactive --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "type=bind,src=$source_dir,dst=/db" --entrypoint python3 "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" - "$source_name" 2>/dev/null <<'PY'
import sqlite3, sys
from urllib.parse import quote
db = sqlite3.connect("file:/db/" + quote(sys.argv[1]) + "?mode=ro", uri=True)
try:
    db.execute("pragma query_only=on")
    tables = {row[0] for row in db.execute("select name from sqlite_master where type = 'table' and name not like 'sqlite_%'")}
    expected_tables = {
        "admin_bootstrap_claim", "chat_message", "local_track", "local_upload_access",
        "migration_state", "room", "room_history", "room_history_track", "room_invite",
        "room_membership", "room_playback_state", "room_playlist", "room_playlist_track",
        "room_queue", "room_subsonic_source", "site_setting", "subsonic_source",
        "user_account", "user_binding", "user_playlist", "user_playlist_track",
        "user_profile", "user_session",
    }
    expected_migration_keys = {
        "legacy.queue-data.json",
        "legacy.rooms.json",
        "schema.admin_bootstrap_claim.table",
        "schema.local_track.original_hash_unique",
        "schema.local_track.product_fields",
        "schema.local_track.table",
        "schema.local_upload_access.table",
        "schema.room_history_track.table",
        "schema.room_playback_state.like_markers_json",
        "schema.room_playback_state.liked_user_ids_json",
        "schema.room_subsonic_source.table",
        "schema.site_setting.table",
        "schema.subsonic_source.owner_room_id",
        "schema.subsonic_source.table",
        "schema.user_account.table",
        "schema.user_binding.table",
        "schema.user_playlist.system_key",
        "schema.user_playlist.table",
        "schema.user_playlist_track.table",
        "schema.user_profile.current_room_id",
    }
    initializer_migration_keys = {
        "schema.room_membership.table",
        "schema.room_invite.table",
        "schema.user_account.platform_admin_role",
    }
    ledger_rows = list(db.execute("select migration_key, completed_at from migration_state"))
    expected_ledger_keys = expected_migration_keys.union(initializer_migration_keys)
    valid = (
        tables == expected_tables
        and len(ledger_rows) == len(expected_ledger_keys)
        and {migration_key for migration_key, _ in ledger_rows} == expected_ledger_keys
        and all(completed_at is not None for _, completed_at in ledger_rows)
    )
    print("valid" if valid else "invalid")
finally:
    db.close()
PY
}

integrity_and_fk() {
  access_mode=$1
  source_dir=$2
  source_name=$3
  mount="type=bind,src=$source_dir,dst=/db"
  case $access_mode in
    live) ;;
    snapshot) mount="$mount,readonly" ;;
    *) fail "invalid SQLite access mode" ;;
  esac
  docker_cmd run --pull=never --rm --interactive --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "$mount" --entrypoint python3 "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" - "$source_name" "$access_mode" 2>/dev/null <<'PY'
import sqlite3, sys
from urllib.parse import quote
suffix = "?mode=ro&immutable=1" if sys.argv[2] == "snapshot" else "?mode=ro"
db = sqlite3.connect("file:/db/" + quote(sys.argv[1]) + suffix, uri=True)
try:
    db.execute("pragma query_only=on")
    integrity = db.execute("pragma integrity_check").fetchall()
    foreign_keys = db.execute("pragma foreign_key_check").fetchall()
    print("valid" if integrity == [("ok",)] and not foreign_keys else "invalid")
finally:
    db.close()
PY
}

require_integrity_and_fk() {
  if ! result=$(integrity_and_fk "$1" "$2" "$3"); then
    fail "cannot run SQLite integrity and foreign-key checks"
  fi
  [ "$result" = valid ] || fail "SQLite integrity or foreign-key check failed"
}

comparison_fingerprint() {
  access_mode=$1
  source_dir=$2
  source_name=$3
  mount="type=bind,src=$source_dir,dst=/db"
  case $access_mode in
    live) ;;
    snapshot) mount="$mount,readonly" ;;
    *) fail "invalid SQLite access mode" ;;
  esac
  docker_cmd run --pull=never --rm --interactive --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "$mount" --entrypoint python3 "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" - "$source_name" "$access_mode" 2>/dev/null <<'PY'
import hashlib, sqlite3, sys
from urllib.parse import quote
suffix = "?mode=ro&immutable=1" if sys.argv[2] == "snapshot" else "?mode=ro"
db = sqlite3.connect("file:/db/" + quote(sys.argv[1]) + suffix, uri=True)
db.text_factory = bytes
def encode(value):
    if value is None: return b"N"
    if isinstance(value, bytes): return b"B" + value
    return b"T" + str(value).encode("utf-8", "surrogatepass")
def digest(query):
    result, count = hashlib.sha256(), 0
    for row in db.execute(query):
        count += 1
        for value in row:
            data = encode(value)
            result.update(len(data).to_bytes(8, "big"))
            result.update(data)
    return count, result.hexdigest()
try:
    db.execute("pragma query_only=on")
    settings = digest("select setting_key, setting_value, secret, updated_at from site_setting where setting_key in ('netease.cookie', 'bilibili.sessdata') order by setting_key")
    sessions = digest("select session_token_hash, public_id, created_at, last_seen_at from user_session order by session_token_hash")
    print(settings[0], settings[1], sessions[0], sessions[1])
finally:
    db.close()
PY
}

comparison_or_fail() {
  if ! result=$(comparison_fingerprint "$1" "$2" "$3"); then
    fail "cannot compare credential settings and user sessions"
  fi
  set -- $result
  [ "$#" -eq 4 ] || fail "credential/session comparison returned invalid data"
  case $1:$3 in *[!0-9:]*|'') fail "credential/session comparison returned invalid counts" ;; esac
  case $2:$4 in *[!0123456789abcdef:]*|'') fail "credential/session comparison returned invalid internal data" ;; esac
  [ "$(printf '%s' "$2" | wc -c | tr -d ' ')" -eq 64 ] &&
    [ "$(printf '%s' "$4" | wc -c | tr -d ' ')" -eq 64 ] ||
    fail "credential/session comparison returned invalid internal data"
  printf '%s %s %s %s\n' "$1" "$2" "$3" "$4"
}

snapshot_database() {
  snapshot_name=$1
  [ ! -e "$BACKUP_ABSOLUTE/$snapshot_name" ] ||
    fail "snapshot already exists; refusing to overwrite it"
  if ! docker_cmd run --pull=never --rm --interactive --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "type=bind,src=$DATA_ABSOLUTE,dst=/source" --mount "type=bind,src=$BACKUP_ABSOLUTE,dst=/backup" --entrypoint python3 "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" - "$snapshot_name" >/dev/null 2>&1 <<'PY'
import os, sqlite3, sys
target = "/backup/" + sys.argv[1]
fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
os.close(fd)
source = destination = None
try:
    source = sqlite3.connect("file:/source/musicparty.db?mode=ro", uri=True)
    source.execute("pragma query_only=on")
    destination = sqlite3.connect(target)
    source.backup(destination)
finally:
    if destination is not None: destination.close()
    if source is not None: source.close()
PY
  then
    fail "consistent SQLite snapshot failed"
  fi
  [ -f "$BACKUP_ABSOLUTE/$snapshot_name" ] ||
    fail "consistent SQLite snapshot was not created"
}

new_bridge_token() {
  require_command od
  [ -r /dev/urandom ] || fail "cannot read /dev/urandom for bridge ownership token"
  token=$(od -An -N 32 -tx1 /dev/urandom | tr -d ' \n') ||
    fail "cannot generate bridge ownership token"
  case $token in ''|*[!0123456789abcdef]*) fail "bridge ownership token was not hexadecimal" ;; esac
  [ "$(printf '%s' "$token" | wc -c | tr -d ' ')" -eq 64 ] ||
    fail "bridge ownership token had an unexpected length"
  printf '%s\n' "$token"
}

bridge_container_is_owned() {
  [ -n "$BRIDGE_CONTAINER" ] && [ -n "$BRIDGE_TOKEN" ] || return 1
  actual=$(docker_cmd inspect --format '{{ index .Config.Labels "musicparty.schema-bridge-token" }}' "$BRIDGE_CONTAINER" 2>/dev/null) ||
    return 1
  [ "$actual" = "$BRIDGE_TOKEN" ]
}

require_bridge_ownership() {
  bridge_container_is_owned ||
    fail "Java schema bridge container ownership check failed"
}

cleanup_bridge() {
  if bridge_container_is_owned; then
    docker_cmd rm --force "$BRIDGE_CONTAINER" >/dev/null 2>&1 || true
  fi
  BRIDGE_CONTAINER=''
  BRIDGE_TOKEN=''
}

trap cleanup_bridge EXIT HUP INT TERM

run_java_initializer() {
  BRIDGE_TOKEN=$(new_bridge_token)
  if ! BRIDGE_CONTAINER=$(docker_cmd run --pull=never --detach --rm --label "musicparty.schema-bridge-token=$BRIDGE_TOKEN" --network none --read-only --tmpfs /tmp:rw,exec,nosuid,nodev,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "type=bind,src=$DATA_ABSOLUTE,dst=/app/data" --env APP_MODE=bridge --env DB_PATH=/app/data/musicparty.db --env DB_INIT_SCHEMA=true --env NETEASE_COOKIE= --env BILIBILI_SESSDATA= --env YOUTUBE_ENABLED=false --env NAVIDROME_ENABLED=false --env SQUIDIFY_ENABLED=false "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" --spring.main.web-application-type=none 2>/dev/null); then
    fail "Java schema bridge container did not start"
  fi
  case $BRIDGE_CONTAINER in ''|*[!0123456789abcdef]*) fail "Java schema bridge container did not return a valid ID" ;; esac
  [ "$(printf '%s' "$BRIDGE_CONTAINER" | wc -c | tr -d ' ')" -ge 12 ] ||
    fail "Java schema bridge container did not return a valid ID"
  attempt=0
  completed=''
  while [ "$attempt" -lt 60 ]; do
    require_bridge_ownership
    if docker_cmd logs "$BRIDGE_CONTAINER" 2>/dev/null | grep -Fq 'SQLite schema initialized'; then
      if completed=$(initializer_complete "$DATA_ABSOLUTE" musicparty.db); then
        [ "$completed" = valid ] ||
          fail "Java schema bridge did not complete the required migrations"
        break
      fi
    fi
    if ! state=$(docker_cmd inspect --format '{{.State.Status}}' "$BRIDGE_CONTAINER" 2>/dev/null); then
      fail "Java schema bridge container became unavailable before initializer completion"
    fi
    case $state in
      running|created|restarting) ;;
      exited|dead) fail "Java schema bridge container exited before initializer completion" ;;
      *) fail "Java schema bridge container entered an unexpected state before initializer completion" ;;
    esac
    attempt=$((attempt + 1))
    sleep 1
  done
  [ "$completed" = valid ] || fail "Java schema bridge did not report the initializer completion marker"
  require_bridge_ownership
  docker_cmd stop --time 30 "$BRIDGE_CONTAINER" >/dev/null 2>&1 ||
    fail "Java schema bridge container did not stop cleanly"
  BRIDGE_CONTAINER=''
  BRIDGE_TOKEN=''
}

run_immutable_dbcheck() {
  snapshot_dir=$(absolute_dir "$(dirname "$1")")
  snapshot_name=$(basename "$1")
  if DBCHECK_OUTPUT=$(docker_cmd run --pull=never --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "type=bind,src=$snapshot_dir,dst=/snapshot,readonly" --entrypoint /app/dbcheck "$MUSIC_PARTY_IMAGE" -db "/snapshot/$snapshot_name" 2>/dev/null); then
    DBCHECK_STATUS=0
  else
    DBCHECK_STATUS=$?
  fi
}

validate_dbcheck() {
  snapshot_kind=$1
  dbcheck_json=$2
  dbcheck_payload=$(printf '%s' "$dbcheck_json" | base64 | tr -d '\r\n')
  [ -n "$dbcheck_payload" ] || fail "immutable Go dbcheck returned no JSON"
  docker_cmd run --pull=never --rm --interactive --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --entrypoint python3 "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" - "$snapshot_kind" "$dbcheck_payload" 2>/dev/null <<'PY'
import base64, json, sys

def exact_pre_differences(value):
    expected = {
        ("table.room_invite", "missing required table"),
        ("table.room_membership", "missing required table"),
    }
    return (
        type(value) is list
        and len(value) == len(expected)
        and all(type(item) is dict and set(item) == {"path", "issue"} for item in value)
        and {(item["path"], item["issue"]) for item in value} == expected
    )

try:
    data = json.loads(base64.b64decode(sys.argv[2], validate=True).decode("utf-8"))
except Exception:
    valid = False
else:
    common = (
        type(data) is dict
        and data.get("integrity") == ["ok"]
        and data.get("foreignKeyViolations") == []
    )
    if sys.argv[1] == "pre-bridge":
        valid = (
            common
            and type(data.get("applicationTables")) is int
            and data["applicationTables"] == 21
            and data.get("schemaCompatible") is False
            and exact_pre_differences(data.get("schemaDifferences"))
        )
    elif sys.argv[1] == "post-bridge":
        valid = (
            common
            and type(data.get("applicationTables")) is int
            and data["applicationTables"] == 23
            and data.get("schemaCompatible") is True
            and data.get("schemaDifferences") == []
        )
    else:
        valid = False
print("valid" if valid else "invalid")
PY
}

snapshot_kind_from_dbcheck() {
  run_immutable_dbcheck "$1"
  case $DBCHECK_STATUS in
    0) snapshot_kind=post-bridge ;;
    1) snapshot_kind=pre-bridge ;;
    *) fail "immutable Go dbcheck execution failed" ;;
  esac
  if ! result=$(validate_dbcheck "$snapshot_kind" "$DBCHECK_OUTPUT"); then
    fail "cannot parse immutable Go dbcheck JSON"
  fi
  [ "$result" = valid ] ||
    fail "immutable Go dbcheck did not report the exact $snapshot_kind snapshot state"
  printf '%s\n' "$snapshot_kind"
}

verify() {
  require_command docker
  require_command base64
  require_images
  require_runtime_ids
  require_local_images
  [ "$#" -eq 1 ] || fail "verify requires a snapshot path"
  [ -f "$1" ] || fail "snapshot not found"
  snapshot_kind=$(snapshot_kind_from_dbcheck "$1")
  if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum "$1" 2>/dev/null) || fail "cannot calculate snapshot SHA-256"
  elif command -v shasum >/dev/null 2>&1; then
    digest=$(shasum -a 256 "$1" 2>/dev/null) || fail "cannot calculate snapshot SHA-256"
  else
    fail "sha256sum or shasum is required to record the snapshot digest"
  fi
  digest=$(printf '%s' "$digest" | cut -d ' ' -f 1)
  case $digest in *[!0123456789abcdef]*|'') fail "snapshot SHA-256 calculation returned invalid data" ;; esac
  [ "$(printf '%s' "$digest" | wc -c | tr -d ' ')" -eq 64 ] ||
    fail "snapshot SHA-256 calculation returned invalid data"
  case $snapshot_kind in
    pre-bridge) printf '%s\n' 'Go dbcheck schemaCompatible=false integrity=ok foreignKeyViolations=0 applicationTables=21' ;;
    post-bridge) printf '%s\n' 'Go dbcheck schemaCompatible=true integrity=ok foreignKeyViolations=0 applicationTables=23' ;;
    *) fail "immutable Go dbcheck returned an unknown snapshot kind" ;;
  esac
  printf '%s\n' "snapshot-kind=$snapshot_kind"
  printf '%s\n' "snapshot=$1"
  printf '%s\n' "snapshot-sha256=$digest"
}

preflight() {
  require_command docker
  require_command base64
  require_command od
  require_images
  require_runtime_ids
  docker_cmd compose version >/dev/null 2>&1 || fail "docker compose is required"
  [ -f "$COMPOSE_FILE" ] || fail "compose file not found"
  [ -f "$GO_COMPOSE_FILE" ] || fail "Go compose override not found"
  DATA_ABSOLUTE=$(absolute_dir "$DATA_DIR")
  [ -f "$DATA_ABSOLUTE/musicparty.db" ] || fail "database not found"
  [ -w "$DATA_ABSOLUTE" ] || fail "data directory is not writable by the operator"
  require_local_images
  require_original_java_container
  docker_cmd run --pull=never --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "type=bind,src=$DATA_ABSOLUTE,dst=/data" --entrypoint /bin/sh "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" -c 'test -r /data/musicparty.db && test -w /data/musicparty.db && test -w /data' >/dev/null 2>&1 || fail "recorded Java uid/gid cannot write the database and data directory"
  require_prebridge_shape live "$DATA_ABSOLUTE" musicparty.db
  docker_cmd compose -f "$COMPOSE_FILE" -f "$GO_COMPOSE_FILE" config --quiet >/dev/null 2>&1 ||
    fail "Compose configuration is invalid"
  printf '%s\n' 'preflight passed applicationTables=21'
}

apply() {
  preflight
  confirmed=$(env_value MUSICPARTY_MAINTENANCE_CONFIRMED)
  [ "$confirmed" = YES ] ||
    fail "set MUSICPARTY_MAINTENANCE_CONFIRMED=YES after entering the maintenance window"
  state=$(docker_cmd inspect --format '{{.State.Status}}' "$CONTAINER_NAME" 2>/dev/null) ||
    fail "application container cannot be inspected"
  [ "$state" = exited ] ||
    fail "application container must explicitly be exited before apply"
  mkdir -p "$BACKUP_DIR"
  BACKUP_ABSOLUTE=$(absolute_dir "$BACKUP_DIR")
  [ -w "$BACKUP_ABSOLUTE" ] || fail "backup directory is not writable by the operator"
  docker_cmd run --pull=never --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m --cap-drop=ALL --security-opt=no-new-privileges --user "$MUSIC_PARTY_RUNTIME_UID:$MUSIC_PARTY_RUNTIME_GID" --mount "type=bind,src=$BACKUP_ABSOLUTE,dst=/backup" --entrypoint /bin/sh "$MUSIC_PARTY_JAVA_BRIDGE_IMAGE" -c 'test -w /backup' >/dev/null 2>&1 ||
    fail "recorded Java uid/gid cannot write the backup directory"
  timestamp=$(date -u +%Y%m%dT%H%M%SZ)
  pre_name="musicparty-pre-bridge-$timestamp.db"
  post_name="musicparty-post-bridge-pre-go-$timestamp.db"
  [ ! -e "$BACKUP_ABSOLUTE/$pre_name" ] ||
    fail "pre-bridge snapshot already exists; refusing to overwrite it"
  [ ! -e "$BACKUP_ABSOLUTE/$post_name" ] ||
    fail "post-bridge snapshot already exists; refusing to overwrite it"
  snapshot_database "$pre_name"
  require_prebridge_shape snapshot "$BACKUP_ABSOLUTE" "$pre_name"
  require_integrity_and_fk snapshot "$BACKUP_ABSOLUTE" "$pre_name"
  snapshot_kind=$(snapshot_kind_from_dbcheck "$BACKUP_ABSOLUTE/$pre_name")
  [ "$snapshot_kind" = pre-bridge ] ||
    fail "pre-bridge snapshot did not report the known 21-table incompatibility"
  before=$(comparison_or_fail snapshot "$BACKUP_ABSOLUTE" "$pre_name")
  run_java_initializer
  require_table_count 23 live "$DATA_ABSOLUTE" musicparty.db
  after=$(comparison_or_fail live "$DATA_ABSOLUTE" musicparty.db)
  set -- $before
  before_settings_count=$1
  before_settings_digest=$2
  before_sessions_count=$3
  before_sessions_digest=$4
  set -- $after
  after_settings_count=$1
  after_settings_digest=$2
  after_sessions_count=$3
  after_sessions_digest=$4
  [ "$before_settings_count" = "$after_settings_count" ] &&
    [ "$before_settings_digest" = "$after_settings_digest" ] ||
    fail "credential settings changed during the Java schema bridge"
  [ "$before_sessions_count" = "$after_sessions_count" ] &&
    [ "$before_sessions_digest" = "$after_sessions_digest" ] ||
    fail "existing user sessions changed during the Java schema bridge"
  snapshot_database "$post_name"
  require_table_count 23 snapshot "$BACKUP_ABSOLUTE" "$post_name"
  require_integrity_and_fk snapshot "$BACKUP_ABSOLUTE" "$post_name"
  printf '%s\n' "credential settings unchanged=true rows=$after_settings_count"
  printf '%s\n' "existing user sessions unchanged=true rows=$after_sessions_count"
  printf '%s\n' "pre-bridge snapshot=$BACKUP_ABSOLUTE/$pre_name"
  verify "$BACKUP_ABSOLUTE/$post_name"
  printf '%s\n' "post-bridge pre-Go snapshot=$BACKUP_ABSOLUTE/$post_name"
  printf '%s\n' 'apply passed; no application service was started or stopped'
}

DATA_DIR=$(env_value DATA_DIR)
COMPOSE_FILE=$(env_value COMPOSE_FILE)
GO_COMPOSE_FILE=$(env_value GO_COMPOSE_FILE)
CONTAINER_NAME=$(env_value CONTAINER_NAME)
BACKUP_DIR=$(env_value BACKUP_DIR)
[ -n "$DATA_DIR" ] || DATA_DIR=./music_party/data
[ -n "$COMPOSE_FILE" ] || COMPOSE_FILE=./docker-compose.yml
[ -n "$GO_COMPOSE_FILE" ] || GO_COMPOSE_FILE=./backend-go/deploy/compose.go.yml
[ -n "$CONTAINER_NAME" ] || CONTAINER_NAME=music-party-app
[ -n "$BACKUP_DIR" ] || BACKUP_DIR=./music_party/backups/go-cutover

case "$#" in
  1) command=$1 ;;
  2) command=$1 ;;
  *) command='' ;;
esac
case $command in
  preflight) [ "$#" -eq 1 ] || { usage; exit 2; }; preflight ;;
  apply) [ "$#" -eq 1 ] || { usage; exit 2; }; apply ;;
  verify) shift; verify "$@" ;;
  *) usage; exit 2 ;;
esac
