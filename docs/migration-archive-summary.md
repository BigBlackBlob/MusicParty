# Java to Go migration archive summary

MusicParty completed its production transition from the Java backend to the Go modular monolith. Go is now the only supported runtime and owns HTTP, WebSocket, configuration and future SQLite migrations.

The migration established these durable conclusions:

- The frozen SQLite compatibility contract contains 23 required application tables.
- Current Go repositories can read, write, close and reopen databases built from the frozen and legacy-upgraded fixtures.
- Migration acceptance exercised room and WebSocket workloads at 10, 100 and 300 concurrent clients, including a 20-room, 1000-item queue scenario.
- The production cutover completed successfully; supported rollback is now a previous immutable Go image digest plus a verified database snapshot.
- Java is not a supported rollback runtime.

Historical checkpoints remain identifiable by the annotated tags `go-rewrite-baseline-v1` and `legacy-java-master-20260802`. Candidate manifests, raw logs, databases, cookies and media artifacts are intentionally not retained in the active tree. Detailed historical source remains available through the scrubbed tags without exposing runtime secrets or binaries.
