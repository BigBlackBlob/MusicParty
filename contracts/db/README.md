# SQLite compatibility contract

This directory preserves the database shapes that existed when Go became MusicParty's sole schema owner.

- `schema.sql` is the deterministic executable frozen schema.
- `schema.json` describes tables, indexes, columns and foreign keys.
- `schema.sha256` fingerprints the exact `schema.sql` bytes.
- `fixtures/legacy-upgraded/` represents an accepted database upgraded by the retired runtime.

The frozen snapshot contains 23 application tables. It is a compatibility input, not an active Java dependency and not by itself a runtime rejection rule. Go validates required tables, columns, primary keys and explicit indexes, and compatibility tests exercise repository reads, writes and reopen behavior.

Do not generate contracts from a production or local runtime database. Initialize an isolated database with the current Go schema/migration code or use a frozen migration fixture, close the writer, then run from `backend-go/`:

```powershell
go run ./cmd/schemasnapshot -db C:\path\to\isolated-fixture.db -out ..\contracts\db
go test -run TestContractSchemaIsSelfConsistentAndReproducible ./internal/store/sqlite
```

Snapshots contain schema only. Fixtures must never contain credentials, cookies, session tokens or personal data. Future schema changes belong to versioned Go migrations and must prove frozen-schema-to-current compatibility.
