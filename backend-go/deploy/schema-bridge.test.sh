#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd -P)
SCRIPT="$ROOT/backend-go/deploy/schema-bridge.sh"
GO_IMAGE='ghcr.io/bigblackblob/musicparty-go@sha256:f16d4a26b94d28ca42e47c0d1bee8043f5f2434d2a982f375290ef49c390e759'
JAVA_IMAGE='ghcr.io/bigblackblob/musicparty@sha256:1f08392fa699bcb655e3fd2a6eb2347432378804412b2e5521f955e48cf299e2'
ORIGINAL_IMAGE='ghcr.io/bigblackblob/musicparty@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
SECRET='schema-bridge-secret-must-not-appear'
BRIDGE_ID='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
SANDBOX=$(mktemp -d /tmp/schema-bridge-test.XXXXXX)

cleanup() {
  case "$SANDBOX" in
    /tmp/schema-bridge-test.*) rm -rf "$SANDBOX" ;;
    *) printf '%s\n' 'unsafe test sandbox path' >&2; exit 1 ;;
  esac
}
trap cleanup EXIT

fail() {
  printf '%s\n' "test failed: $*" >&2
  exit 1
}

contains() {
  [[ $1 == *"$2"* ]] || fail "expected output to contain: $2"
}

no_secret() {
  [[ $1 != *"$SECRET"* ]] || fail 'unsafe output contained protected test data'
}

mkdir -p "$SANDBOX/bin" "$SANDBOX/data" "$SANDBOX/backups"
: > "$SANDBOX/data/musicparty.db"
: > "$SANDBOX/compose.yml"
: > "$SANDBOX/compose.go.yml"

cat > "$SANDBOX/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
conversion=$(printenv MSYS_NO_PATHCONV 2>/dev/null || true)
args="$*"
bridge_id=$(printenv FAKE_BRIDGE_ID 2>/dev/null || true)
[[ -n $bridge_id ]] || bridge_id=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
pre_dbcheck='{"integrity":["ok"],"foreignKeyViolations":[],"applicationTables":21,"schemaCompatible":false,"schemaDifferences":[{"path":"table.room_invite","issue":"missing required table"},{"path":"table.room_membership","issue":"missing required table"}]}'
post_dbcheck='{"integrity":["ok"],"foreignKeyViolations":[],"applicationTables":23,"schemaCompatible":true,"schemaDifferences":[]}'
if [[ $args == *'--entrypoint python3'* ]]; then
  python_source=$(cat)
  args="$args $python_source"
  [[ $args != *' -c '* ]] || exit 1
fi
printf 'pathconv=%s args=%s\n' "$conversion" "$args" >> "$FAKE_DOCKER_LOG"
if [[ $1 == compose ]]; then exit 0; fi
if [[ $1 == image && $2 == inspect ]]; then
  missing=$(printenv FAKE_LOCAL_IMAGE_MISSING 2>/dev/null || true)
  [[ -z $missing || $args != *"$missing"* ]] || exit 1
  [[ $args == *'{{.Id}}'* ]] && printf '%s\n' 'sha256:java-image-id'
  exit 0
fi
if [[ $1 == inspect ]]; then
  if [[ $args == *'.Config.Labels'* ]]; then
    owner=$(printenv FAKE_BRIDGE_OWNERSHIP 2>/dev/null || true)
    if [[ -z $owner && -f $FAKE_BRIDGE_OWNER_FILE ]]; then owner=$(<"$FAKE_BRIDGE_OWNER_FILE"); fi
    printf '%s\n' "$owner"
    exit 0
  fi
  if [[ $args == *'{{.State.Status}}'* ]]; then
    if [[ $args == *"$bridge_id"* ]]; then
      state=$(printenv FAKE_BRIDGE_STATUS 2>/dev/null || true)
      [[ -n $state ]] || state=exited
    else
      state=$(printenv FAKE_RUNNING 2>/dev/null || true)
      [[ -n $state ]] || state=exited
    fi
    printf '%s\n' "$state"
  fi
  if [[ $args == *'{{.Image}}'* ]]; then printf '%s\n' 'sha256:java-image-id'; fi
  exit 0
fi
if [[ $1 == container && $2 == inspect ]]; then exit 1; fi
if [[ $1 == logs ]]; then
  marker=$(printenv FAKE_BRIDGE_MARKER 2>/dev/null || true)
  [[ -n $marker ]] || marker='SQLite schema initialized'
  printf '%s\n' "$marker"
  exit 0
fi
if [[ $1 == stop || $1 == rm ]]; then exit 0; fi
[[ $1 == run ]] || exit 0
if [[ $args == *'mode=ro&immutable=1'* && $args == *' snapshot' && $args != *'dst=/db,readonly'* ]]; then exit 1; fi
if [[ $args == *'source.backup'* && ( $args != *'dst=/source '* || $args != *'source.execute("pragma query_only=on")'* ) ]]; then exit 1; fi
if [[ $args == *'--detach'* ]]; then
  label=''
  next_is_label=false
  for arg in "$@"; do
    if [[ $next_is_label == true ]]; then label=$arg; break; fi
    [[ $arg == --label ]] && next_is_label=true
  done
  printf '%s\n' "${label#musicparty.schema-bridge-token=}" > "$FAKE_BRIDGE_OWNER_FILE"
  : > "$FAKE_STARTED"
  printf '%s\n' "$bridge_id"
  exit 0
fi
if [[ $args == *'--entrypoint /app/dbcheck'* ]]; then
  if [[ $args == *'musicparty-pre-bridge-'* ]]; then
    output=$(printenv FAKE_PRE_DBCHECK_JSON 2>/dev/null || true)
    [[ -n $output ]] || output=$pre_dbcheck
    printf '%s\n' "$output"
    exit 1
  fi
  output=$(printenv FAKE_POST_DBCHECK_JSON 2>/dev/null || true)
  [[ -n $output ]] || output=$post_dbcheck
  printf '%s\n' "$output"
  exit 0
fi
if [[ $args == *'exact_pre_differences'* && $args == *'json.loads'* ]]; then
  parser_kind=''
  for arg in "$@"; do
    case $arg in pre-bridge|post-bridge) parser_kind=$arg ;; esac
  done
  last=''
  for last in "$@"; do :; done
  decoded=$(printf '%s' "$last" | base64 -d) || exit 1
  case $parser_kind:$decoded in
    pre-bridge:"$pre_dbcheck"|post-bridge:"$post_dbcheck") printf '%s\n' valid ;;
    *) printf '%s\n' invalid ;;
  esac
  exit 0
fi
if [[ $args == *'source.backup'* ]]; then
  last=''
  for last in "$@"; do :; done
  backup=''
  for arg in "$@"; do
    case "$arg" in
      type=bind,src=*,dst=/backup*) backup=$(printf '%s' "$arg" | sed 's/^type=bind,src=//; s/,dst=\/backup.*//') ;;
    esac
  done
  : > "$backup/$last"
  exit 0
fi
if [[ $args == *'pragma integrity_check'* ]]; then printf '%s\n' valid; exit 0; fi
if [[ $args == *'expected_migration_keys'* && $args != *'initializer_migration_keys'* ]]; then
  shape=$(printenv FAKE_PRE_SHAPE 2>/dev/null || true)
  arbitrary_ledger_key=$(printenv FAKE_PRE_LEDGER_KEY 2>/dev/null || true)
  [[ -z $arbitrary_ledger_key ]] || shape=invalid
  [[ -n $shape ]] || shape=valid
  printf '%s\n' "$shape"
  exit 0
fi
if [[ $args == *'expected_migration_keys'* && $args == *'initializer_migration_keys'* ]]; then
  shape=$(printenv FAKE_POST_SHAPE 2>/dev/null || true)
  arbitrary_ledger_key=$(printenv FAKE_POST_LEDGER_KEY 2>/dev/null || true)
  [[ -z $arbitrary_ledger_key ]] || shape=invalid
  [[ -n $shape ]] || shape=valid
  printf '%s\n' "$shape"
  exit 0
fi
if [[ $args == *'hashlib'* ]]; then
  printf '%s\n' '2 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 3 bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
  exit 0
fi
if [[ $args == *'sqlite_master'* ]]; then
  if [[ $args == *'musicparty-post-bridge-pre-go-'* || -f $FAKE_STARTED ]]; then printf '%s\n' 23; else printf '%s\n' 21; fi
  exit 0
fi
exit 0
EOF
chmod +x "$SANDBOX/bin/docker"

cat > "$SANDBOX/bin/date" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' '20260803T010203Z'
EOF
chmod +x "$SANDBOX/bin/date"

cat > "$SANDBOX/bin/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$SANDBOX/bin/sleep"

export PATH="$SANDBOX/bin:$PATH"
export FAKE_DOCKER_LOG="$SANDBOX/docker.log"
export FAKE_STARTED="$SANDBOX/started"
export FAKE_BRIDGE_ID="$BRIDGE_ID"
export FAKE_BRIDGE_OWNER_FILE="$SANDBOX/bridge-owner"
export MUSIC_PARTY_IMAGE="$GO_IMAGE"
export MUSIC_PARTY_JAVA_BRIDGE_IMAGE="$JAVA_IMAGE"
export MUSIC_PARTY_ORIGINAL_JAVA_IMAGE="$ORIGINAL_IMAGE"
export MUSIC_PARTY_RUNTIME_UID=100
export MUSIC_PARTY_RUNTIME_GID=101
export DATA_DIR="$SANDBOX/data"
export COMPOSE_FILE="$SANDBOX/compose.yml"
export GO_COMPOSE_FILE="$SANDBOX/compose.go.yml"
export BACKUP_DIR="$SANDBOX/backups"
export CONTAINER_NAME=music-party-app
unset MUSICPARTY_MAINTENANCE_CONFIRMED FAKE_RUNNING FAKE_PRE_SHAPE FAKE_POST_SHAPE FAKE_BRIDGE_STATUS FAKE_BRIDGE_MARKER FAKE_BRIDGE_OWNERSHIP FAKE_PRE_LEDGER_KEY FAKE_POST_LEDGER_KEY FAKE_PRE_DBCHECK_JSON FAKE_POST_DBCHECK_JSON FAKE_LOCAL_IMAGE_MISSING

bash -n "$SCRIPT"
preflight_output=$(sh "$SCRIPT" preflight)
contains "$preflight_output" 'preflight passed applicationTables=21'

export MUSIC_PARTY_RUNTIME_UID=invalid
if unsafe_output=$(sh "$SCRIPT" preflight 2>&1); then fail 'invalid uid unexpectedly passed'; fi
contains "$unsafe_output" 'MUSIC_PARTY_RUNTIME_UID must be a numeric id'
no_secret "$unsafe_output"
export MUSIC_PARTY_RUNTIME_UID=100

export MUSIC_PARTY_RUNTIME_GID=invalid
if unsafe_output=$(sh "$SCRIPT" preflight 2>&1); then fail 'invalid gid unexpectedly passed'; fi
contains "$unsafe_output" 'MUSIC_PARTY_RUNTIME_GID must be a numeric id'
no_secret "$unsafe_output"
export MUSIC_PARTY_RUNTIME_GID=101

export MUSIC_PARTY_IMAGE='ghcr.io/bigblackblob/musicparty-go:mutable'
if unsafe_output=$(sh "$SCRIPT" preflight 2>&1); then fail 'mutable image unexpectedly passed'; fi
contains "$unsafe_output" 'MUSIC_PARTY_IMAGE must use an immutable @sha256: digest'
no_secret "$unsafe_output"
export MUSIC_PARTY_IMAGE="$GO_IMAGE"

export FAKE_PRE_SHAPE=invalid
if unsafe_output=$(sh "$SCRIPT" preflight 2>&1); then fail 'wrong 21-table shape unexpectedly passed'; fi
contains "$unsafe_output" 'pre-bridge schema must lack only'
unset FAKE_PRE_SHAPE

export FAKE_PRE_LEDGER_KEY='unexpected.migration.key'
if unsafe_output=$(sh "$SCRIPT" preflight 2>&1); then fail 'arbitrary pre-bridge ledger key unexpectedly passed'; fi
contains "$unsafe_output" 'pre-bridge schema must lack only'
no_secret "$unsafe_output"
unset FAKE_PRE_LEDGER_KEY

export MUSICPARTY_MAINTENANCE_CONFIRMED=YES
export FAKE_RUNNING=running
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'running writer unexpectedly passed'; fi
contains "$unsafe_output" 'application container must explicitly be exited'
no_secret "$unsafe_output"
unset FAKE_RUNNING

unset MUSICPARTY_MAINTENANCE_CONFIRMED
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'missing confirmation unexpectedly passed'; fi
contains "$unsafe_output" 'MUSICPARTY_MAINTENANCE_CONFIRMED=YES'
no_secret "$unsafe_output"

export MUSICPARTY_MAINTENANCE_CONFIRMED=YES
preexisting="$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db"
: > "$preexisting"
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'existing snapshot unexpectedly passed'; fi
contains "$unsafe_output" 'pre-bridge snapshot already exists'
[[ -f $preexisting ]] || fail 'existing snapshot was removed'
rm -f "$preexisting"

export FAKE_PRE_DBCHECK_JSON='{"integrity":["ok"],"foreignKeyViolations":[],"applicationTables":21,"schemaCompatible":false,"schemaDifferences":[{"path":"table.room_invite","issue":"missing required table"},{"path":"table.unexpected","issue":"missing required table"}]}'
: > "$SANDBOX/docker.log"
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'wrong pre-snapshot schema difference unexpectedly passed'; fi
contains "$unsafe_output" 'exact pre-bridge snapshot state'
no_secret "$unsafe_output"
if grep -q -- '--detach' "$SANDBOX/docker.log"; then fail 'Java initializer started before the pre-snapshot semantic gate'; fi
unset FAKE_PRE_DBCHECK_JSON
rm -f "$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db"

export FAKE_BRIDGE_OWNERSHIP='foreign-bridge-token'
: > "$SANDBOX/docker.log"
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'foreign bridge ownership unexpectedly passed'; fi
contains "$unsafe_output" 'Java schema bridge container ownership check failed'
no_secret "$unsafe_output"
if grep -Fq -- "rm --force $BRIDGE_ID" "$SANDBOX/docker.log"; then fail 'cleanup removed a container with mismatched ownership'; fi
if grep -Fq -- "logs $BRIDGE_ID" "$SANDBOX/docker.log"; then fail 'logs inspected a container with mismatched ownership'; fi
if grep -Fq -- "stop --time 30 $BRIDGE_ID" "$SANDBOX/docker.log"; then fail 'stop targeted a container with mismatched ownership'; fi
unset FAKE_BRIDGE_OWNERSHIP
rm -f "$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db"

export FAKE_BRIDGE_MARKER="$SECRET"
export FAKE_BRIDGE_STATUS=exited
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'early-exited Java initializer unexpectedly passed'; fi
contains "$unsafe_output" 'Java schema bridge container exited before initializer completion'
no_secret "$unsafe_output"
unset FAKE_BRIDGE_MARKER FAKE_BRIDGE_STATUS
rm -f "$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db"

export FAKE_POST_SHAPE=invalid
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'incomplete Java initializer unexpectedly passed'; fi
contains "$unsafe_output" 'Java schema bridge did not complete the required migrations'
no_secret "$unsafe_output"
unset FAKE_POST_SHAPE
rm -f "$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db"

export FAKE_POST_LEDGER_KEY='unexpected.post.bridge.migration'
if unsafe_output=$(sh "$SCRIPT" apply 2>&1); then fail 'arbitrary post-bridge ledger key unexpectedly passed'; fi
contains "$unsafe_output" 'Java schema bridge did not complete the required migrations'
no_secret "$unsafe_output"
unset FAKE_POST_LEDGER_KEY
rm -f "$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db"

rm -f "$SANDBOX/started"
: > "$SANDBOX/docker.log"
apply_output=$(sh "$SCRIPT" apply)
contains "$apply_output" 'credential settings unchanged=true rows=2'
contains "$apply_output" 'existing user sessions unchanged=true rows=3'
contains "$apply_output" 'Go dbcheck schemaCompatible=true integrity=ok foreignKeyViolations=0 applicationTables=23'
contains "$apply_output" 'apply passed; no application service was started or stopped'
no_secret "$apply_output"
[[ -f "$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db" ]] || fail 'pre-bridge snapshot missing'
[[ -f "$SANDBOX/backups/musicparty-post-bridge-pre-go-20260803T010203Z.db" ]] || fail 'post-bridge snapshot missing'
grep -q -- '--network none' "$SANDBOX/docker.log" || fail 'offline Docker networking was not requested'
grep -q -- '--user 100:101' "$SANDBOX/docker.log" || fail 'recorded Java uid/gid was not used'
grep -q -- '--entrypoint /app/dbcheck' "$SANDBOX/docker.log" || fail 'immutable Go dbcheck was not invoked'
if grep -E 'docker_cmd run ' "$SCRIPT" | grep -qv -- '--pull=never'; then fail 'a Docker task could pull a mutable image'; fi
grep -Eq 'args=run --pull=never --detach --rm --label musicparty\.schema-bridge-token=[0-9a-f]{64} --network none --read-only --tmpfs /tmp:rw,exec,nosuid,nodev,size=64m' "$SANDBOX/docker.log" || fail 'Java initializer did not receive an owned executable tmpfs task'
if grep -- '--detach' "$SANDBOX/docker.log" | grep -q -- '--name'; then fail 'Java initializer used a predictable container name'; fi
if grep -- '--tmpfs /tmp:rw,exec,nosuid,nodev,size=64m' "$SANDBOX/docker.log" | grep -qv -- '--detach'; then fail 'executable tmpfs escaped the Java initializer'; fi
if grep -- '--entrypoint python3' "$SANDBOX/docker.log" | grep -qv -- '--interactive'; then fail 'Python tasks were not attached for stdin transport'; fi
if grep -- '--entrypoint python3' "$SANDBOX/docker.log" | grep -qv -- '--tmpfs /tmp:rw,noexec,nosuid,size=64m'; then fail 'Python tasks lost their noexec tmpfs'; fi
if grep -- '--entrypoint /app/dbcheck' "$SANDBOX/docker.log" | grep -qv -- '--tmpfs /tmp:rw,noexec,nosuid,size=64m'; then fail 'Go dbcheck lost its noexec tmpfs'; fi
grep -Fq -- "--entrypoint python3 $JAVA_IMAGE -" "$SANDBOX/docker.log" || fail 'Python tasks did not receive source through stdin'
grep -q 'mode=ro&immutable=1' "$SANDBOX/docker.log" || fail 'closed snapshots were not opened immutable'
grep -q 'pragma query_only=on' "$SANDBOX/docker.log" || fail 'live SQLite reads did not set query_only'
grep -q 'source.execute("pragma query_only=on")' "$SANDBOX/docker.log" || fail 'snapshot source did not set query_only'
if [[ $(uname -s) == MINGW* || $(uname -s) == MSYS* ]]; then
  grep -q '^pathconv=1 args=logs ' "$SANDBOX/docker.log" || fail 'Docker wrapper did not disable MSYS path conversion'
  grep -Eq 'type=bind,src=[A-Za-z]:/.+,dst=/db,readonly' "$SANDBOX/docker.log" || fail 'Windows host bind source was not normalized'
  grep -Eq 'type=bind,src=[A-Za-z]:/.+,dst=/db --entrypoint python3' "$SANDBOX/docker.log" || fail 'Windows live SQLite bind source was not normalized'
  grep -Eq 'type=bind,src=[A-Za-z]:/.+,dst=/source --mount' "$SANDBOX/docker.log" || fail 'Windows snapshot source was not normalized'
else
  grep -q '^pathconv= args=logs ' "$SANDBOX/docker.log" || fail 'Linux Docker wrapper unexpectedly set MSYS path conversion'
  grep -Eq 'type=bind,src=/tmp/.+,dst=/db,readonly' "$SANDBOX/docker.log" || fail 'Linux snapshot bind source was not normalized'
  grep -Eq 'type=bind,src=/tmp/.+,dst=/db --entrypoint python3' "$SANDBOX/docker.log" || fail 'Linux live SQLite bind source was not normalized'
  grep -Eq 'type=bind,src=/tmp/.+,dst=/source --mount' "$SANDBOX/docker.log" || fail 'Linux snapshot source was not normalized'
fi
if grep -Eq '^pathconv=1 args=compose (up|start|stop|down)' "$SANDBOX/docker.log"; then fail 'script changed a Compose service lifecycle'; fi

verify_output=$(sh "$SCRIPT" verify "$SANDBOX/backups/musicparty-post-bridge-pre-go-20260803T010203Z.db")
contains "$verify_output" 'Go dbcheck schemaCompatible=true'
contains "$verify_output" 'snapshot-kind=post-bridge'
contains "$verify_output" 'snapshot-sha256='
no_secret "$verify_output"

verify_pre_output=$(sh "$SCRIPT" verify "$SANDBOX/backups/musicparty-pre-bridge-20260803T010203Z.db")
contains "$verify_pre_output" 'Go dbcheck schemaCompatible=false'
contains "$verify_pre_output" 'snapshot-kind=pre-bridge'
contains "$verify_pre_output" 'snapshot-sha256='
no_secret "$verify_pre_output"

export FAKE_POST_DBCHECK_JSON='{"integrity":["ok"],"foreignKeyViolations":[],"applicationTables":230,"schemaCompatible":true,"schemaDifferences":[]}'
if unsafe_output=$(sh "$SCRIPT" verify "$SANDBOX/backups/musicparty-post-bridge-pre-go-20260803T010203Z.db" 2>&1); then fail 'applicationTables=230 unexpectedly passed'; fi
contains "$unsafe_output" 'exact post-bridge snapshot state'
no_secret "$unsafe_output"
unset FAKE_POST_DBCHECK_JSON

export FAKE_LOCAL_IMAGE_MISSING="$GO_IMAGE"
if unsafe_output=$(sh "$SCRIPT" verify "$SANDBOX/backups/musicparty-post-bridge-pre-go-20260803T010203Z.db" 2>&1); then fail 'standalone verify accepted a missing local image'; fi
contains "$unsafe_output" 'approved Go image is not present locally'
no_secret "$unsafe_output"
unset FAKE_LOCAL_IMAGE_MISSING

printf '%s\n' 'schema-bridge safety tests passed'
