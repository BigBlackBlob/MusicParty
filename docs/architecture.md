# MusicParty architecture

MusicParty is a Go modular monolith serving a Vue 3 application. Go is the sole runtime and contract owner.

## Runtime boundaries

```text
Browser
  ├─ typed HTTP client → Chi HTTP API → domain services → serialized SQLite writer
  ├─ typed WebSocket client → room runtime → authoritative player/queue/presence state
  └─ local audio element → buffering, volume and playback-position interpolation
```

The root Docker image contains the Go server, database tools, built frontend, FFmpeg and yt-dlp. It runs as non-root UID/GID `10001:10001` by default.

## Backend

- `backend-go/internal/httpapi`: route registration, request decoding and responses.
- `backend-go/internal/domain`: account, room, playback, chat, platform and media behavior.
- `backend-go/internal/realtime`: typed WebSocket routing and room-scoped runtime state.
- `backend-go/internal/store/sqlite`: schema initialization, validation and repositories.
- `backend-go/internal/config`: Go runtime configuration.
- `backend-go/internal/contractspec`: Go-owned HTTP, WebSocket and environment contract generation.

Authentication uses HttpOnly Cookie sessions. Guest, invited-member, room-owner and platform-administrator behavior is derived from the resolved session and capabilities, not display names or browser-readable tokens. Room-password access is a separate room-scoped Cookie concern.

## Frontend

- TanStack Vue Query owns HTTP server resources.
- Pinia owns session/context, realtime connection/runtime state, local audio, layout/preferences and transient UI feedback.
- Generated contracts under `music-party-web/src/contracts/generated/` type the transport boundary.
- Versioned WebSocket state rejects stale generations and resynchronizes queue-version gaps.
- The command bus correlates ACK/NACK and rolls back failed optimistic mutations.

The desktop free layout, mobile layout and Lite Mode consume the same domain state. UI components should not duplicate server resources or infer behavior from translated error messages.

## Persistence

SQLite is the single application database. A serialized writer limits contention while read connections serve queries. `contracts/db/` preserves the frozen 23-table compatibility input and a synthetic legacy-upgraded fixture. Future changes belong to versioned Go migrations; production rollback uses a pre-migration snapshot and a previous immutable Go image digest.

## Delivery

`CI` checks Go contracts, static analysis, tests/race, frontend checks, E2E, container readiness and vulnerability scanning. A green `NRT-Base` application-code push publishes the same immutable image to GHCR and Aliyun ACR. GitHub Actions never deploys the VPS automatically.
