#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="musicparty-verify-$(date +%s)-$$"
IMAGE_NAME="$PROJECT_NAME:local"
TEMP_DIR="$(mktemp -d)"
CONTAINER_NAME="$PROJECT_NAME-app"
HOST_PORT="${MUSICPARTY_VERIFY_PORT:-18848}"

cleanup() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  rm -rf "$TEMP_DIR"
}
trap cleanup EXIT INT TERM

command -v docker >/dev/null 2>&1 || { echo 'docker is required' >&2; exit 1; }
command -v pnpm >/dev/null 2>&1 || { echo 'pnpm is required' >&2; exit 1; }
docker version >/dev/null

cd "$ROOT_DIR"
NETEASE_API_IMAGE="binaryify/neteasecloudmusicapi:4.21.3" \
  MUSIC_PARTY_IMAGE="$IMAGE_NAME" \
  BASE_URL="http://127.0.0.1:$HOST_PORT" \
  ALLOWED_ORIGINS="http://127.0.0.1:$HOST_PORT" \
  BOOTSTRAP_ADMIN_USERNAME="e2e-admin" \
  BOOTSTRAP_ADMIN_PASSWORD="compose-config-placeholder" \
  docker compose -p "$PROJECT_NAME" config --quiet
docker build --label "org.opencontainers.image.revision=local-verification" -t "$IMAGE_NAME" -f Dockerfile .

mkdir -p "$TEMP_DIR/data" "$TEMP_DIR/cache"
ADMIN_PASSWORD="$(node -e "process.stdout.write(require('node:crypto').randomBytes(24).toString('hex'))")"
docker run -d --name "$CONTAINER_NAME" \
  -p "127.0.0.1:$HOST_PORT:8080" \
  -e APP_MODE=server \
  -e DB_PATH=/app/data/musicparty.db \
  -e DB_INIT_SCHEMA=true \
  -e STATIC_PATH=/app/static \
  -e BOOTSTRAP_ADMIN_USERNAME=e2e-admin \
  -e BOOTSTRAP_ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  -e AUTH_SECURE_COOKIES=false \
  -v "$TEMP_DIR/data:/app/data" \
  -v "$TEMP_DIR/cache:/app/cached_media" \
  "$IMAGE_NAME" >/dev/null

for _ in $(seq 1 60); do
  if curl --fail --silent "http://127.0.0.1:$HOST_PORT/actuator/health/readiness" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --fail --show-error "http://127.0.0.1:$HOST_PORT/actuator/health"
curl --fail --show-error "http://127.0.0.1:$HOST_PORT/actuator/health/readiness"

(
  cd "$ROOT_DIR/music-party-web"
  E2E_BASE_URL="http://127.0.0.1:$HOST_PORT" \
    E2E_ADMIN_USERNAME="e2e-admin" \
    E2E_ADMIN_PASSWORD="$ADMIN_PASSWORD" \
    pnpm test:e2e
)

echo "Go-only local verification passed. Temporary project: $PROJECT_NAME"
