# Java backend retirement

The Java/Spring backend stopped being an active MusicParty runtime when the Go-only source and delivery pipeline landed on `NRT-Base`.

- Go owns HTTP, WebSocket, configuration and SQLite evolution.
- Maven, Java sources, Java tests, launchers and Java image workflows are absent from the active branch.
- Historical Git tags, Actions runs and already-published Java images remain read-only evidence.
- Historical Java images receive no fixes and are not supported rollback targets.
- Rollback uses the previous known-good Go digest and, when schema changes are involved, a migration-time SQLite snapshot.
- Frozen schemas and synthetic compatibility fixtures remain under `contracts/db/`; they contain no production credentials or user data.

The concise migration record is in `docs/migration-archive-summary.md`. No new Stage 8 or Stage 9 candidate manifests should be created.
