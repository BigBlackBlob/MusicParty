# MusicParty contracts

Go is the sole owner of active HTTP, WebSocket, environment and SQLite contracts.

## Generated contracts

- `http/routes.json` and `http/openapi.yaml`: routes registered by the Go HTTP server.
- `ws/client-messages.schema.json` and `ws/server-messages.schema.json`: typed realtime envelopes.
- `ws/scenarios/catalog.json`: stable Go-only protocol scenarios.
- `config/environment.yaml`: variables consumed by Go configuration, including secret and deprecation metadata.
- `music-party-web/src/contracts/generated/`: TypeScript types and contract version consumed by the frontend.

Regenerate or check drift from `backend-go/`:

```powershell
go run ./cmd/contractgen -repo ..
go run ./cmd/contractgen -check -repo ..
go test ./internal/contractspec ./integration
```

## SQLite compatibility

`db/schema.sql`, `db/schema.json`, `db/schema.sha256`, `db/fixtures/legacy-upgraded/` and the embedded required schema preserve compatibility with databases created before Go became the schema owner. They are compatibility fixtures, not a Java runtime dependency.

Never run contract tooling against `music_party/data/musicparty.db` or a production service.
