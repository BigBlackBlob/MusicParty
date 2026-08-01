# Codebase Audit Report — 2026-07-06

Comprehensive audit of the MusicParty codebase. Findings are organized by severity tier (CRITICAL / HIGH / MEDIUM / LOW), then by domain (Backend / Frontend / Security / Test & Config).

**Audit scope**: `src/main/java` (166 files), `music-party-web/src` (101 files), `pom.xml`, `package.json`, `Dockerfile`, `docker-compose*.yml`, `application.yml`, `.github/workflows/quality.yml`, `README.md`.

**Deployment context**: Public internet (multi-user, some untrusted). Security findings are rated accordingly.

**Decision log** (from grill-me session):
- Deployment: **Public internet** — all security findings are urgent
- Remediation scope: **Critical fixes only** (~15 findings to fix now, rest deferred to this backlog)
- Thread-safety approach: **Offload + synchronize** (Option A, not per-room event loop)
- WS credentials in URL: **Deferred to backlog** (you control reverse proxy logs)
- Chat XSS: **Deferred** (not exploitable with current Vue `{{ }}` rendering — no `v-html` anywhere)

---

## Summary

| Domain | CRITICAL | HIGH | MEDIUM | LOW | Total |
|---|---|---|---|---|---|
| Backend services | 11 | 21 | 42+ | — | 74+ |
| Frontend | 6 | 17 | 21 | 20 | 64 |
| Security | 2 | 3 | 5 | 6 | 16 |
| Test & Config | 8 | 8 | 10 | 8 | 34 |
| **Total** | **27** | **49** | **78+** | **34+** | **188+** |

**Top themes**:
1. Backend state machine races (11 CRITICAL) — blocking DB on reactor threads, unsynchronized mutations, torn snapshots
2. Security access control (2 CRITICAL + 3 HIGH) — unauthenticated room playlists, unauthenticated media, no login rate limiting
3. Frontend state lifecycle (6 CRITICAL) — dead mobile buttons, watcher/timer leaks, incomplete room-switch reset
4. Test coverage (8 CRITICAL) — auth/crypto/admin paths untested; existing tests are source-string matching
5. i18n breakdown (HIGH) — large fraction of strings hardcoded despite full i18n setup

---

# CRITICAL Findings (27)

## Backend Services

### B-C1 — Double broadcast of `UserCountChangeEvent` on every non-guest disconnect
- **File**: `UserService.java:148-156`
- **Category**: Concurrency / event storm
- **Description**: `disconnectUser` invokes `roomSessionCoordinator.onUserDisconnected(roomId, ...)` immediately at line 156 AND again inside the 10-second-delayed scheduled task at line 152. Each call publishes a `UserCountChangeEvent` and calls `roomService.publishRoomList()` (DB query).
- **Impact**: Duplicate online-count broadcast 10s after every disconnect; phantom user-count flicker; extra DB load.

### B-C2 — `ConcurrentHashMap.computeIfAbsent` performs blocking DB I/O on the 1Hz scheduler thread
- **File**: `MusicPlayerService.java:139-142`, `:291-298`
- **Category**: Reactive / blocking-in-reactor / thread-safety
- **Description**: `@Scheduled(fixedRate=1000) playerLoop()` calls `getActiveRoomIds()` → `roomService.listRooms()` (DB query) every second, then `session(roomId)` uses `sessions.computeIfAbsent` which calls `restorePersistentState` (3 blocking repository calls) **while holding the ConcurrentHashMap bin lock**.
- **Impact**: Single cold-room activation stalls the 1Hz player loop for ALL rooms; slow DB → global playback stall.

### B-C3 — Blocking DB transactions executed on `reactor-http-nio` threads via fire-and-forget `.subscribe()`
- **File**: `MusicPlayerService.java:522, 542, 568, 379, 451`
- **Category**: Reactive / blocking-in-reactor
- **Description**: Multiple `getPlayableMusic(...).subscribe(playable -> { ... })` callbacks run on WebClient NIO event-loop threads. Inside: `queueManager.add` (synchronized), `persistQueueMutation` → DB transaction, `applyPlaybackTransition` → DB transaction + event publish. `enqueuePlaylist` (542) and `enqueueAlbum` (568) subscribe with **no error handler**.
- **Impact**: NIO event-loop threads blocked by JDBC → entire HTTP client stalls under load; failed enqueues vanish silently.
- **Decision**: **Fix now** — wrap callbacks in `Schedulers.boundedElastic()`.

### B-C4 — `isRateLimited` check-then-act race on `AtomicLong`
- **File**: `MusicPlayerService.java:1049-1055`
- **Category**: Thread-safety / rate-limit bypass
- **Description**: `if (now - lastControlTimestamp.get() < GLOBAL_COOLDOWN_MS) return true; lastControlTimestamp.set(now);` is not atomic. Two concurrent control calls from the same user both read the old timestamp and both proceed.
- **Impact**: Per-user 1s cooldown bypassable by concurrent requests.

### B-C5 — `skipToNext`, `togglePause`, `toggleShuffle` are NOT synchronized while mutating shared playback state
- **File**: `MusicPlayerService.java:652-666, 668-687, 701-709`
- **Category**: Thread-safety / race condition
- **Description**: These methods mutate `playbackState` (setLoading, clearCurrentTrack, setPaused, updatePlaybackAnchor, setShuffle, bumpStateVersion) and call `runInTransaction` + `playNextInQueue` without holding the `RoomPlayerSession` monitor. Concurrent invocations interleave: e.g. `togglePause` does `setPaused(newState)` then `updatePlaybackAnchor(currentPos)` — between these, `calculateCurrentPosition` sees the new `paused` flag with the **old** anchor, producing a position jump.
- **Impact**: Position jumps, double-skips, lost pause toggles, inconsistent `stateVersion` snapshots.
- **Decision**: **Fix now** — make these methods `synchronized`.

### B-C6 — Mixed AtomicReference/AtomicBoolean + selective `synchronized` yields torn snapshots
- **File**: `RoomPlaybackState.java:38-48, 130-227`
- **Category**: Thread-safety / visibility / inconsistency window
- **Description**: Some getters are `synchronized` (`currentMusic`, `setCurrentTrack`, `startNewTrack`, `calculateCurrentPosition`); others (`isShuffle`, `isPaused`, `isLoading`, `setShuffle`, `setPaused`, `setLoading`, `bumpStateVersion`) are **not**. Callers chain multiple unsynchronized calls so the trio (music, enqueuerId, enqueuerName) can be observed torn.
- **Impact**: Broadcast `PlayerState` snapshots can mix pre- and post-mutation field values; persisted state inconsistent.
- **Decision**: **Fix now** — synchronize all field setters + add composite getter.

### B-C7 — `CacheEntry` fields are non-volatile and mutated from multiple threads
- **File**: `LocalCacheService.java:48, 88-139, 417-422`
- **Category**: Thread-safety / visibility
- **Description**: `cacheIndex` is `ConcurrentHashMap<String, CacheEntry>`, but `CacheEntry` is a `@Data` POJO with plain (non-volatile) fields. `setStatus(DOWNLOADING)` from the download worker may not be visible to `getStatus` on the player-loop thread.
- **Impact**: `getPlayableMusic` may return `PENDING_DOWNLOAD` for a track that already completed; `cleanupStaleTasks` may delete `.part` files for entries still downloading.

### B-C9 — `CoverColorService` semaphore acquired at assembly time, no timeout on `webClient`, DNS + image decode on reactor thread
- **File**: `CoverColorService.java:50-129`
- **Category**: Reactive / resource-leak / blocking-in-reactor
- **Description**: `extract()` calls `concurrentExtracts.tryAcquire()` imperatively at assembly time. If the Mono is never subscribed or `webClient.get()` hangs (no `.timeout()`), the permit is never released. `ImageIO.read()` runs on the NIO thread; `InetAddress.getAllByName()` (blocking DNS) runs on the controller thread.
- **Impact**: A few hung cover fetches permanently drain the semaphore; image decoding blocks the NIO event loop.

### B-C10 — Scheduled tasks mutate `sessions` and run DB transactions on scheduler thread without per-session locking
- **File**: `MusicPlayerService.java:60, 139, 144, 149`
- **Category**: Thread-safety / race condition / blocking
- **Description**: `playerLoop` (1Hz), `cleanupIdlePlayer` (10min), `evictColdRooms` (5min) iterate `sessions.values()` and call `runInTransaction` (JDBC writes). `evictColdRooms` does `sessions.remove(roomId)` based on a TOCTOU check — a user can connect between check and removal. Spring's default `TaskScheduler` is single-threaded.
- **Impact**: Concurrent eviction vs. user-connect races; scheduler thread blocked on JDBC stalls all `@Scheduled` work.

### B-C11 — `Executors.newSingleThreadScheduledExecutor()` never shut down; non-daemon thread
- **File**: `UserService.java:40`
- **Category**: Resource-leak
- **Description**: `scheduler` field is a `newSingleThreadScheduledExecutor()` with no `@PreDestroy` and no daemon flag.
- **Impact**: JVM will not exit on application shutdown because of this non-daemon thread.

## Frontend

### F-C1 — Mobile transport buttons call non-existent store methods
- **File**: `src/components/mobile/MobileNowPlaying.vue:126, 160, 162, 165`
- **Category**: Component / State management
- **Description**: `@click="player.playPrevious"`, `@click="player.toggleRepeat"`, `:class="player.repeatMode !== 'none'"` — none of `playPrevious`, `toggleRepeat`, `repeatMode` are defined in `src/stores/player.js`. These resolve to `undefined`; the Previous and Repeat buttons are completely dead.
- **Impact**: Three transport controls on mobile silently do nothing. No error thrown, making this hard to notice.
- **Decision**: **Fix now** — add methods to player store or remove buttons.

### F-C2 — Session token and room access token passed in WebSocket URL query string
- **File**: `src/services/socket.js:57-69`
- **Category**: WebSocket / Security
- **Description**: `buildUrl` writes `user-name`, `session-token`, `room-id`, `room-access-token` into the query string of the `ws://`/`wss://` URL. Query strings are captured in reverse-proxy access logs, browser history, and `Referer` headers.
- **Impact**: Credential leakage to logs/history. Anyone with log access can impersonate a user or access private rooms.
- **Decision**: **Defer to backlog** — you control the reverse proxy logs.

### F-C3 — `orientationchange` window listener never removed
- **File**: `src/App.vue:444-446` (add) vs `456-460` (remove)
- **Category**: Component / Event listener leak
- **Description**: `onMounted` adds `resize`, `orientationchange`, and `visualViewport.resize` listeners. `onBeforeUnmount` only removes `resize` and `visualViewport.resize`. The `orientationchange` listener is never removed.
- **Impact**: Listener leak; `setAppViewportHeight` keeps firing on orientation changes after unmount.

### F-C4 — Watchers created inside scroll handler may never dispose
- **File**: `src/components/ChatOverlay.vue:232-237, 244-249`; same pattern in `src/components/modules/ChatModule.vue:150-161`
- **Category**: Component / Memory leak
- **Description**: `handleScroll` calls `chatStore.loadMoreHistory()` then creates `const unwatch = watch(...)` and calls `unwatch()` only after the watcher fires once. If the network request fails or `isLoadingMore` is never reset, `messages.length` never changes and the watcher lives forever. Each subsequent scroll re-arms another watcher.
- **Impact**: Unbounded watcher accumulation → memory leak + stale closures.

### F-C5 — Untracked `setTimeout` in audio retry paths fires on wrong track
- **File**: `src/composables/useAudio.js:98-108` (`hardReloadSource`), `432-468` (`handleError`)
- **Category**: Audio / Timer leak / Race
- **Description**: `hardReloadSource` schedules `setTimeout(() => { currentAudio.load() }, STALLED_RETRY_DELAY_MS)` and `handleError` schedules `setTimeout(() => { currentAudio.load() }, 1500)`. Neither timer is stored or cleared on track-change. The guards check the *current* track, not the track that errored — if the user skips while a retry is pending, the timeout will `load()` the new track's source at the wrong moment.
- **Impact**: Spurious `load()` on the wrong track → playback glitches, lost buffer, or `pendingResumePositionMs` applied to the wrong song.
- **Decision**: **Fix now** — track all retry timers and clear on track-change/unmount.

### F-C6 — Playlist stores not reset on room switch (stale data + memory growth)
- **File**: `src/stores/player.js:297-323`; `src/stores/roomPlaylists.js`; `src/stores/userPlaylists.js`
- **Category**: State management / Store coupling
- **Description**: `resetRoomState` resets player/chat/online users but never touches `useRoomPlaylistsStore` or `useUserPlaylistsStore`. On room switch, playlists from the previous room remain visible until `loadRoomPlaylists()` resolves. `tracksByPlaylist` is never pruned — accumulates every playlist ever browsed.
- **Impact**: Stale cross-room playlist UI; unbounded growth of `tracksByPlaylist`.

## Security

### S-C1 — RoomPlaylistController completely missing authentication on all mutation endpoints (IDOR)
- **File**: `src/main/java/org/thornex/musicparty/controller/RoomPlaylistController.java:29-73`
- **Category**: Broken Access Control / IDOR / Missing Auth
- **Description**: Every state-changing endpoint (`POST` create, `PATCH` rename, `DELETE` playlist, `POST` addTrack, `DELETE` track, `POST` reorder, `POST` import) accepts only `@PathVariable roomId` and `playlistId`/`trackId`. There is **no `sessionToken` parameter and no call to any authorization service**. Compare with `UserPlaylistController` which correctly requires `@RequestParam String sessionToken` on every method.
- **Attack scenario**: An unauthenticated attacker enumerates `roomId` (leaked via API responses, URLs, lobby list) and `playlistId` (UUIDs returned by the public `GET /api/rooms/{roomId}/playlists` endpoint). The attacker issues `DELETE /api/rooms/{roomId}/playlists/{playlistId}` to wipe any room's playlists, or `POST .../import` to trigger expensive external playlist imports (DoS).
- **Impact**: Any anonymous user can destroy or tamper with every room playlist in the system.
- **Decision**: **Fix now** — add `sessionToken` param + ownership check to every mutation endpoint.

### S-C2 — LocalTrackController unauthenticated media streaming when local library is enabled (default)
- **File**: `src/main/java/org/thornex/musicparty/controller/LocalTrackController.java:118-141, 166-168`
- **Category**: Broken Access Control / Authentication Bypass
- **Description**: `canReadMedia(token, internalToken)` returns `internalStreamProxyToken.matches(internalToken) || accessService.isEnabled()`. `accessService.isEnabled()` returns `true` whenever `app.music-api.local-library.enabled` is true, which **defaults to `true`**. Therefore, by default, **no token, no session is required** to stream any local track. `GET /api/local/cover/{id}` has **no auth check at all**.
- **Attack scenario**: Track IDs are server-generated UUIDs but are returned in player state/queue payloads broadcast over WebSocket. An attacker who has observed any track id calls `GET /api/local/media/{id}` with no credentials and downloads the full OGG audio of privately-uploaded tracks.
- **Impact**: Confidentiality breach of all locally uploaded audio/cover art; trivial mass download.
- **Decision**: **Fix now** — gate behind session-token check.

## Test & Config

### T-C1 — Documented login brute-force rate limiting is NOT implemented
- **Area**: Configuration audit / Security / Documentation drift
- **Evidence**: `application.yml:97-102` defines `app.music-api.auth.{rate-limit-enabled, max-attempts, window-seconds, block-duration-seconds, max-tracked-ips}`. A grep for these getters across `src/main/java` returns **zero** matches. `AccountService.login` performs no attempt tracking. `README.md:130-133` and `docker-compose.yml:76-79` advertise this protection as enabled.
- **Risk**: Unbounded password brute-force against `/api/account/login`. Operators believe they are protected when they are not.
- **Decision**: **Fix now** — wire up `AuthConfig.maxAttempts/windowSeconds/blockDurationSeconds` in `AccountService.login`.

### T-C2 — `AccountAuthController` (auth endpoint) has NO test
- **Area**: Test coverage gap (backend)
- **Evidence**: `AccountAuthController.java` (register/login/me/logout/change-password/update-profile) has no corresponding test file. `AccountServiceTests` covers the service layer only.
- **Risk**: Auth HTTP contract (401 vs 400, session header issuance/revocation, register→admin bootstrap) can regress silently.

### T-C3 — `AdminController` and `AdminAuthorizationService` have NO test
- **Area**: Test coverage gap (backend, security)
- **Risk**: Privilege-escalation / missing-authorization regressions in admin endpoints go undetected.

### T-C4 — `SubsonicCredentialCipher` (AES-GCM credential encryption) has NO test
- **Area**: Test coverage gap (backend, security/crypto)
- **Risk**: A silent crypto bug could corrupt all stored credentials or decrypt to the wrong plaintext. Crypto code without round-trip tests is a classic source of unrecoverable data loss.

### T-C5 — `RoomPasswordHasher` and `SecureCompare` have NO test
- **Area**: Test coverage gap (backend, security)
- **Risk**: A regression in `SecureCompare` reintroduces timing attacks. A regression in `RoomPasswordHasher` could silently accept wrong passwords.

### T-C6 — `StreamTokenService` and `StreamController` have NO test
- **Area**: Test coverage gap (backend, security)
- **Risk**: Token validation logic can regress, allowing unauthorized stream access.

### T-C7 — Dockerfile runs as root, has no `USER`, no healthcheck, bloated runtime image
- **Area**: Configuration audit / Dockerfile security
- **Evidence**: `Dockerfile` Stage 3 has no `USER` directive — the app runs as root. Runtime stage installs `python3`, `py3-pip`, `ffmpeg`, and `yt-dlp` via `pip3 install --break-system-packages`. No `HEALTHCHECK`. No `--read-only` / `--cap-drop` guidance in compose.
- **Risk**: Container escape or RCE via yt-dlp/ffmpeg parser bugs executes as root.
- **Decision**: **Fix now** — add `USER` directive + `HEALTHCHECK`.

### T-C8 — `BilibiliProxyControllerTest` is source-string matching, not behavioral
- **Area**: Test quality (backend)
- **Evidence**: `BilibiliProxyControllerTest.java` reads `.java` source as string and asserts substrings like `"cdnUrlCache"`, `"fallback to local cache"`. It never instantiates the controller or exercises the proxy/range/fallback logic.
- **Risk**: Tests pass even if the Bilibili proxy is completely broken. The failsafe mechanisms are effectively untested.

---

# HIGH Findings (49)

## Backend Services

### B-H1 — `playNextInQueue` synchronized but work happens in async `.subscribe` without the monitor
- **File**: `MusicPlayerService.java:379-407`
- **Category**: Thread-safety / reactive
- **Description**: `synchronized void playNextInQueue()` increments `playHeadVersion`, sets `loading=true`, then `getPlayableMusic(...).subscribe(playable -> { applyNewSong(...); })`. The subscribe callback runs on a reactor thread and calls `applyNewSong` → `playbackState.startNewTrack` (synchronized on a different monitor). The `synchronized` on `playNextInQueue` only protects the *setup*, not the *completion*.
- **Impact**: Two songs can race to `applyNewSong`; the loser is silently dropped.

### B-H2 — `enqueue` subscribe runs `queueManager.add` + DB transaction on reactor NIO thread; per-user queue cap bypassable
- **File**: `MusicPlayerService.java:522-533`
- **Category**: Reactive / concurrency
- **Description**: The `count >= maxUserSongs` check runs before the async resolution; two concurrent enqueues from the same user both see `count < max` and both add, exceeding the cap. The callback also does JDBC on the NIO thread.
- **Impact**: Queue cap violation; NIO thread blocked on JDBC.

### B-H3 — `enqueuePlaylist`/`enqueueAlbum` subscribe with no error handler
- **File**: `MusicPlayerService.java:542-552, 568-577`
- **Category**: Reactive / error-handling
- **Description**: `.subscribe(musics -> {...})` — no `errorConsumer`. If `getPlaylistMusics`/`getAlbumMusics` errors, Reactor drops it via `Operators.onErrorDropped` (just a log line). The user receives no `ERROR_LOAD` event.
- **Impact**: Silent failures; users see "nothing happened" with no feedback.

### B-H4 — `markRoomActive` (DB write) called outside the transaction, on every queue mutation and every playback transition
- **File**: `MusicPlayerService.java:959-971, 973-983`
- **Category**: Performance / blocking
- **Description**: `persistQueueMutation` and `applyPlaybackTransition` all call `roomSessionCoordinator.markRoomActive(roomId)` which does a DB UPDATE. This fires on every like, every seek, every pause, every queue add/remove/reorder, every track finish — and the DB write is outside the surrounding transaction.
- **Impact**: One extra auto-commit DB write per user action; combined with NIO-thread callbacks this is blocking JDBC on the event loop.

### B-H5 — `listRooms()`/`publishRoomList()` runs a DB query and re-populates in-memory map on every user event
- **File**: `RoomService.java:64-72, 155-160, 227-229`
- **Category**: Performance / blocking
- **Description**: `listRooms()` calls `roomRepository.findAllActive()` and uses `.peek(room -> rooms.put(...))`. `publishRoomList()` is invoked on every connect/disconnect.
- **Impact**: O(rooms) DB query per user connection event; thrashes the DB under bursty reconnects.

### B-H6 — `onlineCountProvider` is O(users) and called once per room in `listRooms()`
- **File**: `UserService.java:54, 282-284`
- **Category**: Performance
- **Description**: `RoomService.toInfo` calls `onlineCountProvider.applyAsInt(room.roomId())` for each room. `UserService.getOnlineCount` iterates all in-memory users. `listRooms()` is O(rooms × users).
- **Impact**: Quadratic cost per reconnect burst; 50 rooms × 200 users = 10k scans per `publishRoomList`.

### B-H9 — `downloadQueue` is unbounded; `pendingDownloadTasks` decremented before completion
- **File**: `LocalCacheService.java:128-138, 52, 174-199`
- **Category**: Resource-leak / backpressure
- **Description**: `Sinks.many().unicast().onBackpressureBuffer()` is unbounded. `processTask` decrements `pendingDownloadTasks` at start, before download completes. A slow yt-dlp (15 min timeout) causes counter to hit 0 while hundreds of submissions pile into the unbounded buffer.
- **Impact**: Unbounded memory growth under sustained prefetch load; OOM under queue storm.

### B-H10 — `runDownloadCommand` leaks `CompletableFuture` on common ForkJoinPool; never closes stdout on timeout
- **File**: `LocalCacheService.java:263-299`
- **Category**: Resource-leak / process management
- **Description**: `CompletableFuture.supplyAsync(() -> readProcessOutput(...))` uses the common FJ pool. If `process.waitFor(15, MINUTES)` times out, `destroyForcibly()` is called but the supplyAsync task continues running on the common pool, blocked on `read()` forever.
- **Impact**: Common FJ pool threads leak per timed-out yt-dlp run.

### B-H11 — `currentTotalSize` updated outside `ensureCapacity`'s synchronized block; concurrent completions over-evict
- **File**: `LocalCacheService.java:322-338, 371-393`
- **Category**: Concurrency
- **Description**: `completeDownload` does `currentTotalSize.addAndGet(size)` then `ensureCapacity()` (synchronized). Two concurrent completions both enter `ensureCapacity` and both run the eviction loop.
- **Impact**: Cache thrash — valid entries evicted because of duplicate eviction passes.

### B-H12 — `ChatService.onSystemEvent` persists messages without a transaction and runs on reactor NIO thread
- **File**: `ChatService.java:304-349`
- **Category**: Reactive / transaction / blocking
- **Description**: Unlike `addMessage`/`broadcastSystemMessage` (which wrap in `runInTransaction`), `onSystemEvent` calls `persistRoomMessage` directly with no active transaction. The event is published from `MusicPlayerService` subscribe callbacks on reactor NIO threads.
- **Impact**: NIO thread blocked on JDBC for every `PLAY_START`/`LIKE`/`SKIP`/`PAUSE` system event.

### B-H13 — `pollFromHistory` races with `addToHistory` due to mismatched monitors; `get(index)` on LinkedList may throw
- **File**: `MusicQueueManager.java:227-271, 477-492`
- **Category**: Thread-safety
- **Description**: `playHistory` is `Collections.synchronizedList(new LinkedList<>())` (monitor = the list itself). `pollNext` (synchronized on `MusicQueueManager`, a different monitor) calls `pollFromHistory` which does `playHistory.get(new Random().nextInt(playHistory.size()))` **without synchronizing on `playHistory`**. If `addToHistory` runs concurrently and the list shrinks, `get` throws `IndexOutOfBoundsException`.
- **Impact**: AutoDJ crashes mid-playback when history is being appended.

### B-H14 — `getQueueSnapshot()` is NOT synchronized while `top`/`reorder`/`restore` do `queue.clear(); queue.addAll(...)`
- **File**: `MusicQueueManager.java:427-429`
- **Category**: Thread-safety
- **Description**: `getQueueSnapshot()` returns `new ArrayList<>(queue)` without holding the monitor. Concurrently, `top`/`topManyGlobal`/`reorder`/`restore` do `queue.clear()` then `queue.addAll(snapshot)`.
- **Impact**: `getQueueWithUpdatedStatus`/`persistQueueMutation`/`broadcastQueueUpdate` observe transient empty/partial queue states.

### B-H15 — `playerLoop`'s "track finished" path runs `runInTransaction` (DB) on the 1Hz scheduled thread
- **File**: `MusicPlayerService.java:339-346`
- **Category**: Blocking / reactive
- **Description**: When a track finishes, `playerLoop` (1Hz scheduled) calls `runInTransaction(...)` with 3 JDBC writes. Spring's default `TaskScheduler` is single-threaded.
- **Impact**: One finished track stalls the player loop for all rooms.

### B-H16 — `removeRoom`/`evictColdRooms` race with `session()` re-creating the session
- **File**: `MusicPlayerService.java:124-137, 159-165`
- **Category**: Thread-safety / race condition
- **Description**: `removeRoom` does `sessions.remove(normalized)` then cleanup. A concurrent `session(roomId)` can `computeIfAbsent` a new session for the same roomId between `remove` and cleanup.
- **Impact**: Evicted/deleted rooms silently resurrect with empty state.

### B-H17 — Session tokens / admin passwords passed as `@RequestParam` (URL query strings)
- **File**: `RoomController.java:39, 99, 124, 130`, `AdminController.java:177, 225`, `LocalTrackController.java:41, 92, 119`, `ApiController.java:66, 116`, `StreamController.java:37`
- **Category**: API / security
- **Description**: Many endpoints take `sessionToken`/`token`/`adminPassword`/`key` as `@RequestParam`. Query strings are logged by reverse proxies and browser history.
- **Impact**: Credential exposure via logs/referrers.

### B-H18 — Netease raw session cookie placed in request URL as query parameter
- **File**: `NeteaseMusicApiService.java:189, 210, 235, 272, 326, 351, 371, 389`
- **Category**: Security
- **Description**: `uri(baseUrl + "/...?cookie={cookie}", ..., getCookie())`. The Netease cookie is part of the request URL → logged by WebClient debug, access logs.
- **Impact**: Netease account cookie leakage via logs.

### B-H19 — `InternalStreamProxyToken` comparison is non-constant-time
- **File**: `InternalStreamProxyToken.java:24`
- **Category**: Security / timing side-channel
- **Description**: `matches` uses `String.equals`, which short-circuits. The project ships `SecureCompare` (uses `MessageDigest.isEqual`) but it's not used here.
- **Impact**: Timing side-channel on a token that bypasses all per-user auth.
- **Decision**: **Fix now** — use `SecureCompare.equals`.

### B-H20 — Proxy controllers' `HttpResponse<InputStream>` may not close on client cancel mid-stream
- **File**: `BilibiliProxyController.java:166-211`, `NeteaseProxyController.java:71-113`, `NavidromeProxyController.java:66-116`, `SubsonicProxyController.java:77-124`
- **Category**: Resource-leak
- **Description**: On client cancel mid-stream, the InputStream closure depends on the operator's `doFinally`. If not closed, the underlying HttpClient connection is not returned to the pool.
- **Impact**: Under clients that disconnect mid-stream, HttpClient connections leak until pool exhausted.

### B-H21 — Per-controller `HttpClient` instances never closed
- **File**: `BilibiliProxyController.java:42`, `NeteaseProxyController.java:33`, `NavidromeProxyController.java:37`, `SubsonicProxyController.java:29`
- **Category**: Resource-leak
- **Description**: Each proxy controller constructs a `java.net.http.HttpClient` as an instance field. None have `@PreDestroy` to close them. `HttpClient` owns an internal `Executor` and connection pool.
- **Impact**: Executor leak on hot-reload / context refresh.

## Frontend

### F-H1 — Public chat state not reset on room switch
- **File**: `src/stores/chat.js:114-119`
- **Category**: State management
- **Description**: `resetRoomMessages` clears `messages`, `unreadCount`, `hasMore`, `isLoadingMore` but not `publicMessages`, `publicUnreadCount`, `publicHasMore`, `isLoadingPublicMore`.
- **Impact**: Public channel messages from old room persist into new room.

### F-H2 — WebSocket reconnect has no backoff, no max attempts, untracked reconnect timer
- **File**: `src/services/socket.js:9, 111-117, 134-141`
- **Category**: WebSocket
- **Description**: `reconnectDelay` is fixed 2000ms with no exponential backoff and no cap. `reconnectNow()` calls `setTimeout(connect, 100)` without storing the timer; calling repeatedly queues parallel `connect()` calls.
- **Impact**: Network flapping → client hammers server every 2s forever; overlapping connects race `this.client` assignment.

### F-H3 — No socket-level heartbeat; visibility/online hooks are the only liveness check
- **File**: `src/services/socket.js` (no ping), `src/composables/useAudio.js:498`
- **Category**: WebSocket
- **Description**: The only heartbeat is `playerStore.requestPing('interval')` fired from `useAudio`'s `onMounted`. If `AudioEngine` is not mounted, there is no heartbeat and no stale-pong detection.
- **Impact**: A silently half-open socket with no visibility change is never detected; UI shows "connected" while no data flows.

### F-H4 — `onAuthError` hard-reloads the page, discarding all state
- **File**: `src/services/socketHandler.js:211-221`
- **Category**: WebSocket / UX
- **Description**: Any auth error calls `window.location.reload()`. This drops queued actions, liked-songs sync state, layout edits, and the current view.
- **Impact**: Jarring full reloads; users lose in-progress input.

### F-H5 — `RESET` event directly mutates store state, bypassing actions and missing public chat
- **File**: `src/services/socketHandler.js:31-33`
- **Category**: Store coupling / WebSocket
- **Description**: `if (event.action === 'RESET') { chatStore.messages = []; }` reaches into the store's internal ref, bypasses `resetRoomMessages`, and does not clear `publicMessages`. Falls through (no `return`) and also shows a toast.
- **Impact**: Inconsistent state; public chat not reset; double notification.

### F-H6 — L2 "soft retry" reassigns `audio.src`, likely destroying the buffer it claims to preserve
- **File**: `src/composables/useAudio.js:113-131`
- **Category**: Audio
- **Description**: `softRetrySource` sets `audio.src = currentSrc`. In practice, assigning to `src` (even the same value) triggers a fresh media fetch and clears `buffered` in most browsers. The Range-resume behavior is implementation-dependent.
- **Impact**: L2 is effectively as destructive as L3, contradicting the three-level design.

### F-H7 — `fadeToGain` uses `requestAnimationFrame`, which is paused in background tabs
- **File**: `src/composables/useAudio.js:224-243`
- **Category**: Audio
- **Description**: The fade loop drives `requestAnimationFrame(step)`. When tab is hidden, rAF is throttled to ~0 fps, so the fade never completes. `smoothSeekTo` awaits `fadeToGain`, so a seek initiated just before backgrounding can hang with `smoothSeekInFlight = true` until the tab returns.
- **Impact**: Stuck `smoothSeekInFlight` while backgrounded → smooth-seek path disabled.

### F-H8 — `releaseWakeLock` is async but called from synchronous `onUnmounted`
- **File**: `src/composables/useAudio.js:202-207, 572`
- **Category**: Audio / Resource leak
- **Description**: `onUnmounted` calls `releaseWakeLock()` (async) without awaiting. The `wakeLock.release()` promise may reject after the composable's closure is gone.
- **Impact**: Potential unhandled promise rejection; wake lock may not be released promptly.

### F-H9 — `player.connect()` reads username from localStorage instead of `userStore`
- **File**: `src/stores/player.js:250`
- **Category**: Store coupling
- **Description**: `'user-name': localStorage.getItem(STORAGE_KEYS.USERNAME) || '游客'` bypasses `userStore.currentUser.name` and hardcodes the Chinese fallback.
- **Impact**: Inconsistent auth identity; hardcoded Chinese breaks i18n.

### F-H10 — `topSongsCompat` / `removeSongsCompat` send N messages in tight loop with no cooldown
- **File**: `src/stores/player.js:378-389`
- **Category**: WebSocket / Performance
- **Description**: `queueIds.forEach(queueId => socketService.send(WS_DEST.QUEUE_TOP, { queueId }))` fires N synchronous sends, bypassing `sendControl`'s cooldown.
- **Impact**: Server-side rate limiting; potential backpressure disconnects.

### F-H11 — `useToast().register()` is deprecated dead code, but `App.vue` still wires a ref through it
- **File**: `src/composables/useToast.js:8-10`; `src/App.vue:201, 421-423, 453`
- **Category**: Dead code / Coupling
- **Description**: `register` only logs a deprecation warning. `App.vue` still declares `toastInstance = ref(null)`, passes it, and calls `register(toastInstance.value)`.
- **Impact**: Confusing dead wiring; console warning on every app boot.

### F-H12 — `sessionToken` and `publicId` are module-level refs outside Pinia state
- **File**: `src/stores/user.js:7-8`
- **Category**: State management
- **Description**: Declared at module scope, outside `defineStore`. Invisible to Pinia DevTools, not subject to `$reset`/`mapState`.
- **Impact**: DevTools opacity; hard to reset/test.

### F-H13 — Pervasive hardcoded user-facing strings break i18n
- **Files**: `player.js:141-145, 154-158, 164, 182, 189, 250`; `socketHandler.js:24, 36, 45, 52-53, 63, 82-92`; `AuthOverlay.vue:141, 152-153`; `useShortcuts.js:66-72`; `useChatViewModel.js:15`; `ChatOverlay.vue:170`; `userPlaylists.js:42`; `user.js:20, 168`; `MobileNowPlaying.vue:10`; `NamePromptModal.vue:53`
- **Category**: i18n
- **Description**: The app has a full `i18n` setup, yet a large fraction of user-visible strings are hardcoded Chinese/English literals. Switching to English leaves half the UI in Chinese.
- **Impact**: Locale switching is broken for affected strings.

### F-H14 — `ChatModule.vue` `chatTabs` is non-reactive constant; won't update on locale change
- **File**: `src/components/modules/ChatModule.vue:122-126`
- **Category**: i18n / Reactivity
- **Description**: `const chatTabs = [{ value: CHAT_TAB, label: t('chat.tabChat') }, ...]` is a plain array created once. `t()` is called once and frozen. `ChatOverlay.vue` correctly uses `computed(() => [...])`.
- **Impact**: Changing language at runtime does not update chat tab labels in docked ChatModule.

### F-H15 — `userStore.onNameSetCallback` is a single slot, silently overwritten
- **File**: `src/stores/user.js:28, 77-80, 152-154`
- **Category**: State management
- **Description**: `setPostNameAction(fn)` stores a single callback. Multiple components racing to set a post-name action overwrite each other.
- **Impact**: Lost post-name actions; user clicks "Create Room" as guest, then opens search — create flow never runs.

### F-H16 — `room.currentRoom` computed returns fresh object literal on every access when no room matches
- **File**: `src/stores/room.js:31-36`
- **Category**: Performance / Reactivity
- **Description**: When `rooms.value` is empty, the computed returns `new { roomId: DEFAULT_ROOM_ID, ... }` every time. Object identity changes each evaluation → spurious re-renders.
- **Impact**: Spurious re-renders of `MainLayout`, `MobileNowPlaying` while `rooms` is loading.

### F-H17 — `AudioEngine` uses `crossorigin="anonymous"` unconditionally
- **File**: `src/components/AudioEngine.vue:7`
- **Category**: Audio
- **Description**: For cross-origin media URLs, this requires the media server to send `Access-Control-Allow-Origin`; if it doesn't, the browser blocks playback entirely.
- **Impact**: Some platforms' streams may fail to play due to CORS.

## Security

### S-H1 — First registered account becomes ADMIN, with no rate limiting / lockout
- **File**: `AccountService.java:45`; `AccountAuthController.java:37-53`; `AccountService.java:68-88`
- **Category**: Privilege Escalation / Brute Force
- **Description**: `GET /api/account/status` (unauthenticated) returns whether an admin exists. On a fresh instance, the first `POST /api/account/register` grants `ADMIN`. `login` performs no attempt counting. `AuthConfig.maxAttempts/windowSeconds/blockDurationSeconds` are **defined but never referenced**.
- **Attack scenario**: (a) Attacker registers first on a fresh public instance → full admin. (b) Unlimited BCrypt password guessing with no lockout.
- **Impact**: Full admin takeover of freshly deployed public instances; credential brute-forcing.
- **Decision**: **Fix now** — implement rate limiting using already-defined `AuthConfig`.

### S-H2 — Chat content has no server-side sanitization (stored XSS vector)
- **File**: `ChatService.java:149-168, 398-407, 409-414`
- **Category**: Stored XSS / Input Validation
- **Description**: `handleRoomChat`/`handlePublicChat` only `trim()`, length-check, and rate-limit. Raw `content` is persisted and broadcast verbatim. No HTML/entity escaping.
- **Impact**: If frontend ever renders chat with `v-html`/`innerHTML`, stored XSS → account takeover. **Not exploitable with current Vue `{{ }}` rendering** (verified: no `v-html` in codebase).
- **Decision**: **Defer** — not a live exploit; document as defense-in-depth gap.

### S-H3 — WebSocket handshake credentials passed as URL query parameters
- **File**: `WebSocketConfig.java:57-83, 115-132`
- **Category**: Information Disclosure / Credential Leakage
- **Description**: `session-token`, `room-access-token`, `user-name` are read from the WebSocket handshake URL query string. Query strings are logged by reverse proxies, browser history.
- **Impact**: Long-lived credential exposure via logs.
- **Decision**: **Defer to backlog** — you control the reverse proxy logs.

## Test & Config

### T-H1 — Frontend "component tests" are source-string matchers, not behavioral
- **Area**: Frontend tests / Test quality
- **Evidence**: `AuthOverlay.test.js`, `a11yControls.test.js`, `desktopShellAdaptation.test.js`, `personalInfoPanelControls.test.js` all use `readFileSync` and assert substrings. No Vue component is ever mounted. Grep for `mount(|@vue/test-utils|@testing-library` returns **zero** matches.
- **Risk**: Zero confidence that components render correctly. Tests pass even if component is completely broken.

### T-H2 — `music.test.js` and other API-wrapper tests assert call-URLs, not behavior
- **Area**: Frontend tests / Test quality
- **Risk**: Tests break on param-ordering changes and pass when response handling is broken.

### T-H3 — Critical backend services with NO test at all
- **Area**: Test coverage gap (backend)
- **Evidence**: `SocketRateLimiter`, `SiteSecretService`, `SiteSettingService`, `LocalLibraryAccessService`, `LocalTranscodeService`, `RoomStateMutationService`, `RoomStatePersistenceService`, `BilibiliMusicApiService`, `BilibiliWbiService`, `SubsonicClient`, `SubsonicMusicApiService`, `ClientIpResolver`, `ReactiveSocketBroker`, `WebSocketBroadcaster`, `WebSocketConfig` — all untested.
- **Risk**: Abuse-prevention, secret-management, and proxy layers have the thinnest coverage.

### T-H4 — All JDBC repositories are untested (only InMemory doubles are exercised)
- **Area**: Test coverage gap (backend persistence)
- **Evidence**: 13 `Jdbc*Repository` classes. No test uses a real SQLite connection to validate SQL, row-mapping, or schema migrations.
- **Risk**: SQL errors, wrong column bindings, schema-drift caught only at runtime in production.

### T-H5 — `playNextInQueue` / download-failure-recovery edge cases under-tested
- **Area**: Test coverage gap (backend, critical path)
- **Evidence**: Untested: `pollNext` when all items PENDING, `pollFromHistory` AutoDJ path, `pollNextFairShuffle` round-robin, pending-download timeout path, re-resolve-after-download-failed error branch, concurrent `add`/`poll` races.
- **Risk**: Core playback scheduler has weak coverage.

### T-H6 — `MusicPlayerServicePendingDownloadTest` uses flaky timing patterns
- **Area**: Test quality (backend)
- **Evidence**: `pollUntil(...)` busy-waits with `Thread.sleep(100)` and hard-coded timeouts (3000/8000/12000 ms). No `Awaitility`. Relies on real scheduled executor timing inside a unit test.
- **Risk**: Tests are slow and flaky under CI load.

### T-H7 — Floating image tags and no version pinning in compose
- **Area**: Configuration audit / docker-compose
- **Evidence**: `docker-compose.yml` uses `moefurina/ncm-api:latest` and `rclone/rclone:latest` and `deluan/navidrome:latest`. No `mem_limit`/`cpus`, no `healthcheck`, no dependency `condition: service_healthy`.
- **Risk**: `:latest` tags cause silent breaking changes on next `pull`.

### T-H8 — Dockerfile Stage 1 uses `npm install` while CI uses `npm ci`
- **Area**: Configuration audit / Build reproducibility
- **Evidence**: `Dockerfile` line 16 `RUN npm install`. `.github/workflows/quality.yml` line 35 `npm ci`.
- **Risk**: Production frontend deps can diverge from what CI tested.

---

# MEDIUM Findings (78+)

## Backend Services (42+ — truncated; key items below)

### B-M1 — `pendingDownloadPoller` field is non-volatile, mutated without synchronization
- **File**: `MusicPlayerService.java:310, 423, 426, 490-495`
- **Impact**: Cancel may not be visible to concurrent poller tick.

### B-M2 — `resetSystem` mutates in-memory state before the transaction; if transaction fails, memory and DB diverge
- **File**: `MusicPlayerService.java:743-767`
- **Impact**: On DB error, room is wiped in memory but restores to pre-reset state on restart.

### B-M3 — `RoomPlaybackState` synchronized getters give false safety; external callers chain them non-atomically
- **File**: `RoomPlaybackState.java:38-48`
- **Impact**: `setCurrentTrack(refreshed, enqueuerId, enqueuerName)` can use enqueuerId from before a concurrent `startNewTrack`.

### B-M4 — `cleanupIdlePlayer` idle-reset timeout can be defeated by repeatedly seeking an idle-paused room
- **File**: `MusicPlayerService.java:817-829`
- **Impact**: Stale track loaded forever.

### B-M5 — `ChatService.canUserSendMessage` check-then-act race
- **File**: `ChatService.java:63-74`
- **Impact**: Chat rate limit bypassable by concurrent sends.

### B-M6 — `trimHistory` calls `history.size()` (O(n)) on `ConcurrentLinkedDeque` in a while loop
- **File**: `ChatService.java:187-191`
- **Impact**: O(n²) per message; high CPU on large chat histories.

### B-M7 — `deleteRoomData` performs 4 repository deletes without a transaction
- **File**: `RoomStatePersistenceService.java:91-96`
- **Impact**: Partial cleanup leaves orphaned rows on failure.

### B-M8 — Events published inside the transaction
- **File**: `PlaybackTransitionService.java:22-32`
- **Impact**: Synchronous event listeners hold the transaction open; re-entrant DB access can deadlock.

### B-M9 — `getApiService` for subsonic platforms allocates new anonymous `IMusicApiService` on every call
- **File**: `MusicPlayerService.java:1102-1164`
- **Impact**: Per-call allocation; minor GC pressure.

### B-M10 — `MusicSocketController.dispatch` `objectMapper.convertValue` can throw on malformed payloads; no try/catch
- **File**: `MusicSocketController.java:59-89`
- **Impact**: A single malformed frame disconnects the client's session instead of returning a structured error.

### B-M11 — `AdminController.request.command()` can be null; `.trim()` NPE
- **File**: `AdminController.java:64-74`
- **Impact**: Ungraceful 500 instead of 400 on malformed admin command.

### B-M12 — No bound validation on `offset`/`limit`/`keyword` length
- **File**: `ApiController.java:113-139, 160-181, 206-212`
- **Impact**: Upstream API abuse, log injection via long keywords, DoS via huge `limit`.

### B-M13 — `extractCoverColor` SSRF via misconfigured baseUrl
- **File**: `ApiController.java:243-248`
- **Impact**: Limited SSRF via misconfigured baseUrl; cover path prefixes are enumerable.

### B-M14 — `verifyRoomAccess` has no rate limiting; BCrypt `matches` is the only throttle
- **File**: `RoomController.java:44-52`
- **Impact**: Private room passwords brute-forceable; ~10 passwords/sec.

### B-M15 — `LocalTrackController.cover` endpoint has no auth check
- **File**: `LocalTrackController.java:143-164`
- **Impact**: Information disclosure of local library cover art.

### B-M16 — `SubsonicSourceRegistry.roomLocks` accumulates one lock Object per room forever
- **File**: `SubsonicSourceRegistry.java:39, 412-414`
- **Impact**: Slow memory growth proportional to total rooms ever created.

### B-M17 — `RoomService.listRooms()` uses `Stream.peek` for cache refresh
- **File**: `RoomService.java:64-72`
- **Impact**: Future short-circuiting would silently break cache refresh.

### B-M18 — `UserService` three ConcurrentHashMaps updated non-atomically
- **File**: `UserService.java:64-123`
- **Impact**: Rare double-connect races leave user in half-registered state.

### B-M19 — `User` object shared and mutated without `volatile` fields
- **File**: `UserService.java:129-163`
- **Impact**: Visibility issues; `getOnlineCount` may over/under-count briefly.

### B-M20 — `@Scheduled` methods share the default single-thread `TaskScheduler`
- **File**: `MusicPlayerService.java:60`
- **Impact**: A slow DB write in `playerLoop` delays cache cleanup, user expiry, rate-limiter cleanup — cascading staleness.

### B-M21 — `RoomPlayerSession` is a non-static inner class holding implicit reference to `MusicPlayerService`
- **File**: `MusicPlayerService.java:60`
- **Impact**: Not testable in isolation; couples to god-class.

### B-M22 — `enqueue` triggers `prefetchMusic` before `queueManager.add` may reject (queue full)
- **File**: `MusicPlayerService.java:507-533`
- **Impact**: Wasted downloads for rejected enqueues.

### B-M23 — `likedUserIds` + `likeMarkers` updated as a pair non-atomically
- **File**: `RoomPlaybackState.java:32-33`
- **Impact**: Like-marker order non-deterministic.

### B-M24 — `handleDownloadEvent` iterates `getQueueSnapshot()` (not synchronized, B-H14) to check membership
- **File**: `MusicPlayerService.java:775-788`
- **Impact**: Download-complete event may fail to trigger playback when queue is being mutated.

### B-M27 — `BilibiliProxyController.cdnUrlCache` ConcurrentHashMap unbounded; stale entries only removed on re-resolve
- **File**: `BilibiliProxyController.java:51`
- **Impact**: Slow growth proportional to distinct BVIDs ever streamed.

*(Backend MEDIUM findings M28-M42+ were truncated in the audit output — additional items include: NeteaseMusicApiService cookie validation fire-and-forget, RoomPlaybackState startNewTrack atomicity undermined by unsynchronized callers, RoomStatePersistenceService non-transactional persistPlaybackState, RoomPlayerSession pendingDownloadPoller DB transactions on 2-thread scheduler, BilibiliMusicApiService sessdata check-then-set, and more. Full backend audit output saved at `tool-output/tool_f366f9412001GX5wdOuVnwU6Y5`)*

## Frontend (21)

### F-M1 — `api/client.js` has no retry, no token refresh, no global auth header
- **File**: `src/api/client.js:1-19`
- **Impact**: Transient 5xx errors fail immediately; no resilience.

### F-M2 — `onConnectionProfileChange` listener never unsubscribed
- **File**: `src/api/client.js:9-12`
- **Impact**: Minor; latent leak if client module re-imported (HMR).

### F-M3 — `requestChatHistory` lacks the `initial` guard that `requestPublicChatHistory` has
- **File**: `src/stores/player.js:528-542`
- **Impact**: On reconnect, room chat history is reset to latest 50, losing pagination state.

### F-M4 — `chat.addMessage` performs O(n) slice on every message once over 2000
- **File**: `src/stores/chat.js:25-30, 39-43`
- **Impact**: Minor GC pressure in very active rooms.

### F-M5 — `layout.setColumnCount` generates column ids from `Date.now()`
- **File**: `src/stores/layout.js:134`
- **Impact**: Two columns added in same tick produce identical ids.

### F-M6 — `layout.js` deep-watches the entire layout object
- **File**: `src/stores/layout.js:241-243`
- **Impact**: O(n) deep traversal per drag frame + localStorage serialization → jank during column resize.

### F-M7 — `updateBufferedMs` doesn't guard `Infinity`/`NaN` `audio.duration`
- **File**: `src/composables/useAudio.js:58-60`
- **Impact**: `bufferedMs` can become `NaN`, breaking progress bar width.

### F-M8 — `updateMediaSession` hardcodes artwork MIME type as `image/png`
- **File**: `src/composables/useAudio.js:313-315, 409`
- **Impact**: Incorrect/missing lock-screen artwork (covers are frequently jpg/webp).

### F-M9 — Sync loop applies `playbackRate` 1.03/0.97 for drift correction (audible pitch shift)
- **File**: `src/composables/useAudio.js:542-543`
- **Impact**: Audible pitch wobble during sync corrections.

### F-M10 — `AudioVisualizer.js` is procedural canvas (not real spectrum) and appears unused
- **File**: `src/logic/AudioVisualizer.js`
- **Impact**: Misleading naming; potential dead code inflating bundle.

### F-M11 — `NowPlayingModule.vue` references `player.value` (store is not a ref)
- **File**: `src/components/modules/NowPlayingModule.vue:239`
- **Impact**: No functional bug (fallback works), but misleading dead code.

### F-M12 — `SubsonicSourceManager.moveSource` swaps `sortOrder` via two sequential awaits with no rollback
- **File**: `src/components/SubsonicSourceManager.vue:243-250`
- **Impact**: Duplicate sort orders if second call fails.

### F-M13 — `socketHandler.onConnect` re-binds every binding on every connect
- **File**: `src/services/socketHandler.js:201-203`
- **Impact**: Wasted bandwidth/processing on every reconnect.

### F-M14 — `player.resetRoomState` leaves several sync/health fields unreset
- **File**: `src/stores/player.js:297-317`
- **Impact**: Stale error/buffering UI and cooldown bleed into new room.

### F-M15 — `ui.fetchConfig` swallows errors silently
- **File**: `src/stores/ui.js:208-216`
- **Impact**: Footer/branding silently shows defaults if config endpoint fails.

### F-M16 — `userPlaylists.tracksByPlaylist` and `roomPlaylists.tracksByPlaylist` never pruned
- **File**: `src/stores/userPlaylists.js:72`, `src/stores/roomPlaylists.js:36-39`
- **Impact**: Slow memory growth for heavy playlist browsers.

### F-M17 — `NamePromptModal` has no optimistic update or local error feedback
- **File**: `src/components/NamePromptModal.vue:49-60`
- **Impact**: Users get no in-modal feedback if rename fails for non-"taken" reason.

### F-M18 — `useSearchLogic` caches full search results in localStorage unbounded
- **File**: `src/composables/useSearchLogic.js:20-21, 107, 113`
- **Impact**: `QuotaExceededError` on `setItem` (uncaught) → search crash.

### F-M19 — `useAudio` sync loop double-writes `setPlaybackPosition`
- **File**: `src/composables/useAudio.js:516` and `src/components/AudioEngine.vue:68-70`
- **Impact**: Redundant reactivity; minor CPU on every 200ms tick.

### F-M20 — `player.js` lyric `watch` and `lyricCache` Map are never disposed
- **File**: `src/stores/player.js:37, 546-604`
- **Impact**: Acceptable for singleton, but breaks if store ever disposed/recreated.

### F-M21 — `socketService.send` has no outbound queue
- **File**: `src/services/socket.js:90-105`
- **Impact**: Enqueue/rename/bind during brief disconnect is lost with no retry.

## Security (5)

### S-M1 — Missing path-containment check on served file paths
- **File**: `LocalTrackController.java:134, 156`; `LocalLibraryService.java:162-173`
- **Category**: Path Traversal (defense-in-depth gap)
- **Description**: `root().resolve(track.oggPath()).normalize()` does not verify `path.startsWith(root())`. Today `oggPath` is set server-side, but any future bug or DB manipulation would yield arbitrary file read.
- **Decision**: **Fix now** — add `path.startsWith(root())` check.

### S-M2 — Proxy controllers hardcode `Access-Control-Allow-Origin: *` on media responses
- **File**: `SubsonicProxyController.java:108-109`; `NavidromeProxyController.java:100-101`; `NeteaseProxyController.java:102-103`; `LocalTrackController.java:197-198`; `LocalResourceConfig.java:34-38`
- **Category**: CORS / CSRF surface
- **Impact**: Cross-origin audio exfiltration once a token is leaked.

### S-M3 — Admin endpoints accept credentials as `@RequestParam` (URL query) on GET
- **File**: `AdminController.java:177, 225`; `LocalTrackController.java:41-42, 81-83, 92-93`; `UserPlaylistController.java` (entire file)
- **Category**: Credential in URL / Information Disclosure
- **Impact**: Session/admin token leakage via logs/history.

### S-M4 — `InternalStreamProxyToken` comparison is non-constant-time
- **File**: `InternalStreamProxyToken.java:24-26`
- **Category**: Timing side channel
- **Decision**: **Fix now** — use `SecureCompare.equals`.

### S-M5 — No username-impersonation guard on WebSocket connect (`user-name` query)
- **File**: `UserService.java:64-123, 366-377`; `WebSocketConfig.java:80`
- **Category**: Impersonation / Spoofing
- **Description**: `handleConnect` uses the query-supplied `user-name` directly as display name. No check prevents connecting with `user-name=SYSTEM` or another user's display name.
- **Impact**: Spoofing of system/admin identity in chat; confusion attacks.

## Test & Config (10)

### T-M1 — No Spring Security; auth is entirely hand-rolled
- **Area**: Configuration audit / pom.xml
- **Evidence**: `pom.xml` declares only `spring-security-crypto`, not `spring-boot-starter-security`. No framework-provided CORS filter, CSRF protection, session management.
- **Risk**: No centralized authorization enforcement; every controller must remember to check manually.

### T-M2 — No backend static-analysis / vulnerability scanning
- **Area**: Build/CI
- **Evidence**: No SpotBugs, Spotless, Checkstyle, Error Prone, or OWASP dependency-check. CI runs only `mvn test`. Spring Boot 3.2.5 is from May 2024; later 3.2.x/3.3.x patch CVEs.
- **Risk**: Known-vulnerable transitive deps never flagged.

### T-M3 — No pre-commit hooks; lint is CI-only
- **Area**: Build/CI
- **Evidence**: No `.husky/`, no `.pre-commit-config.yaml`. Frontend `eslint.config.js` runs only in CI.

### T-M4 — Orphaned E2E test not wired into CI
- **Area**: Build/CI / Frontend tests
- **Evidence**: `playwright-acceptance.js` exists. `package.json` does not declare `@playwright/test`. No `playwright.config.js`. `quality.yml` has no e2e job.
- **Risk**: The acceptance script is dead — cannot run without manual install.

### T-M5 — `vitest.config.js` globally mutates `TMPDIR`/`TEMP`/`TMP` and creates `.tmp`
- **Area**: Test quality (frontend, isolation)
- **Risk**: Test pollution of working tree; `.tmp` not in `.gitignore`.

### T-M6 — `RoomService` core flows only partially tested
- **Area**: Test coverage gap (backend)
- **Risk**: Room lifecycle and access-token invalidation on deletion are security-relevant and unverified.

### T-M7 — `ChatService` has only a performance test
- **Area**: Test coverage gap (backend)
- **Risk**: Chat abuse controls and slash-command routing can regress.

### T-M8 — Documentation drift: undocumented env vars (several security-relevant)
- **Area**: Documentation drift
- **Evidence**: `ROOM_ACCESS_TOKEN_SECRET` (HMAC secret), `TRUSTED_PROXY_CIDRS`, `AUTH_MAX_TRACKED_IPS`, `SQUIDIFY_*` (8 vars), `DB_ENABLED`/`DB_PATH`, `LOCAL_LIBRARY_*`, `APP_MODE` — all undocumented in README.
- **Risk**: `ROOM_ACCESS_TOKEN_SECRET` defaults to empty → random secret that doesn't survive restarts → all room-access tokens invalidate on every restart.

### T-M9 — `application.yml` logging defaults to DEBUG for app package in production
- **Area**: Configuration audit
- **Evidence**: Line 18: `org.thornex.musicparty: DEBUG` in bundled config.
- **Risk**: Verbose production logs may leak request details, session ids.

### T-M10 — `ADMIN_PASSWORD` defaults to empty; bootstrap behavior under-documented
- **Area**: Configuration audit / Documentation drift
- **Risk**: Attacker who reaches a freshly deployed instance before operator can register becomes admin. Combined with T-C1 (no rate limit), first-to-register race is real.

---

# LOW Findings (34+)

## Frontend (20)

- **F-L1**: `userStore.initUser` always returns `false` — dead return
- **F-L2**: `userStore.saveName` is empty deprecated function still exported
- **F-L3**: `useChatViewModel.tabLabel` hardcodes Chinese
- **F-L4**: `player.enqueue` returns `&&` expression result, not boolean
- **F-L5**: `toast.idCounter` never reset
- **F-L6**: Many `<span class="material-symbols-outlined">` icons lack `aria-hidden="true"`
- **F-L7**: `CoverImage` silently upgrades `http://` to `https://` with no fallback
- **F-L8**: `useShortcuts` ignores `<select>` focus check
- **F-L9**: `socketHandler` `handleGameEvent` references `resolveName` from stores but `player.connect` never passes it — dead plumbing
- **F-L10**: `App.vue` `ACTIVITY_EVENTS` listeners fire `recordInteraction` on `pointermove` with no throttle
- **F-L11**: `ChatOverlay`/`ChatModule` define `tabLabel`/`chatTabs` with `'PUBLIC'` literal and `t()` mix
- **F-L12**: `player.reorderQueue` schedules untracked fallback resync
- **F-L14**: `useExternalPlaylist` and `useSearchLogic` create module-scoped cache keys but instance-scoped refs — cross-component interference
- **F-L15**: `layout.js` `swapColumns` mutates `order` but doesn't re-normalize array order
- **F-L16**: `socketService.reconnectNow` setTimeout not cleared on subsequent `disconnect`
- **F-L17**: `player.connect` re-creates handlers capturing stale `useChatStore()` etc.
- **F-L18**: `useAudio` `safePlay` does not reset `transitionFadeInPending` on non-autoplay errors
- **F-L19**: `AppleLyricsPanel` `lineRefs` array is mutated but never truncated on shrink
- **F-L20**: `usePlatforms.onMounted` checks `platforms.length <= 2` to decide whether to load — magic number

## Security (6)

- **S-L1**: `SubsonicCredentialCipher.decrypt` catches all exceptions and returns `""` — fail-open masks tampering (crypto itself sound)
- **S-L2**: `RoomAccessService` random signing secret when not configured — breaks multi-instance / restarts (HMAC correct)
- **S-L3**: `CoverColorService` residual DNS-rebinding SSRF on unauthenticated cover-color endpoint
- **S-L4**: `JdbcLocalTrackRepository.deleteLocalReferences` fragile JSON `LIKE` matching (parameterized — no injection)
- **S-L5**: `SecureCompare` helper correct but not applied at `InternalStreamProxyToken`
- **S-L6**: `ClientIpResolver` — **positive finding**: correct secure-default IP resolution

## Test & Config (8)

- **T-L1**: `ApiControllerTests` passes `null` for several constructor collaborators — brittle to refactor
- **T-L2**: ESLint disables many rules; `no-unused-vars` is warn-only; no Prettier — lint effectively non-blocking
- **T-L3**: `.dockerignore` is minimal — `cookies.json`, `.env.local`, `*.png`, and `docs/` are not excluded
- **T-L4**: Frontend composables with no tests: `useAudio`, `useChatViewModel`, `usePlatforms`, `usePlaylistLogic`, `useQueueSelection`, `useShortcuts`, `useToast`
- **T-L5**: `docker-compose.yml` binds `8848:8080` to `0.0.0.0` — reachable on all host interfaces
- **T-L6**: No `healthcheck` in Dockerfile or compose; no graceful-shutdown tuning
- **T-L7**: `SocketRateLimiter` window boundary edge case — untested

---

# Positive Findings

During the audit, several implementations were verified as correct and secure:

- **Command injection**: All `ProcessBuilder` usage builds commands as `List<String>` — no shell concatenation. User input is separate argv elements. No command-injection vector found.
- **SQL injection**: All `Jdbc*Repository` usages of parameterized queries (`?` placeholders). No user data interpolated into SQL text.
- `RoomAccessService` token crypto: HMAC-SHA256 + constant-time compare (`MessageDigest.isEqual`) + payload structure validation (room id, public id, expiry, password version). Correct.
- `AccountService` password storage: BCrypt via `BCryptPasswordEncoder`; session tokens hashed (SHA-256) before persistence. Correct.
- WebSocket connect auth: `WebSocketConfig.authorize` correctly validates account session token and room-access token. Correct.
- Admin auth: `AdminAuthorizationService.isAuthorized` correctly ignores legacy `adminPassword` (always returns `false`) and requires real admin session token. Correct.
- `ClientIpResolver`: Only trusts `X-Forwarded-For`/`X-Real-IP` when direct peer is within configured trusted-proxy CIDR (default empty → trusts nothing). Correct secure defaults.

---

# Remediation Priority

## Fix Now (~15 findings)

### Security (5)
1. **S-C1**: Add `sessionToken` + ownership check to `RoomPlaylistController` mutations
2. **S-C2**: Gate `LocalTrackController.media`/`cover` behind session-token check
3. **S-H1 / T-C1**: Implement login rate limiting using already-defined `AuthConfig`
4. **S-M1**: Add `path.startsWith(root())` containment on local file serving
5. **S-M4 / B-H19**: Use `SecureCompare.equals` in `InternalStreamProxyToken.matches`

### Backend thread-safety (3)
6. **B-C3**: Wrap all `.subscribe()` callback bodies in `.publishOn(Schedulers.boundedElastic())`
7. **B-C5**: Make `skipToNext`, `togglePause`, `toggleShuffle` `synchronized`
8. **B-C6**: Synchronize all `RoomPlaybackState` field setters + add composite getter

### Frontend (2)
9. **F-C1**: Add `playPrevious`, `toggleRepeat`, `repeatMode` to player store (or remove dead buttons)
10. **F-C5**: Track all retry timers and clear on track-change/unmount

### Config (1)
11. **T-C7**: Add non-root `USER` + `HEALTHCHECK` to Dockerfile

## Deferred to Backlog (all other findings)

- All MEDIUM/LOW findings
- WebSocket credential in URL (S-H3 / F-C2) — deferred per decision
- Chat XSS (S-H2) — deferred (not exploitable with current Vue `{{ }}` rendering)
- All 8 CRITICAL test coverage gaps (T-C2 through T-C6, T-C8) — documented as backlog
- i18n hardcoded strings (F-H13, F-H14) — documented as backlog
- Full thread-safety refactor (per-room event loop, Option B) — documented as backlog
- Backend performance optimizations (B-H4 through B-H6) — documented as backlog
- Dockerfile `npm ci` (T-H8), image tag pinning (T-H7) — documented as backlog
- Spring Security adoption (T-M1) — documented as long-term backlog
