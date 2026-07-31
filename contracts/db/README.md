# SQLite compatibility contract

These files are generated from a fresh, isolated database initialized by the current Java `SqliteSchemaInitializer` and its `db/schema.sql` resource:

- `schema.sql` is the deterministic, executable schema reconstructed from `sqlite_master`.
- `schema.json` records every non-internal table/index/trigger/view plus table columns, foreign keys, implicit indexes, index columns, collations, sort direction, and partial-index flags.
- `schema.sha256` fingerprints the exact bytes of `schema.sql`.

Current snapshot: 23 application tables and 45 explicit `sqlite_master` objects. `admin_bootstrap_claim` is migration-created and is why copying only `src/main/resources/db/schema.sql` would be incomplete.

The SHA-256 identifies the canonical fresh-database schema; it is not, by itself, a runtime rejection rule. A database upgraded through older Java migrations can have equivalent required columns but different original `CREATE TABLE` text or foreign-key metadata. Those variants must be admitted through explicit old-database fixtures and round-trip tests rather than by weakening the fresh-schema snapshot.

`fixtures/legacy-upgraded/` captures that second accepted shape. Its raw hash differs from the fresh schema, while `ValidateSchemaCompatibility` verifies all required tables, columns, primary keys, and explicit indexes. Missing historical foreign-key metadata is tolerated because SQLite cannot add those constraints with `ALTER TABLE`; missing columns or indexes are not tolerated.

Do not generate this contract from `music_party/data/musicparty.db`. Create a new temporary database with Java, close Java, and then run from `backend-go/`:

```powershell
go run ./cmd/schemasnapshot -db C:\path\to\isolated-java.db -out ..\contracts\db
go test -run TestContractSchemaIsSelfConsistentAndReproducible ./internal/store/sqlite
```

The snapshot intentionally contains schema only. Production-like, anonymized data fixtures and old-database migration fixtures remain separate acceptance assets and must not contain credentials, cookies, session tokens, or personal data.
