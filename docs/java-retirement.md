# Java backend retirement

The Java/Spring backend stopped being an active MusicParty runtime when the Go-only delivery pipeline and source cleanup landed on `NRT-Base`.

- Go owns HTTP, WebSocket, configuration and SQLite evolution.
- Maven, Java source, Java tests, Java launchers and Java image workflows are removed from the active branch.
- Historical Git tags, including `legacy-java-master-20260802`, GitHub Actions runs and already published Java images remain read-only evidence.
- Historical images receive no fixes and are not supported production rollback targets.
- Supported rollback uses the previous known-good Go digest and, for schema changes, the migration-time SQLite snapshot.
- Frozen SQLite schemas and synthetic legacy fixtures remain in `contracts/db/` to protect existing installations. They contain no production credentials or user data.

The archived `backend-go/acceptance/stage8` and `stage9` directories remain migration evidence only. Do not add new release manifests there.
