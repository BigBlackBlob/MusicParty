#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$ROOT_DIR/.dev-logs"
API_PORT="${API_PORT:-3000}"
COOKIE_FILE="$ROOT_DIR/cookies.json"
LOG_FILE="$LOG_DIR/netease-api.log"

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

NETEASE_COOKIE="${NETEASE_COOKIE:-$(read_json_field "$COOKIE_FILE" neteaseCookie)}"

{
  echo "========== $(date '+%Y-%m-%d %H:%M:%S') netease api start =========="
  echo "root: $ROOT_DIR"
  echo "cookie-config-path: $COOKIE_FILE"
  echo "effective-netease-cookie-length: ${#NETEASE_COOKIE}"
  echo "port: $API_PORT"
} >> "$LOG_FILE"

export PORT="$API_PORT"
export NETEASE_COOKIE

echo "Starting NeteaseCloudMusicApi on http://127.0.0.1:$API_PORT"
echo "Log: $LOG_FILE"
exec npx -y NeteaseCloudMusicApi@latest 2>&1 | tee -a "$LOG_FILE"
