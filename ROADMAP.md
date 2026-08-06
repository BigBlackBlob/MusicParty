# MusicParty engineering roadmap

This roadmap tracks reliability work. It deliberately avoids unconfirmed product features and dates.

## Now

- Complete repository history scrubbing and rotate external platform credentials.
- Remove remaining inactive Java and migration-era names and development entry points.
- Finish the TypeScript boundary migration for transports, generated contracts, realtime reducers, stores and composables.
- Make an explicit Go `RouteSpec` the shared source for HTTP registration and contract generation.
- Make a Go `EnvironmentSpec` the shared source for configuration loading and generated environment documentation.
- Establish the first Go-owned, versioned SQLite migration framework.

## Next

- Test every database migration from the frozen legacy schema to the current schema.
- Add a maintained Go-only load-test entry point to replace one-off Stage 8 tooling.
- Rehearse backup, restore and immutable-digest rollback procedures.
- Consolidate Prometheus metrics, alert rules and operational runbooks.
- Reduce remaining oversized frontend stores and compatibility-era naming.
- Stabilize Linux CI and Windows local visual-baseline maintenance.
- Remove the `musicparty-go` GHCR compatibility alias in `v1.1.0`.

## Later

- Establish long-running memory, goroutine, WebSocket and SQLite-contention baselines.
- Repeat capacity validation at 100 or more concurrent users.
- Evaluate multi-instance deployment only when real scale requires distributed state.
- Automate disaster-recovery verification.
- Add stronger supply-chain signing, SBOM and provenance guarantees.

Not planned here: microservice commitments, a mobile app, AI recommendation or lyric-translation products, a brand redesign, or unconfirmed commercial release dates.
