#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend-go"
FRONTEND_DIR="$ROOT_DIR/music-party-web"
LOG_DIR="$ROOT_DIR/.dev-logs"
RUNTIME_DIR="$LOG_DIR/runtime"
PID_DIR="$LOG_DIR/pids"
COOKIE_FILE="$ROOT_DIR/cookies.json"

START_NETEASE_API=false
NAVIDROME_LOCAL=false
SKIP_BROWSER=false
BACKEND_ONLY=false
FRONTEND_ONLY=false
ENV_FILE="$ROOT_DIR/.env.local"
BACKEND_PORT="${SERVER_PORT:-18081}"
FRONTEND_PORT="${VITE_PORT:-5173}"
NETEASE_API_PORT="${API_PORT:-3000}"
STARTED_PIDS=()

info() { printf '[dev] %s\n' "$*"; }
warn() { printf '[dev][warn] %s\n' "$*" >&2; }
die() { printf '[dev][error] %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
Usage: ./start-dev.sh [options]

  --start-netease-api  Start the pinned local Netease API package.
  --navidrome-local    Use http://127.0.0.1:4533 for Navidrome.
  --skip-browser       Do not open the frontend after readiness succeeds.
  --backend-only       Start only the Go backend.
  --frontend-only      Start only Vite; use VITE_BACKEND_URL for its backend.
  --env-file <path>    Load a local environment file instead of .env.local.
  -h, --help           Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --start-netease-api) START_NETEASE_API=true; shift ;;
    --navidrome-local) NAVIDROME_LOCAL=true; shift ;;
    --skip-browser) SKIP_BROWSER=true; shift ;;
    --backend-only) BACKEND_ONLY=true; shift ;;
    --frontend-only) FRONTEND_ONLY=true; shift ;;
    --env-file)
      [[ $# -ge 2 ]] || die "--env-file requires a path"
      ENV_FILE="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

if [[ "$BACKEND_ONLY" == true && "$FRONTEND_ONLY" == true ]]; then
  die "--backend-only and --frontend-only are mutually exclusive"
fi
if [[ "$BACKEND_ONLY" == true ]]; then
  SKIP_BROWSER=true
fi

[[ -f "$BACKEND_DIR/go.mod" ]] || die "missing backend-go/go.mod"
[[ -f "$FRONTEND_DIR/package.json" ]] || die "missing music-party-web/package.json"
command -v node >/dev/null 2>&1 || die "Node.js 22 is required"
command -v pnpm >/dev/null 2>&1 || die "pnpm 11.10.0 is required"
if [[ "$FRONTEND_ONLY" == false ]]; then
  command -v go >/dev/null 2>&1 || die "Go is required"
fi

NODE_MAJOR="$(node -p 'process.versions.node.split(".")[0]')"
[[ "$NODE_MAJOR" == 22 ]] || die "Node.js 22 is required (found $(node --version))"
PNPM_VERSION="$(pnpm --version)"
[[ "$PNPM_VERSION" == 11.10.0 ]] || die "pnpm 11.10.0 is required (found $PNPM_VERSION)"

if [[ -n "$ENV_FILE" ]]; then
  [[ -f "$ENV_FILE" ]] || { [[ "$ENV_FILE" == "$ROOT_DIR/.env.local" ]] || die "env file not found: $ENV_FILE"; }
  if [[ -f "$ENV_FILE" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
    info "loaded local environment: $ENV_FILE"
  fi
fi

read_cookie_field() {
  local field="$1"
  [[ -f "$COOKIE_FILE" ]] || return 0
  node - "$COOKIE_FILE" "$field" <<'NODE'
const fs = require('node:fs');
const [file, field] = process.argv.slice(2);
try {
  const data = JSON.parse(fs.readFileSync(file, 'utf8'));
  const value = field.split('.').reduce((item, key) => item?.[key], data);
  if (typeof value === 'string') process.stdout.write(value);
} catch {
  process.stderr.write('[dev][warn] cookies.json could not be parsed\n');
}
NODE
}

export NETEASE_COOKIE="${NETEASE_COOKIE:-$(read_cookie_field neteaseCookie)}"
export BILIBILI_SESSDATA="${BILIBILI_SESSDATA:-$(read_cookie_field bilibiliSessdata)}"
export NAVIDROME_BASE_URL="${NAVIDROME_BASE_URL:-$(read_cookie_field navidrome.baseUrl)}"
export NAVIDROME_USERNAME="${NAVIDROME_USERNAME:-$(read_cookie_field navidrome.username)}"
export NAVIDROME_PASSWORD=[REDACTED] navidrome.password)}"
if [[ "$NAVIDROME_LOCAL" == true ]]; then
  export NAVIDROME_ENABLED=true
  export NAVIDROME_BASE_URL="${NAVIDROME_BASE_URL:-http://127.0.0.1:4533}"
fi

mkdir -p "$LOG_DIR" "$RUNTIME_DIR" "$PID_DIR"
rm -f "$PID_DIR"/*.pid

cleanup() {
  local pid
  for pid in "${STARTED_PIDS[@]:-}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  wait >/dev/null 2>&1 || true
  rm -f "$PID_DIR"/*.pid
}
trap cleanup EXIT INT TERM

start_process() {
  local name="$1"
  local workdir="$2"
  shift 2
  (
    cd "$workdir"
    exec "$@"
  ) >>"$LOG_DIR/$name.log" 2>&1 &
  local pid=$!
  STARTED_PIDS+=("$pid")
  printf '%s\n' "$pid" >"$PID_DIR/$name.pid"
  info "started $name (pid $pid, log $LOG_DIR/$name.log)"
}

wait_for_url() {
  local url="$1"
  local name="$2"
  local attempt
  for attempt in $(seq 1 60); do
    if curl --fail --silent --show-error "$url" >/dev/null 2>&1; then
      info "$name is ready"
      return 0
    fi
    sleep 1
  done
  die "$name did not become ready; inspect $LOG_DIR"
}

if [[ "$START_NETEASE_API" == true ]]; then
  API_PORT="$NETEASE_API_PORT" start_process netease-api "$ROOT_DIR" "$ROOT_DIR/start-netease-api.sh"
  wait_for_url "http://127.0.0.1:$NETEASE_API_PORT" "Netease API"
fi

if [[ "$FRONTEND_ONLY" == false ]]; then
  export SERVER_PORT="$BACKEND_PORT"
  export BASE_URL="${BASE_URL:-http://127.0.0.1:$BACKEND_PORT}"
  export ALLOWED_ORIGINS="${ALLOWED_ORIGINS:-http://127.0.0.1:$FRONTEND_PORT,http://localhost:$FRONTEND_PORT}"
  export DB_ENABLED="${DB_ENABLED:-true}"
  export DB_INIT_SCHEMA="${DB_INIT_SCHEMA:-true}"
  export DB_PATH="${DB_PATH:-$RUNTIME_DIR/musicparty.db}"
  export STATIC_PATH="${STATIC_PATH:-$ROOT_DIR/music-party-web/dist}"
  export AUTH_SECURE_COOKIES="${AUTH_SECURE_COOKIES:-false}"
  export NETEASE_API_URL="${NETEASE_API_URL:-http://127.0.0.1:$NETEASE_API_PORT}"
  start_process backend "$ROOT_DIR" go run "$BACKEND_DIR/cmd/musicparty"
  wait_for_url "http://127.0.0.1:$BACKEND_PORT/actuator/health/readiness" "Go backend"
fi

if [[ "$BACKEND_ONLY" == false ]]; then
  export VITE_BACKEND_URL="${VITE_BACKEND_URL:-http://127.0.0.1:$BACKEND_PORT}"
  start_process frontend "$ROOT_DIR" pnpm --dir "$FRONTEND_DIR" dev --host 127.0.0.1 --port "$FRONTEND_PORT" --strictPort
  wait_for_url "http://127.0.0.1:$FRONTEND_PORT" "Vite frontend"
fi

if [[ "$SKIP_BROWSER" == false && "$BACKEND_ONLY" == false ]]; then
  URL="http://127.0.0.1:$FRONTEND_PORT"
  if command -v cmd.exe >/dev/null 2>&1; then
    cmd.exe //c start "" "$URL" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$URL" >/dev/null 2>&1 || true
  fi
fi

info "development processes are running; press Ctrl+C to stop only these child processes"
wait
