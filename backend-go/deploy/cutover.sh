#!/usr/bin/env sh
set -eu

usage() {
  cat <<'EOF'
Usage: cutover.sh preflight|snapshot|verify [snapshot-path]

Required environment:
  MUSIC_PARTY_IMAGE  Immutable Go image reference containing @sha256:

Optional environment:
  DATA_DIR           Host data directory (default: ./music_party/data)
  COMPOSE_FILE       Base compose file (default: ./docker-compose.yml)
  GO_COMPOSE_FILE    Go override (default: ./backend-go/deploy/compose.go.yml)
  CONTAINER_NAME     Application container (default: music-party-app)
  BACKUP_DIR         Snapshot directory (default: ./music_party/backups/go-cutover)

snapshot additionally requires MUSICPARTY_MAINTENANCE_CONFIRMED=YES and the
application container to be stopped. Existing files are never overwritten.
EOF
}

fail() {
  printf '%s\n' "error: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

absolute_dir() {
  directory=$1
  [ -d "$directory" ] || fail "directory does not exist: $directory"
  (cd "$directory" && pwd -P)
}

require_image_digest() {
  case ${MUSIC_PARTY_IMAGE:-} in
    *@sha256:*) ;;
    *) fail "MUSIC_PARTY_IMAGE must use an immutable @sha256: digest" ;;
  esac
}

container_is_running() {
  [ "$(docker inspect --format '{{.State.Running}}' "$CONTAINER_NAME" 2>/dev/null || true)" = "true" ]
}

preflight() {
  require_command docker
  require_image_digest
  docker compose version >/dev/null
  [ -f "$COMPOSE_FILE" ] || fail "compose file not found: $COMPOSE_FILE"
  [ -f "$GO_COMPOSE_FILE" ] || fail "Go compose override not found: $GO_COMPOSE_FILE"
  data_absolute=$(absolute_dir "$DATA_DIR")
  [ -f "$data_absolute/musicparty.db" ] || fail "database not found: $data_absolute/musicparty.db"
  [ -w "$data_absolute" ] || fail "data directory is not writable by the operator: $data_absolute"
  docker image inspect "$MUSIC_PARTY_IMAGE" >/dev/null 2>&1 || fail "Go image is not present locally; pull and inspect it first"
  docker compose -f "$COMPOSE_FILE" -f "$GO_COMPOSE_FILE" config --quiet
  printf '%s\n' "preflight passed"
}

snapshot() {
  preflight
  [ "${MUSICPARTY_MAINTENANCE_CONFIRMED:-}" = "YES" ] || fail "set MUSICPARTY_MAINTENANCE_CONFIRMED=YES after entering the maintenance window"
  container_is_running && fail "$CONTAINER_NAME is still running; stop it before taking the cutover snapshot"

  data_absolute=$(absolute_dir "$DATA_DIR")
  mkdir -p "$BACKUP_DIR"
  backup_absolute=$(absolute_dir "$BACKUP_DIR")
  timestamp=$(date -u +%Y%m%dT%H%M%SZ)
  snapshot_name="musicparty-pre-go-$timestamp.db"
  snapshot_path="$backup_absolute/$snapshot_name"

  docker run --rm \
    --mount "type=bind,src=$backup_absolute,dst=/backup" \
    --entrypoint /bin/sh \
    "$MUSIC_PARTY_IMAGE" -c 'test -w /backup' \
    || fail "backup directory is not writable by the image user (uid 10001): $backup_absolute"

  docker run --rm \
    --read-only \
    --tmpfs /tmp:rw,noexec,nosuid,size=64m \
    --cap-drop=ALL \
    --security-opt=no-new-privileges \
    --mount "type=bind,src=$data_absolute,dst=/source,readonly" \
    --mount "type=bind,src=$backup_absolute,dst=/backup" \
    --entrypoint /app/dbsnapshot \
    "$MUSIC_PARTY_IMAGE" \
    -source /source/musicparty.db -destination "/backup/$snapshot_name"

  verify "$snapshot_path"
  printf '%s\n' "snapshot=$snapshot_path"
}

verify() {
  require_command docker
  require_image_digest
  snapshot_path=${1:-}
  [ -n "$snapshot_path" ] || fail "verify requires a snapshot path"
  [ -f "$snapshot_path" ] || fail "snapshot not found: $snapshot_path"
  snapshot_dir=$(absolute_dir "$(dirname "$snapshot_path")")
  snapshot_name=$(basename "$snapshot_path")

  docker run --rm \
    --read-only \
    --cap-drop=ALL \
    --security-opt=no-new-privileges \
    --mount "type=bind,src=$snapshot_dir,dst=/snapshot,readonly" \
    --entrypoint /app/dbcheck \
    "$MUSIC_PARTY_IMAGE" -db "/snapshot/$snapshot_name"

  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$snapshot_path"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$snapshot_path"
  else
    fail "sha256sum or shasum is required to record the snapshot digest"
  fi
}

DATA_DIR=${DATA_DIR:-./music_party/data}
COMPOSE_FILE=${COMPOSE_FILE:-./docker-compose.yml}
GO_COMPOSE_FILE=${GO_COMPOSE_FILE:-./backend-go/deploy/compose.go.yml}
CONTAINER_NAME=${CONTAINER_NAME:-music-party-app}
BACKUP_DIR=${BACKUP_DIR:-./music_party/backups/go-cutover}

case ${1:-} in
  preflight) preflight ;;
  snapshot) snapshot ;;
  verify) verify "${2:-}" ;;
  *) usage; exit 2 ;;
esac
