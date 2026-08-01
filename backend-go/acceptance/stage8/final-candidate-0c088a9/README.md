# Stage 8 final-candidate acceptance

Accepted source: `0c088a91d10a2a50badd8fa9c9727749acd246f1`

Candidate image: `sha256:57199f06862e8dd18d5be6640b15b67114ac02001f4a263b84b782072f6ceebc`

Execution window: 2026-07-31 to 2026-08-01, local Windows host with isolated Go processes and Docker containers.

## Result

Stage 8 core acceptance passed for the final candidate. The required first-release sources Netease, Bilibili, and the local library passed real playback flows on the same candidate image. The 10, 100, and 300 WebSocket scenarios completed 30 minutes, including a combined 10-connection, 20-room, 1000-item queue run.

Radio is intentionally outside the Go product scope. `/radio/stream` returning 404 is the expected result.

No account Cookie, session token, favorite name, response body, database, executable, audio file, or runner log is part of this evidence.

## Quality and image gates

- Java: 181 tests passed.
- Frontend: clean frozen install, 31 Vitest files and 101 tests passed; ESLint reported 0 errors and 3 pre-existing warnings; production build passed.
- Go: gofmt/tidy check, vet, all tests, build, staticcheck, race, Linux amd64 CGO-free build, and seven generated-contract checks passed.
- Java/Go HTTP and WebSocket golden comparison passed.
- govulncheck found 0 reachable vulnerabilities and 1 known but unreachable required-module vulnerability.
- Dockerfile rebuilt from the clean candidate checkout.
- The final runtime image ran as UID/GID 10001 with a read-only root filesystem, all capabilities dropped, and `no-new-privileges`.
- Trivy found 0 CRITICAL findings in the final runtime image.
- `/actuator/health` returned `UP`; the packaged static frontend returned HTTP 200.

## Real platform gates

- Netease: authoritative metadata, WebSocket enqueue, full upstream audio Range, and seek passed. The sampled stream reported `bytes 0-65535/10310052`, not the earlier cookie-free preview boundary.
- Bilibili: authenticated search returned 10 results; authenticated favorites returned 14 non-empty collections; playable metadata, WebSocket enqueue, audio Range, and seek passed. The sampled stream reported `bytes 0-65535/17713611`.
- Local library: upload, FFmpeg transcode, search, WebSocket enqueue, OGG Range, seek, delete, reference cleanup, and post-delete HTTP 410 passed.

Credentials were injected only into an isolated candidate container. They were not written to these files or the repository.

## Load gates

The committed JSON reports are the authoritative measurements.

- 10 connections + 20 rooms + 1000 queue items, 30 minutes: passed. Queue ACK p95 was about 21.80 ms and convergence was about 0.016 ms.
- 100 connections, 30 minutes: passed. Goroutines returned from 218 during load to 18 after recovery.
- 300 connections, 30 minutes: passed. Goroutines returned from 618 during load to 18 after recovery.

An earlier attempted 10-connection command combined a deliberately unreachable Netease URL with 500 real Netease enqueues. It received an enqueue nack and the harness timed out waiting only for an ack. That invalid parameter combination was discarded; it is not counted as a candidate failure or committed as evidence. The replacement run used the real isolated platform configuration and increased the queue gate to 1000.

## Browser evidence boundary

The prior two-tab browser run covered administrator login, room entry, WebSocket state, real playback, pause, skip, seek, backend restart recovery, and rendering a 1000-item queue. A visual browser-control surface was not available during this final evidence pass, so this record does not claim a new click-through run.

The browser result is carried forward because the diff from the browser-tested candidate `9b35fa7` to `0c088a9` contains only `.gitattributes`, CI/acceptance PowerShell scripts, and frontend package/lock updates adding the `ws` acceptance dependency. There is no diff under `music-party-web/src`, `backend-go/internal`, or `backend-go/cmd`. The final image's packaged frontend returned HTTP 200.

## Frontend dependency follow-up

`npm audit` against the official registry reported 9 advisories: 2 MODERATE, 7 HIGH, and 0 CRITICAL. The final runtime image contains no Node runtime and Trivy reported 0 CRITICAL findings, but browser-bundled and build-time dependencies still require a separate upgrade and advisory-reachability review. This is not represented as “0 vulnerabilities.”

## Excluded gate

The Java to Go to Java SQLite handoff using a copy of the VPS database remains the separately agreed final handoff procedure. It is not an ordinary Stage 8 test and no live or copied production database was used here.
