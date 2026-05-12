# MusicParty Handoff to 201

Date: 2026-05-12

Working branch: `NRT-Base`

Current local root:

`C:\Users\Nirotiy\Documents\NRT-GIT\musicparty\MusicParty`

Remote layout:

- `origin` = `https://github.com/Nirotiy/MusicParty.git`
- `upstream` = `https://github.com/PluvIIter/MusicParty.git`

## Purpose of This Handoff

This file is for 201 and their agent. The requested focus is core playback/streaming optimization, especially the HTTP live stream path and synchronization work tracked in `ROADMAP.md`.

The current owner is still actively working on frontend/mobile UX. Avoid overlapping UI work unless explicitly asked.

## Current Product Direction

MusicParty is moving away from the old console/HUD style into a cleaner music-player experience:

- Apple Music / Spotify inspired layout
- less chrome
- stronger surface hierarchy
- theme-aware dynamic accent colors
- custom synchronized lyrics instead of AMLL
- separate desktop and mobile UI paths

Do not revert the app toward the older industrial/control-console look.

## Current Git / Worktree Notes

There are active local changes in progress. Do not reset, discard, or rewrite them.

Known local status at handoff time:

- Modified frontend and backend files for lyrics, mobile controls, favorites, and roadmap.
- `data/queue-data.json` is modified runtime state; treat it as unrelated unless asked.
- `cached_media/` and `packaging/` are untracked local directories; do not delete them.
- New files currently include:
  - `src/main/java/org/thornex/musicparty/dto/LyricResponse.java`
  - `music-party-web/src/utils/likedSongs.js`
  - this file, `HANDOFF_TO_201.md`

Recommended first commands:

```powershell
git status --short --branch
git diff --stat
```

## What Was Recently Implemented

### Structured lyrics and translated lyrics

Old lyrics endpoint remains:

- `GET /api/music/lyric/{platform}/{musicId}`
- still returns plain LRC text

New lyrics detail endpoint:

- `GET /api/music/lyric-detail/{platform}/{musicId}`
- returns `LyricResponse`

DTO:

- `src/main/java/org/thornex/musicparty/dto/LyricResponse.java`

Fields:

- `lyric`
- `translatedLyric`
- `romanizedLyric`

NetEase support:

- `NeteaseMusicApiService` now reads:
  - `lrc.lyric`
  - `tlyric.lyric`
  - `romalrc.lyric`

Bilibili lyrics:

- Intentionally not supported.
- Do not spend time investigating Bilibili lyrics APIs.
- In this project Bilibili is treated as video-to-audio extraction, not a lyrics source.

Frontend lyrics changes:

- `music-party-web/src/utils/parser.js`
  - `parseLyrics()` preserved
  - `mergeTranslatedLyrics()` added
- `music-party-web/src/stores/player.js`
  - `lyricText` preserved for compatibility
  - `lyricDetail` added
- `AppleLyricsPanel.vue`
  - now supports primary lyric + translated lyric
- `CenterConsole.vue`
  - desktop lyrics pass translated lyrics
- `MobileMiniLyrics.vue`
  - mobile mini lyric supports translation line
- `MobileNowPlaying.vue`
  - mobile expanded lyrics pass translated lyrics too

### Mobile UI additions

Mobile UI lives under:

`music-party-web/src/components/mobile/`

Current mobile layout:

- `MobileLayout.vue`
- `MobileBottomNav.vue`
- `MobileNowPlaying.vue`
- `MobileQueueView.vue`
- `MobileSearchView.vue`
- `MobileChatView.vue`
- `MobileMiniLyrics.vue`
- `MobilePreviewShell.vue`

Preview:

```text
http://localhost:5173/?mobilePreview=1
```

Recent mobile now-playing additions:

- vertical volume popover in `MobileNowPlaying.vue`
- volume uses `uiStore.volume`, already watched by `AudioEngine.vue`
- five-slot centered control row:
  - volume
  - shuffle
  - play/pause
  - next
  - heart/favorite

Recent mobile favorite additions:

- red heart button in mobile now-playing
- liked songs stored in localStorage under `mp_liked_songs`
- storage key declared in `music-party-web/src/constants/keys.js`
- data and actions live in `music-party-web/src/stores/player.js`
- mobile queue view has tabs:
  - queue
  - liked songs
- liked songs can be exported as `.txt`

Shared export utility:

- `music-party-web/src/utils/likedSongs.js`

Export format:

- NetEase:
  - `Artist / Artist - Title - https://music.163.com/#/song?id=ID`
- Bilibili:
  - `UP主 - Video Title - https://www.bilibili.com/video/BVID`
- fallback:
  - `Artist - Title`

The favorite/export logic is intentionally shared so desktop can reuse it later.

Desktop favorite work is not done yet. It is tracked in `ROADMAP.md`.

## Current Validation Already Run

Frontend build passed after the latest mobile and favorite work:

```powershell
cd music-party-web
npm run build
```

Backend package passed after lyrics detail work:

```powershell
$env:JAVA_HOME='C:\Program Files\Microsoft\jdk-21.0.11.10-hotspot'
$env:PATH="$env:JAVA_HOME\bin;$env:PATH"
.\mvnw.cmd -DskipTests package
```

`git diff --check` showed no whitespace errors, only Windows line-ending warnings.

## Areas 201 Should Focus On

The most relevant roadmap items for 201 are under Audio and Synchronization plus Streaming.

### 1. HTTP stream broadcaster stability

Current roadmap item:

- Revisit the HTTP stream broadcaster later:
  - isolate each listener with its own bounded writer queue
  - drop slow clients instead of blocking all listeners
  - add better stream keepalive behavior for paused playback if target clients require it

Primary file:

- `src/main/java/org/thornex/musicparty/service/stream/LiveStreamService.java`

Related backend files:

- `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`
- `src/main/java/org/thornex/musicparty/service/LocalCacheService.java`
- `src/main/java/org/thornex/musicparty/service/api/BilibiliMusicApiService.java`
- `src/main/java/org/thornex/musicparty/service/api/NeteaseMusicApiService.java`
- `src/main/java/org/thornex/musicparty/event/StreamStatusEvent.java`

Important product constraint:

- The HTTP live stream path should not get browser-style crossfade unless the server-side ffmpeg pipeline is deliberately redesigned.
- Browser playback and HTTP live stream are different paths.

### 2. Multi-device synchronization

Current roadmap item:

- Add ping/pong RTT measurement for each client.
- Smooth server clock offset instead of using only the latest `serverTimestamp`.
- Use playback-rate correction for small drift, hard seek only for large drift.
- Trigger immediate resync after visibility restore, network restore, and reconnect.
- Add a playback epoch/state version so stale state packets cannot override newer playback state.

Relevant frontend files:

- `music-party-web/src/stores/player.js`
- `music-party-web/src/composables/useAudio.js`
- `music-party-web/src/components/AudioEngine.vue`
- `music-party-web/src/services/socket.js`
- `music-party-web/src/services/socketHandler.js`

Relevant backend files:

- `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`
- `src/main/java/org/thornex/musicparty/dto/PlayerState.java`
- `src/main/java/org/thornex/musicparty/dto/NowPlayingInfo.java`
- `src/main/java/org/thornex/musicparty/controller/MusicSocketController.java`

Current sync baseline:

- Backend sends `serverTimestamp` in `PlayerState`.
- Frontend computes `serverClockOffset = serverTimestamp - clientReceiveTime`.
- Frontend estimates current progress using `remotePosition`, `lastSyncTime`, and `serverClockOffset`.
- This is better than raw client time, but still simple and not RTT-smoothed.

### 3. Web player seek smoothing

Current roadmap item:

- Fade volume down before applying `audio.currentTime`.
- Wait for `seeked` or `canplay`.
- Fade volume back in after target range is playable.
- Skip hard seeking for very small drift and let the player converge naturally.

Relevant files:

- `music-party-web/src/composables/useAudio.js`
- `music-party-web/src/components/AudioEngine.vue`
- `music-party-web/src/stores/player.js`

Constraint:

- The mobile seek permission behavior is already implemented: only the current song enqueuer can seek.
- Preserve that permission model.

### 4. Browser track transition fades

Current roadmap item:

- Add web-player track transition fades for NetEase and Bilibili playback.
- Fade out near the end of current track.
- Fade in after next track is ready.
- Scope this to browser playback.

Do not mix this with HTTP stream crossfade unless explicitly asked.

## Areas to Avoid Unless Asked

Avoid changing these during stream optimization work:

- desktop/mobile lyrics UI
- `AppleLyricsPanel.vue`
- `MobileMiniLyrics.vue`
- mobile favorite/export UI
- `likedSongs.js`
- dynamic accent/theme behavior
- mobile layout routing in `App.vue`
- queue/search/chat mobile views

Reason: the current owner is still iterating frontend UX and plans to move the favorite/export feature to desktop next.

## Bilibili Source Notes

For this project, Bilibili support means:

- search/favorite folders provide video metadata
- `id` is usually the BVID
- `name` is video title
- `artists` is author/UP 主
- playback uses video metadata and then ffmpeg/local cache/audio extraction

Do not treat Bilibili as a lyrics source.

Potentially relevant files:

- `src/main/java/org/thornex/musicparty/service/api/BilibiliMusicApiService.java`
- `src/main/java/org/thornex/musicparty/util/BilibiliApiUtils.java`
- `src/main/java/org/thornex/musicparty/service/LocalCacheService.java`
- `cached_media/` local cache directory may exist

## Build and Run Commands

Frontend:

```powershell
cd music-party-web
npm install
npm run build
npm run dev -- --host 0.0.0.0
```

Backend:

```powershell
$env:JAVA_HOME='C:\Program Files\Microsoft\jdk-21.0.11.10-hotspot'
$env:PATH="$env:JAVA_HOME\bin;$env:PATH"
.\mvnw.cmd -DskipTests package
```

Docker deployment:

```bash
docker compose up -d --build
```

Default Docker app URL:

```text
http://localhost:8848
```

## Suggested Workflow for 201

1. Start with `git status --short --branch`.
2. Read:
   - `ROADMAP.md`
   - `FRIEND_AGENT_HANDOFF.md`
   - `MOBILE_UI_HANDOFF.md`
   - `HANDOFF_TO_201.md`
3. Inspect `LiveStreamService.java` before changing anything.
4. Keep stream optimization changes isolated from UI work.
5. Add focused instrumentation/logging around stream listener behavior if needed.
6. Validate with backend package and at least one local playback/stream scenario.
7. Do not rewrite unrelated local changes.

## Practical Implementation Notes for Stream Work

If implementing the broadcaster rewrite, prefer a conservative design:

- Each HTTP listener gets a bounded queue.
- The ffmpeg/read loop never blocks on a slow listener.
- If a listener queue is full for too long, close that listener.
- Use clear lifecycle cleanup so disconnected clients are removed.
- Log listener count, dropped clients, queue overflow, and ffmpeg restart causes.
- Avoid unbounded byte arrays or per-listener memory growth.

Potential risk areas:

- ffmpeg process lifecycle
- pause/resume behavior
- clients connecting while no song is playing
- Bilibili local cache pending/downloading state
- browser playback and HTTP stream state diverging

## Current Open Product Tasks Not for 201 Unless Asked

- Move liked songs/export affordance to desktop.
- Add explicit desktop heart control.
- Continue mobile UI polish.
- Improve mobile full lyrics page design.
- Add action sheets for mobile queue/search actions.
- Polish mobile chat system events.

## Final Reminder

The user prefers practical incremental changes. For 201's likely scope, stability beats ambitious rewrites. Optimize the stream path in small, verifiable steps and avoid touching the active frontend UX work unless the user redirects the task.
