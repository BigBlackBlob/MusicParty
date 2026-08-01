# MusicParty executable compatibility contracts

This directory freezes externally observable Java behavior for the Go rewrite. Generated assets are derived from the current Java controller, WebSocket dispatch, outbound broker calls, configuration, and isolated SQLite initialization.

## Assets

- `http/openapi.yaml`: OpenAPI 3.1 route/method inventory. JSON syntax is used because JSON is valid YAML and provides deterministic generation.
- `http/routes.json`: source-linked route inventory used to detect additions, removals, and method/path changes.
- `http/golden/scenarios.json`: deterministic Java black-box HTTP scenarios and the explicit normalization allowlist.
- `http/golden/java-baseline.json`: captured normalized Java responses. Status, field presence, array order, error body, cookie attributes, and selected headers remain strict.
- `http/golden/java-test-evidence.json`: supplemental deterministic Java evidence for binary media, Range/proxy, playlist, queue ACK/NACK, and upload/transcode paths.
- `ws/client-messages.schema.json` and `ws/server-messages.schema.json`: JSON Schema envelopes with the complete discovered message-type sets.
- `ws/scenarios/catalog.json`: handshake, close-code, resync, ping, identity/presence, queue NACK, Unicode chat/public chat, history, and reconnect ordering scenarios.
- `config/environment.yaml`: every `${ENV:default}` reference from `application.yml`, with source locations.
- `db/`: the independently verified SQLite contract from stage 3.

## Regeneration and drift checking

From `backend-go/`:

```powershell
go run ./cmd/contractgen -repo ..
go run ./cmd/contractgen -check -repo ..
go test ./integration
```

Capture the Java golden only from the isolated launcher. It creates a temporary database and local-library directory, starts the built JAR on a random loopback port, and deletes the temporary directory after shutdown:

```powershell
.\contracts\java\run-baseline.ps1 -Mode capture
.\contracts\java\run-baseline.ps1 -Mode compare
```

The comparison may replace only values on the declared normalization allowlist: timestamps, generated public IDs, session credentials, and cookie values. It must not normalize HTTP status, missing fields, array order, error bodies, cookie flags, selected header presence, WebSocket message order, or close codes.

The current generated inventory contains 91 HTTP operations, including Actuator and SPA/static behavior, 26 canonical client WebSocket message types with slash aliases, 18 discovered server message types, and 96 environment variables. The black-box golden covers authentication, cookies, CSRF, permissions, empty data, invalid parameters, upstream failure, invitations, duplicate redemption, membership removal, user playlists, ordering/export/likes, queue version NACK, Unicode chat, and reconnect/resync.

Never point these tools at `music_party/data/musicparty.db` or a running production server.

## Frozen baseline provenance

The generated assets and Java black-box golden were revalidated on 2026-08-01 against the published tag `go-rewrite-baseline-v1`, which resolves to commit `95b9f8da32e2375b7183e9d6233d8c45d0d1dfcd`.

The verification used a detached temporary worktree for generation and JAR construction, plus an isolated temporary SQLite database for the black-box comparison. All seven generated assets matched byte for byte, and the HTTP/WebSocket endpoint probe matched `http/golden/java-baseline.json`.

The local health probe explicitly bypasses system HTTP proxies because it only addresses the isolated Java process on `127.0.0.1`.
