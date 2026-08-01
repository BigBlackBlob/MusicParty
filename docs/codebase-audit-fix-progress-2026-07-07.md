# Codebase Audit Fix Progress - 2026-07-07

Tracking follow-up for `docs/codebase-audit-2026-07-06.md`.

## Current State

The audit itself appears complete. There is no explicit breakpoint marker in the report. The interrupted work is the follow-up remediation pass for the "Fix Now" list.

Working tree note: unrelated/unconfirmed local changes currently exist in `NeteaseMusicApiService.java`, `NeteaseMusicApiServiceTests.java`, `music_party/`, `playwright-acceptance.js`, and `playwright-acceptance.png`. Do not overwrite these while continuing remediation.

## Fix Now Checklist

| # | Audit ID | Status | Evidence | Test status | Next action |
|---|---|---|---|---|---|
| 1 | S-C1 | Code done | `RoomPlaylistController` mutation endpoints now require `sessionToken` and validate `roomAccessToken` through `RoomAccessService`. | Missing focused controller tests. | Add tests for anonymous mutation rejection and valid private-room mutation. |
| 2 | S-C2 | Code done | `LocalTrackController.media` and `cover` now require either internal stream token or account session token. | Partial: media anonymous rejection covered in `LocalTrackControllerTests`. Cover not covered. | Add cover rejection/allow tests. |
| 3 | S-H1 / T-C1 | Code done | `LoginRateLimiter` exists and `AccountAuthController.login` blocks by resolved client IP, records failures, clears on success, and returns `429` with `Retry-After`. | Missing focused auth controller/rate limiter tests. | Add login rate-limit tests, including disabled config and success reset. |
| 4 | S-M1 | Code done | Local media and cover paths are normalized and checked with `path.startsWith(root)`. | Missing traversal-specific test. | Add path containment regression test with escaped repository path. |
| 5 | S-M4 / B-H19 | Code done | `InternalStreamProxyToken.matches` uses `SecureCompare.equals`. | Missing direct unit test. | Add constant-time helper/token match behavior test if practical. |
| 6 | B-C3 | Code done, needs review | Main `MusicPlayerService` reactive callbacks now use `publishOn(Schedulers.boundedElastic())` and have error handlers for enqueue/playlist/album paths. | Existing service tests may cover behavior, not thread placement. | Run backend tests; consider targeted regression around failed enqueue error event. |
| 7 | B-C5 | Code done | `skipToNext`, `togglePause`, and `toggleShuffle` are `synchronized` on `RoomPlayerSession`. `isRateLimited` now uses CAS. | No concurrency stress test. | Add/keep as manual code-audit verified unless race reappears. |
| 8 | B-C6 | Code done | `RoomPlaybackState` field getters/setters are synchronized and `currentTrackInfo()` provides composite current-track reads. | No dedicated snapshot consistency test. | Add small unit test if `RoomPlaybackState` visibility/package allows. |
| 9 | F-C1 | Resolved by UI removal | `MobileNowPlaying.vue` no longer calls missing `playPrevious`, `toggleRepeat`, or `repeatMode`; mobile controls now use shuffle, pause, next only. | No component behavior test. Existing grep shows no active references. | Optional: document that previous/repeat were intentionally removed, or implement real backend actions later. |
| 10 | F-C5 | Code done | `useAudio` tracks retry timers in a `Set`, clears them on track-change/unmount paths, and guards scheduled reloads by track id. | Source-level buffering tests exist; no real timer behavior test. | Add fake-timer test if refactoring `useAudio` becomes feasible. |
| 11 | T-C7 | Code done | `Dockerfile` creates `appuser`, switches with `USER appuser`, and adds `HEALTHCHECK`. | Not build-verified in this pass. | Run Docker build or at least CI build when environment allows. |

## Remaining Critical Follow-up

1. Add focused tests for the security fixes. Highest value: `RoomPlaylistController` auth, `AccountAuthController` login rate limiting, `LocalTrackController.cover`.
2. Run the existing quality gates:
   - Backend: `./mvnw test`
   - Frontend: `npm run lint`, `npm run test:run`, `npm run build` from `music-party-web`
3. After tests pass, update this document with results and decide whether to commit the remediation separately from the unrelated Netease/free-trial work.

## 2026-07-21 Follow-up Remediation

| Area | Status | Evidence |
|---|---|---|
| Queue reorder delivery | Code done | Reorder requests now carry `mutationId`; the server replies with `queue.reorder.ack` or `queue.reorder.nack` instead of failing silently. |
| Queue broadcast staleness | Code done | Queue broadcasts now include a room-local `queueVersion`. The frontend rejects stale versions and holds broadcasts while a local reorder awaits acknowledgement. |
| Drag payload validity | Code done | Desktop and mobile Sortable handlers use DOM neighbours first and only fall back to indices when the dragged queue id still matches the current store state. |
| Queue rate limiting | Code done | General queue actions are now `30/10s`; reorder has an isolated `20/10s` bucket. |
| Mobile drag usability | Code done | The mobile drag handle stays visible and Sortable delay only applies to touch input. |
| Socket and layout lifecycle | Code done | Reconnect timers are cancellable, reconnect uses capped exponential backoff, room-scoped messages are filtered, and layout Sortable initializes after mount. |
| SQLite and transactions | Code done | SQLite config enables WAL, NORMAL synchronous mode, and a 5s busy timeout. User playlist delete/reorder operations are transactional. |
| Room command dispatch | Code done | WebSocket dispatch awaits per-room asynchronous command serialization instead of blocking a boundedElastic thread on `join()`. |
| Queue state synchronization | Code done | `MusicQueueManager` now uses one synchronized concurrency model instead of concurrent collections plus multiple locks. |
| Heartbeat lifecycle | Code done | WebSocket heartbeat now follows player socket connection state instead of the audio component lifecycle. |
| Schema migration atomicity | Code done | Each migration apply and completion marker runs in one SQLite transaction. |

## Verification Log

2026-07-07:

- Frontend tests passed: `npm run test:run` completed with 30 test files and 92 tests passing.
- Frontend lint passed with warnings only: `npm run lint` reported 3 `no-unused-vars` warnings in `src/services/socketHandler.js`.
- Frontend production build passed: `npm run build`.
- Backend tests not run: `mvnw.cmd test` failed before test execution because `JAVA_HOME` is not defined and `java` is not available on `PATH` in this environment.

2026-07-21:

- Frontend tests passed: `npm run test:run` completed with 31 test files and 97 tests passing.
- Frontend lint completed with the pre-existing 3 `no-unused-vars` warnings in `src/services/socketHandler.js`; no lint errors.
- Frontend production build passed: `npm run build`.
- Backend compilation passed: `cmd /c mvnw.cmd -DskipTests compile`.
- Backend test suite passed: `cmd /c mvnw.cmd test`.
- Docker image build passed: `docker build --tag musicparty:queue-audit .` completed the production `mvn clean package -DskipTests` path.
- Docker smoke test passed: an isolated `musicparty:queue-audit` container started with a fresh named volume on port `18848`; `/actuator/health` returned `200`, Docker health was `healthy`, `/api/account/status` returned `{"requiresSetup":false}`, and SQLite migrations completed without errors. The temporary container and volume were removed after verification.

## Notes

- The audit report's medium/low backlog is intentionally not included here.
- The WebSocket credentials-in-URL issue and chat XSS item were explicitly deferred in the original decision log.
- Test coverage gaps from T-C2 through T-C6 and T-C8 remain backlog unless the current remediation pass is expanded.
