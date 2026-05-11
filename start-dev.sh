#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/music-party-web"
LOG_DIR="$ROOT_DIR/.dev-logs"
COOKIE_FILE="$ROOT_DIR/cookies.json"

START_NETEASE_API=false
BACKEND_PORT=8080
FRONTEND_HOST=127.0.0.1
FRONTEND_PORT=5173
NETEASE_API_PORT=3000
NETEASE_API_URL="${NETEASE_API_URL:-}"
SKIP_BROWSER=false
STARTED_PIDS=()

info() {
  printf '[dev] %s\n' "$*"
}

warn() {
  printf '[dev][warn] %s\n' "$*" >&2
}

die() {
  printf '[dev][error] %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  ./start-dev.sh [options]

Options:
  --start-netease-api           Start local NeteaseCloudMusicApi on port 3000.
  --backend-port <port>         Backend port. Default: 8080.
  --frontend-port <port>        Frontend port. Default: 5173.
  --api-port <port>             Local Netease API port. Default: 3000.
  --netease-api-url <url>       Use an existing Netease API URL instead of http://127.0.0.1:<api-port>.
  --skip-browser                Do not open frontend URL after startup.
  -h, --help                    Show this help.

Cookie:
  The script reads ./cookies.json if present:
  {
    "neteaseCookie": "MUSIC_U=xxxx...; __csrf=xxxx...",
    "bilibiliSessdata": ""
  }
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --start-netease-api)
      START_NETEASE_API=true
      shift
      ;;
    --backend-port)
      BACKEND_PORT="${2:-}"
      [[ -n "$BACKEND_PORT" ]] || die "--backend-port requires a value"
      shift 2
      ;;
    --frontend-port)
      FRONTEND_PORT="${2:-}"
      [[ -n "$FRONTEND_PORT" ]] || die "--frontend-port requires a value"
      shift 2
      ;;
    --api-port)
      NETEASE_API_PORT="${2:-}"
      [[ -n "$NETEASE_API_PORT" ]] || die "--api-port requires a value"
      shift 2
      ;;
    --netease-api-url)
      NETEASE_API_URL="${2:-}"
      [[ -n "$NETEASE_API_URL" ]] || die "--netease-api-url requires a value"
      shift 2
      ;;
    --skip-browser)
      SKIP_BROWSER=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "Unknown option: $1"
      ;;
  esac
done

command -v node >/dev/null 2>&1 || die "node is required"
command -v npm >/dev/null 2>&1 || die "npm is required"
command -v npx >/dev/null 2>&1 || die "npx is required"

if command -v mvn >/dev/null 2>&1; then
  MVN_CMD=(mvn)
elif [[ -x "$ROOT_DIR/mvnw" ]]; then
  MVN_CMD=("$ROOT_DIR/mvnw")
else
  die "mvn or ./mvnw is required"
fi

mkdir -p "$LOG_DIR"

read_json_field() {
  local file="$1"
  local field="$2"

  if [[ ! -f "$file" ]]; then
    return 0
  fi

  node -e "
const fs = require('fs');
const file = process.argv[1];
const field = process.argv[2];
try {
  const data = JSON.parse(fs.readFileSync(file, 'utf8'));
  const value = data[field] || '';
  process.stdout.write(String(value));
} catch (error) {
  process.stderr.write('Failed to read ' + file + ': ' + error.message + '\n');
  process.exit(2);
}
" "$file" "$field"
}

port_open() {
  local host="$1"
  local port="$2"

  node -e "
const net = require('net');
const socket = net.createConnection({ host: process.argv[1], port: Number(process.argv[2]), timeout: 800 });
socket.on('connect', () => { socket.destroy(); process.exit(0); });
socket.on('timeout', () => { socket.destroy(); process.exit(1); });
socket.on('error', () => process.exit(1));
" "$host" "$port" >/dev/null 2>&1
}

wait_for_port() {
  local host="$1"
  local port="$2"
  local name="$3"
  local attempts="${4:-45}"

  for ((i = 1; i <= attempts; i++)); do
    if port_open "$host" "$port"; then
      info "$name is listening on $host:$port"
      return 0
    fi
    sleep 1
  done

  return 1
}

cleanup() {
  if [[ ${#STARTED_PIDS[@]} -gt 0 ]]; then
    info "stopping child processes: ${STARTED_PIDS[*]}"
    kill "${STARTED_PIDS[@]}" >/dev/null 2>&1 || true
  fi
}

NETEASE_COOKIE="${NETEASE_COOKIE:-$(read_json_field "$COOKIE_FILE" neteaseCookie)}"
BILIBILI_SESSDATA="${BILIBILI_SESSDATA:-$(read_json_field "$COOKIE_FILE" bilibiliSessdata)}"

if [[ -z "$NETEASE_API_URL" ]]; then
  NETEASE_API_URL="http://127.0.0.1:$NETEASE_API_PORT"
fi

BACKEND_LOG="$LOG_DIR/backend-dev.log"
FRONTEND_LOG="$LOG_DIR/frontend-dev.log"
NETEASE_LOG="$LOG_DIR/netease-api.log"

{
  echo "========== $(date '+%Y-%m-%d %H:%M:%S') dev start =========="
  echo "root: $ROOT_DIR"
  echo "cookie-config-path: $COOKIE_FILE"
  echo "effective-netease-cookie-length: ${#NETEASE_COOKIE}"
  echo "effective-bilibili-sessdata-length: ${#BILIBILI_SESSDATA}"
  echo "netease-api-url: $NETEASE_API_URL"
  echo "backend-port: $BACKEND_PORT"
  echo "frontend-url: http://$FRONTEND_HOST:$FRONTEND_PORT"
} >> "$BACKEND_LOG"

info "root: $ROOT_DIR"
info "logs: $LOG_DIR"
info "netease cookie length: ${#NETEASE_COOKIE}"
info "netease api url: $NETEASE_API_URL"

trap cleanup EXIT INT TERM

if [[ "$START_NETEASE_API" == true ]]; then
  if port_open 127.0.0.1 "$NETEASE_API_PORT"; then
    warn "port $NETEASE_API_PORT is already open; not starting another Netease API"
  else
    info "starting Netease API in background"
    (
      cd "$ROOT_DIR"
      API_PORT="$NETEASE_API_PORT" NETEASE_COOKIE="$NETEASE_COOKIE" ./start-netease-api.sh
    ) &
    STARTED_PIDS+=("$!")
  fi

  if ! wait_for_port 127.0.0.1 "$NETEASE_API_PORT" "Netease API" 60; then
    warn "Netease API did not open port $NETEASE_API_PORT within 60s"
    warn "check $NETEASE_LOG"
  fi
else
  if ! port_open 127.0.0.1 "$NETEASE_API_PORT"; then
    warn "local Netease API port $NETEASE_API_PORT is not open"
    warn "run ./start-dev.sh --start-netease-api or start your own API and pass --netease-api-url"
  fi
fi

info "starting backend in background"
(
  cd "$ROOT_DIR"
  SERVER_PORT="$BACKEND_PORT" \
    NETEASE_API_URL="$NETEASE_API_URL" \
    NETEASE_COOKIE="$NETEASE_COOKIE" \
    BILIBILI_SESSDATA="$BILIBILI_SESSDATA" \
    "${MVN_CMD[@]}" spring-boot:run 2>&1 | tee -a "$BACKEND_LOG"
) &
STARTED_PIDS+=("$!")

info "starting frontend in background"
(
  cd "$FRONTEND_DIR"
  npm run dev -- --host "$FRONTEND_HOST" --port "$FRONTEND_PORT" --strictPort 2>&1 | tee -a "$FRONTEND_LOG"
) &
STARTED_PIDS+=("$!")

if [[ "$SKIP_BROWSER" == false ]]; then
  FRONTEND_URL="http://$FRONTEND_HOST:$FRONTEND_PORT"
  if command -v cmd.exe >/dev/null 2>&1; then
    cmd.exe //c start "" "$FRONTEND_URL" >/dev/null 2>&1 || true
  fi
fi

cat <<EOF

Started dev processes in this Git Bash session.

Logs:
  Netease API: $NETEASE_LOG
  Backend:     $BACKEND_LOG
  Frontend:    $FRONTEND_LOG

URLs:
  Frontend:    http://$FRONTEND_HOST:$FRONTEND_PORT
  Backend:     http://127.0.0.1:$BACKEND_PORT
  Netease API: $NETEASE_API_URL

Press Ctrl+C in this terminal to stop processes started by this script.
EOF

wait
