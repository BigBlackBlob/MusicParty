# MusicParty Go backend

This directory contains the compatibility-first Go rewrite. It is a modular monolith and remains isolated from the Spring Boot implementation until the Java baseline tag and executable contracts are frozen.

Current implemented slice:

- Go 1.26 module with pinned runtime and test dependencies.
- Environment-variable loading for the current server configuration surface.
- Chi HTTP lifecycle with graceful shutdown.
- Actuator-compatible health, liveness, readiness, info, metrics, and Prometheus paths.
- Request IDs, structured access logs, bounded HTTP timeouts, Java-compatible media CORS, CSRF, trusted-proxy resolution, and cookie primitives.
- Production startup validation, unified error mapping, and WebSocket origin validation.
- Pure-Go SQLite storage with one writer connection, a two-connection read-only pool, WAL, foreign keys, busy timeout, and integrity checks.
- Capacity-100 serialized write queue with commit-before-return, rollback, cancellation, close, and queue-full behavior under test.
- Java-initialized 23-table schema snapshot and deterministic SQL/JSON/SHA-256 generator under `contracts/db/`.
- Semantic schema compatibility checks for canonical and legacy-upgraded Java databases, while still rejecting missing tables, columns, primary keys, or explicit indexes.
- Typed repositories covering all 23 frozen tables: rooms, accounts, access control, queue/history/playback/chat, room and user playlists, settings, Subsonic sources, and local media.
- Consistent `VACUUM INTO` database snapshots and Java→Go→Java→Go round-trip coverage for fresh, legacy-upgraded, and current application-database copies.
- Process startup integration that initializes only an explicitly allowed empty database, validates every non-empty database without migrating it, and closes storage during graceful shutdown.
- Generated OpenAPI/config/WebSocket contracts plus an isolated Java black-box launcher and strict golden comparator under `contracts/`.
- Password hashing, WebSocket envelope/accept primitives, and bounded concurrency helper.
- Stage 4 query API routes for config, platform discovery, music/album/user search, playlists, lyrics, and cover-color extraction.
- Long-lived, bounded HTTP clients with connection/header/operation timeouts, context cancellation, transient retry, `Retry-After`, response-size limits, and strict JSON decoding.
- Fixture-tested Netease mapping, Bilibili WBI signing and favorites, YouTube Data API search, Navidrome/Subsonic token authentication, encrypted stored Subsonic credentials, room-bound dynamic sources, and local-library search.
- Stage 5 account/session/profile flows, one-time invitation redemption, room membership/owner management, user and room playlists, and persisted Subsonic administration.
- Java-compatible private-room playlist authorization using the signed `roomAccessToken`, including expiry and password-version invalidation.
- Atomic playlist batch/import writes through the single SQLite writer; dynamic Subsonic and Navidrome access changes take effect without restart and are restored from SQLite on restart.
- Stage 6 per-room runtimes, WebSocket Hub, queue/playback/chat commands, mutation de-duplication, versioned state, resync, bounded reliable/latest-only delivery, and slow-client removal.
- Stage 7 local-library upload, duplicate detection, bounded FFmpeg transcode workers, ffprobe duration extraction, embedded-cover extraction, authenticated media/cover delivery, and deletion cleanup.
- Streaming proxies for Netease, Bilibili, YouTube/yt-dlp, Navidrome, and room-bound Subsonic sources with Range forwarding, cancellation propagation, bounded background caching, atomic `.part` promotion, restart discovery, and LRU space reclamation.
- Static Vue distribution serving with SPA fallback, including a multi-stage Docker build that packages the frontend with the Go process.

The live WebSocket state machine and playlist enqueue are implemented through the stage 6 room runtime. The legacy Java HTTP Radio feature is intentionally outside the Go product scope; ordinary player-state payloads retain `streamListenerCount` with the constant value `0`. Passkeys, administrator elevation, and offline recovery are not implemented because the frozen Java baseline does not yet provide those guarantees.

The first-release external platform gates are Netease and Bilibili. The local library is also a required release gate. YouTube, Navidrome, Squidify, and dynamically configured Subsonic sources are conditional capabilities and become release gates only when the target deployment explicitly enables them. The detailed acceptance boundary is recorded in `docs/go-rewrite-launch-platform-scope-2026-08-01.md`.

## Local verification

```powershell
cd backend-go
gofmt -w .
go mod tidy
go test ./...
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
go build ./...
```

The formal workflow is `.github/workflows/go-backend.yml`. It runs Windows and Linux verification, Linux race detection, CGO-free cross-build, Go vulnerability analysis, Docker build, and Trivy image scanning. The production SQLite driver remains CGO-free.

Stage 9 production preparation is under `deploy/`. It adds a manual immutable-image release workflow, a Go Compose override that disables first-cutover schema initialization, and guarded snapshot/integrity tooling. These artifacts do not deploy or modify a production host by themselves.

Regenerate the database contract only from an isolated database created by the current Java `SqliteSchemaInitializer`:

```powershell
go run ./cmd/schemasnapshot -db C:\path\to\isolated-java.db -out ..\contracts\db
```

The opt-in round-trip test never targets the normal application database:

```powershell
$env:MUSICPARTY_JAVA_SQLITE_FIXTURE = 'C:\path\to\isolated-java.db'
$env:MUSICPARTY_JAVA_SQLITE_PHASE = 'seed-go' # or verify-java after the Java probe
go test -run TestSQLiteJavaRoundTrip ./integration
```

Run the empty service:

```powershell
$env:SERVER_PORT = '18081'
go run ./cmd/musicparty
```

Probe it with:

```powershell
Invoke-RestMethod http://127.0.0.1:18081/actuator/health
Invoke-WebRequest http://127.0.0.1:18081/actuator/prometheus
```

## Compatibility boundary

Until `go-rewrite-baseline-v1` exists, values in this directory are implementation scaffolding rather than proof of Java equivalence. The baseline contract suite remains authoritative once frozen; discrepancies must be fixed in Go rather than normalized away.

Stage 3 is complete at the repository and SQLite-compatibility layer. All 23 frozen tables are covered; fresh Java databases, a legacy-upgraded fixture, and a read-only consistent copy of the current application database have completed Java→Go→Java→Go checks with `integrity_check=ok` and no foreign-key violations. The normal application database is never an allowed test target, and no potentially sensitive data copy is committed.

Stages 4 through 7 are implemented locally. Stage 8 acceptance covers strict Java/Go HTTP and WebSocket golden comparison, authoritative playable metadata, real Netease preview playback and seek, two-browser restart recovery, a 1000-item queue, the media fault matrix, 10/100/300 WebSocket 30-minute load, same-hardware Java/Go measurements, race/static/vulnerability gates, and a read-only non-root Trivy-clean runtime image. A clean Dockerfile rebuild also passed during stage 9 preparation. Final candidate acceptance still requires credentialed full-flow Netease and Bilibili checks plus the local-library flow; conditional platforms are checked only when enabled. Evidence is under `acceptance/stage8/`.
