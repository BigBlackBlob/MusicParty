#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/music-party-web"
LOG_DIR="$ROOT_DIR/.dev-logs"
COOKIE_FILE="$ROOT_DIR/cookies.json"
ENV_FILE="$ROOT_DIR/.env.local"

for ((i = 1; i <= $#; i++)); do
  if [[ "${!i}" == "--env-file" ]]; then
    next=$((i + 1))
    ENV_FILE="${!next:-}"
  fi
done

if [[ -n "$ENV_FILE" && -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

START_NETEASE_API=false
BACKEND_PORT=8080
ADMIN_PASSWORD="${ADMIN_PASSWORD:-dev-admin-pass-2026}"
FRONTEND_HOST=127.0.0.1
FRONTEND_PORT=5173
NETEASE_API_PORT=3000
NETEASE_API_URL="${NETEASE_API_URL:-}"
SKIP_BROWSER=false
NAVIDROME_ENABLED="${NAVIDROME_ENABLED:-false}"
NAVIDROME_BASE_URL="${NAVIDROME_BASE_URL:-}"
NAVIDROME_USERNAME="${NAVIDROME_USERNAME:-}"
NAVIDROME_PASSWORD="${NAVIDROME_PASSWORD:-}"
NAVIDROME_ALLOWED_USERS="${NAVIDROME_ALLOWED_USERS:-}"
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
  --env-file <path>             Load local environment file. Default: ./.env.local if present.
  --navidrome-local             Enable Navidrome at http://127.0.0.1:4533; credentials come from env file or env vars.
  --navidrome-base-url <url>    Enable Navidrome and use this base URL.
  --navidrome-username <name>   Navidrome username.
  --navidrome-password <pass>   Navidrome password.
  --navidrome-allowed-users <names>
                                MusicParty usernames allowed to use Navidrome, comma-separated.
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
    --env-file)
      ENV_FILE="${2:-}"
      [[ -n "$ENV_FILE" ]] || die "--env-file requires a value"
      shift 2
      ;;
    --navidrome-local)
      NAVIDROME_ENABLED=true
      NAVIDROME_BASE_URL="${NAVIDROME_BASE_URL:-http://127.0.0.1:4533}"
      shift
      ;;
    --navidrome-base-url)
      NAVIDROME_ENABLED=true
      NAVIDROME_BASE_URL="${2:-}"
      [[ -n "$NAVIDROME_BASE_URL" ]] || die "--navidrome-base-url requires a value"
      shift 2
      ;;
    --navidrome-username)
      NAVIDROME_USERNAME="${2:-}"
      [[ -n "$NAVIDROME_USERNAME" ]] || die "--navidrome-username requires a value"
      shift 2
      ;;
    --navidrome-password)
      NAVIDROME_PASSWORD="${2:-}"
      [[ -n "$NAVIDROME_PASSWORD" ]] || die "--navidrome-password requires a value"
      shift 2
      ;;
    --navidrome-allowed-users)
      NAVIDROME_ALLOWED_USERS="${2:-}"
      [[ -n "$NAVIDROME_ALLOWED_USERS" ]] || die "--navidrome-allowed-users requires a value"
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

ensure_java_home() {
  local candidate=""

  if [[ -n "${JAVA_HOME:-}" && -x "$JAVA_HOME/bin/java" && -x "$JAVA_HOME/bin/javac" ]]; then
    return 0
  fi

  for candidate in \
    /c/Program\ Files/Microsoft/jdk-* \
    /c/Program\ Files/Java/jdk-* \
    /c/Users/"$USERNAME"/Downloads/graalvm-jdk-*/graalvm-jdk-*; do
    if [[ -x "$candidate/bin/java" && -x "$candidate/bin/javac" ]]; then
      export JAVA_HOME="$candidate"
      export PATH="$JAVA_HOME/bin:$PATH"
      info "detected JAVA_HOME: $JAVA_HOME"
      return 0
    fi
  done

  if command -v java >/dev/null 2>&1 && command -v javac >/dev/null 2>&1; then
    return 0
  fi

  die "java/javac not found. Set JAVA_HOME or add a JDK bin directory to PATH."
}

ensure_java_home

if command -v mvn >/dev/null 2>&1; then
  MVN_CMD=(mvn)
elif [[ -x "$ROOT_DIR/mvnw" ]]; then
  MVN_CMD=("$ROOT_DIR/mvnw")
else
  die "mvn or ./mvnw is required"
fi

mkdir -p "$LOG_DIR"
mkdir -p "$LOG_DIR/tmp"
export JAVA_TOOL_OPTIONS="${JAVA_TOOL_OPTIONS:-}"
if [[ "$JAVA_TOOL_OPTIONS" != *"-Djava.io.tmpdir="* ]]; then
  if [[ -n "$JAVA_TOOL_OPTIONS" ]]; then
    export JAVA_TOOL_OPTIONS="-Djava.io.tmpdir=$LOG_DIR/tmp $JAVA_TOOL_OPTIONS"
  else
    export JAVA_TOOL_OPTIONS="-Djava.io.tmpdir=$LOG_DIR/tmp"
  fi
fi

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

check_navidrome() {
  node -e "
const crypto = require('crypto');
const baseUrl = process.env.NAVIDROME_BASE_URL;
const username = process.env.NAVIDROME_USERNAME;
const password = process.env.NAVIDROME_PASSWORD;
if (!baseUrl || !username || !password) {
  console.error('missing Navidrome base URL, username, or password');
  process.exit(2);
}
const salt = crypto.randomBytes(4).toString('hex');
const token = crypto.createHash('md5').update(password + salt).digest('hex');
const url = new URL('/rest/ping.view', baseUrl);
url.searchParams.set('u', username);
url.searchParams.set('s', salt);
url.searchParams.set('t', token);
url.searchParams.set('v', process.env.NAVIDROME_API_VERSION || '1.16.1');
url.searchParams.set('c', process.env.NAVIDROME_CLIENT || 'musicparty-dev');
url.searchParams.set('f', 'json');
fetch(url)
  .then(async response => {
    if (!response.ok) throw new Error('HTTP ' + response.status);
    const body = await response.json();
    const status = body?.['subsonic-response']?.status;
    if (status !== 'ok') {
      const error = body?.['subsonic-response']?.error;
      throw new Error(error?.message || 'Subsonic ping failed');
    }
  })
  .catch(error => {
    console.error(error.message);
    process.exit(1);
  });
"
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
  echo "navidrome-enabled: $NAVIDROME_ENABLED"
  echo "navidrome-base-url: $NAVIDROME_BASE_URL"
  echo "navidrome-allowed-users: $NAVIDROME_ALLOWED_USERS"
} >> "$BACKEND_LOG"

info "root: $ROOT_DIR"
info "logs: $LOG_DIR"
if [[ -n "$ENV_FILE" && -f "$ENV_FILE" ]]; then
  info "env file: $ENV_FILE"
fi
info "netease cookie length: ${#NETEASE_COOKIE}"
info "netease api url: $NETEASE_API_URL"
info "navidrome enabled: $NAVIDROME_ENABLED"
if [[ "$NAVIDROME_ENABLED" == true ]]; then
  info "navidrome base url: $NAVIDROME_BASE_URL"
  info "navidrome allowed users: $NAVIDROME_ALLOWED_USERS"
fi

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

if [[ "$NAVIDROME_ENABLED" == true ]]; then
  info "checking Navidrome connectivity"
  if ! NAVIDROME_BASE_URL="$NAVIDROME_BASE_URL" \
      NAVIDROME_USERNAME="$NAVIDROME_USERNAME" \
      NAVIDROME_PASSWORD="$NAVIDROME_PASSWORD" \
      NAVIDROME_API_VERSION="${NAVIDROME_API_VERSION:-1.16.1}" \
      NAVIDROME_CLIENT="${NAVIDROME_CLIENT:-musicparty-dev}" \
      check_navidrome; then
    warn "Navidrome precheck failed; backend will still start"
    warn "check base URL, credentials, and that the Windows Navidrome service is running"
  else
    info "Navidrome ping succeeded"
  fi
fi

info "starting backend in background"
(
  cd "$ROOT_DIR"
  SERVER_PORT="$BACKEND_PORT" \
    ADMIN_PASSWORD="$ADMIN_PASSWORD" \
    NETEASE_API_URL="$NETEASE_API_URL" \
    NETEASE_COOKIE="$NETEASE_COOKIE" \
    BILIBILI_SESSDATA="$BILIBILI_SESSDATA" \
    NAVIDROME_ENABLED="$NAVIDROME_ENABLED" \
    NAVIDROME_BASE_URL="$NAVIDROME_BASE_URL" \
    NAVIDROME_USERNAME="$NAVIDROME_USERNAME" \
    NAVIDROME_PASSWORD="$NAVIDROME_PASSWORD" \
    NAVIDROME_CLIENT="${NAVIDROME_CLIENT:-musicparty}" \
    NAVIDROME_API_VERSION="${NAVIDROME_API_VERSION:-1.16.1}" \
    NAVIDROME_ALLOWED_USERS="$NAVIDROME_ALLOWED_USERS" \
    "${MVN_CMD[@]}" spring-boot:run 2>&1 | tee -a "$BACKEND_LOG"
) &
STARTED_PIDS+=("$!")

if ! wait_for_port 127.0.0.1 "$BACKEND_PORT" "Backend" 60; then
  warn "backend did not open port $BACKEND_PORT within 60s"
  warn "check $BACKEND_LOG"
fi

info "starting frontend in background"
(
  cd "$FRONTEND_DIR"
  VITE_BACKEND_URL="http://127.0.0.1:$BACKEND_PORT" \
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
