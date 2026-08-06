# MusicParty Go backend

`backend-go/` is the only supported MusicParty backend. It is a Go modular monolith built around Chi, typed domain services, a serialized SQLite writer, room-scoped realtime runtimes, media proxies and the bundled Vue frontend.

## Local verification

```powershell
cd backend-go
./ci/verify.sh
```

The generated HTTP, WebSocket and environment contracts are owned by Go. The SQLite compatibility suite keeps the frozen 23-table production schema and legacy-upgraded fixture readable without invoking a Java process.

## Run locally

```powershell
$env:SERVER_PORT = '18081'
$env:DB_PATH = "$PWD\data\musicparty.db"
$env:DB_INIT_SCHEMA = 'true'
go run ./cmd/musicparty
```

Health endpoints:

```text
GET /actuator/health
GET /actuator/health/liveness
GET /actuator/health/readiness
GET /actuator/prometheus
```

## Database tools

- `go run ./cmd/dbcheck -db <path>` validates integrity, foreign keys and required schema.
- `go run ./cmd/dbsnapshot -source <path> -destination <path>` creates a consistent SQLite snapshot.
- `go test -run TestSQLiteFrozenSchemaCompatibility ./integration` verifies repository reads, writes and reopen behavior against the frozen schema.

Never point tests or contract generators at the production database.

## Delivery

The root `Dockerfile` is the only image definition. `.github/workflows/ci.yml` verifies the application and publishes a single verified image to GHCR and Aliyun ACR after a green `NRT-Base` push. See `docs/release-process.md` and `backend-go/deploy/README.md`.

The former Stage 8/9 candidate directories are summarized in `docs/migration-archive-summary.md`; they are not active release gates.
