#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$ROOT_DIR/.dev-logs"
QQ_SIDECAR_PORT="${QQ_SIDECAR_PORT:-3100}"
LOG_FILE="$LOG_DIR/qq-sidecar.log"
mkdir -p "$LOG_DIR"
echo "========== $(date '+%Y-%m-%d %H:%M:%S') qq-music-sidecar start ==========" >> "$LOG_FILE"
echo "port: $QQ_SIDECAR_PORT" >> "$LOG_FILE"
echo "Starting QQ Music sidecar on http://127.0.0.1:$QQ_SIDECAR_PORT"
echo "Log: $LOG_FILE"
export QQ_SIDECAR_PORT
cd "$ROOT_DIR/qq-music-sidecar"
exec node index.js 2>&1 | tee -a "$LOG_FILE"
