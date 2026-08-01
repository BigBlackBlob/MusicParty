# Stage 8 final-candidate acceptance

Accepted source: `96727c13434567152bed889798f1e4803e30b5cf`

Candidate image: `sha256:bbb8eb103050995e2f85157464bbef406eec8a1a206d52c59d01a18ca0ea002b`

Execution window: 2026-08-01, local Windows host with isolated Go processes and Docker containers.

## Result

Stage 8 passed for the final runtime candidate after retired Radio and Tauri code was removed. Java, frontend, Go, image, real-platform, browser and 30-minute load gates all passed.

The VPS database handoff remains the separately agreed Stage 9 boundary. No production environment, production database or remote image registry was touched during this acceptance.

## Quality and image gates

- Java: 49 suites and 176 tests passed with 0 failures, errors or skips.
- Frontend: 30 Vitest files and 97 tests passed; ESLint reported 0 errors and 3 existing warnings; production build passed.
- npm and pnpm clean installs passed. Both package-manager audits reported 0 vulnerabilities when explicitly run against `https://registry.npmjs.org`.
- Go: formatting/tidy, contract generation, vet, tests, build, staticcheck, race, Linux amd64 CGO-free build and govulncheck passed.
- Java/Go HTTP and WebSocket golden comparisons passed.
- The clean Dockerfile build passed. The image ran as `10001:10001` with a read-only root filesystem, all capabilities dropped and `no-new-privileges`; health was `UP`, the static frontend returned 200 and Trivy found 0 CRITICAL findings.

## Real platform gates

- Netease: authenticated search returned 10 results. Track `28816031` resolved complete metadata, entered playback through WebSocket, returned `bytes 0-65535/10310052`, and accepted a 30-second seek. The source-size gate rejects the approximately 481 KB preview result and accepted this 10,310,052-byte source as the full-source run.
- Bilibili: authenticated search returned 10 results and favorites returned 14 non-empty collections. Metadata, enqueue, `bytes 0-65535/17713611` audio Range and 30-second seek passed.
- Local library: a temporary four-second WAV completed upload, FFmpeg OGG conversion, search, enqueue, Range, seek and deletion. Media returned 410 after deletion.

Credentials were injected only into short-lived containers. The containers were deleted immediately after validation. No Cookie, account identity, favorite name, media body, database or executable is stored here.

## Browser gate

A new visual run on the candidate passed administrator login, Lounge entry, WebSocket presence, real Bilibili playback state, Netease search rendering and queue update after enqueue.

Expected unauthenticated 401 responses, browser Wake Lock denial, one cover-color 400, one browser-aborted media request and the unhandled `player.progress` debug message were non-blocking. Independent Range and seek checks are the authoritative media-byte verification.

## Load gates

The committed JSON files are the authoritative measurements.

- 10 connections, 20 rooms and 1000 queue items ran for 30 minutes. Queue ACK p95 was 13.37 ms, maximum ACK was 20.29 ms and convergence was 23.85 ms. The 20 retained room actors explain the post-recovery goroutine count of 39 versus the initial 19.
- 100 connections ran for 30 minutes and emitted 180,734 messages. Goroutines returned from 218 during load to 18 after recovery.
- 300 connections ran for 30 minutes and emitted 537,854 messages. Goroutines returned from 618 during load to 18 after recovery.
- All three wrapper stderr logs were empty.

## Reproduction notes

`stage8-final-platform.mjs` now includes a Netease mode with an explicit minimum full-source byte gate. Dependency audit commands must name the official npm registry because the locally configured npmmirror endpoint does not implement the npm audit API.

The Java-to-Go production database copy handoff is intentionally excluded from Stage 8 and will be performed once, on an isolated copy supplied for the final Stage 9 handoff.
