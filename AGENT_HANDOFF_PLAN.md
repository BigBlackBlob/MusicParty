# MusicParty Agent Handoff Plan

## Context

This document packages the current project understanding and an implementation plan for a follow-up agent.

Project path:

- `D:\Pluvllter-MusicParty\MusicParty-master`

Current user goals:

1. Add draggable seek bar support for synchronized playback.
2. Redesign the UI to feel more modern, cleaner, and more consistent, while preserving current UX flows.
3. Explore replacing risky arbitrary direct-audio-link playback with a more stable media-library integration path, with Navidrome as the primary candidate.
4. Plan a Windows-local development workflow without WSL2, mainly for debugging.

The user does not write code and wants practical feasibility validation as part of the work.

---

## High-Level Architecture

Frontend:

- Vue 3 + Vite + Pinia + Tailwind
- STOMP WebSocket for player state sync

Backend:

- Spring Boot 3.2.5
- WebSocket/STOMP for control and state broadcasting
- Reactive API integrations via `WebClient`
- FFmpeg-based live audio stream relay/transcoding

Core files already identified:

- Frontend player store: `music-party-web/src/stores/player.js`
- Frontend audio sync logic: `music-party-web/src/composables/useAudio.js`
- Frontend bottom player UI: `music-party-web/src/components/PlayerControl.vue`
- Frontend main layout: `music-party-web/src/components/layout/MainLayout.vue`
- Backend player state machine: `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`
- Backend socket control entry: `src/main/java/org/thornex/musicparty/controller/MusicSocketController.java`
- Backend stream relay: `src/main/java/org/thornex/musicparty/service/stream/LiveStreamService.java`
- Audio provider abstraction: `src/main/java/org/thornex/musicparty/service/api/IMusicApiService.java`

---

## Findings by Requirement

### 1. Draggable seek bar with synchronized playback

Current state:

- The bottom player already renders playback progress visually in `PlayerControl.vue`.
- It already has a manual drag implementation for volume, which is a good local pattern to reuse.
- The frontend sync loop already computes authoritative playback time and continuously corrects drift in `useAudio.js`.
- The backend already models playback using `positionAnchor` + `timestampAnchor` in `MusicPlayerService`.
- There is currently no socket command for seek.

Important existing data already available:

- Current song enqueuer is stored on the backend:
  - `currentEnqueuerId`
  - `currentEnqueuerName`
- Current song enqueuer is also sent to clients in `NowPlayingInfo`:
  - `enqueuedById`
  - `enqueuedByName`
- Frontend user identity already has a stable token in `userStore.userToken`.

#### Agreed permission model

Phase 1 seek authorization:

- Only the user who enqueued the current song can seek.
- `SYSTEM` keeps fallback authority.
- Do not add separate admin manual seek in phase 1.

Reasoning:

- The product is effectively being used as a `PUBLIC` room setup.
- Existing admin capability is command-like, not a real role system.
- Owner-only seek is the clearest first rule and avoids conflicts.

#### Expected UX behavior

- All users can see the progress bar.
- Only the current song owner gets an interactive draggable seek bar.
- Non-owner users should see a non-interactive visual state.
- Dragging should preview locally only.
- Only on release should the client send one seek command.
- Server remains authoritative and broadcasts the new position globally.
- If the server rejects the request, show a clear toast.

#### Main implementation risks

- `useAudio.js` currently performs frequent drift correction every 200ms.
- If drag state is not integrated carefully, the UI will fight the user's drag input.

Conclusion:

- This requirement is highly feasible.
- It is a focused cross-frontend/backend change, not a system rewrite.

---

### 2. UI redesign with unchanged UX

User target:

- Keep the existing UX structure and flows.
- Make the UI feel closer to mature music products such as Spotify or Apple Music.
- Modern, cleaner, more consistent.

Current state:

- The app already uses Tailwind heavily.
- The visual design language is custom, hand-rolled, and inconsistent in places.
- The current theme is a light industrial / medical style with many explicit borders and ornamental details.

Important constraint:

- Do not do a full framework migration for UI.
- This should be a design-system refactor, not a product-flow rewrite.

#### Recommended direction

Use a low-intrusion component approach:

- Prefer `shadcn-vue` or `Reka UI` style primitives for selected base components if needed.
- Keep Vue + Tailwind + current application structure.

Do not prioritize:

- Heavy full-component-library migration such as turning the app into an Element Plus or Naive UI application wholesale.

#### Visual direction

Recommended style keywords:

- dark-surface
- soft-elevation
- album-first
- minimal chrome
- strong typography
- subtle accent
- high information hierarchy
- calm motion

Concrete design implications:

- Reduce hard borders.
- Use depth, contrast, spacing, and surface hierarchy instead of industrial line decoration.
- Put visual emphasis on cover art, title, playback state, and queue clarity.
- Make hover and active states more restrained and product-like.
- Standardize spacing, radii, shadows, typography, and density.

#### Recommended rollout order

Batch 1:

- `MainLayout.vue`
- `PlayerControl.vue`
- `SearchModal.vue`

Batch 2:

- `QueueList.vue`
- `QueueItem.vue`
- `UserList.vue`
- `ChatOverlay.vue`
- `ToastNotification.vue`

Batch 3:

- `CenterConsole.vue`
- `TutorialOverlay.vue`
- remaining stylistic cleanup

Conclusion:

- Very feasible.
- Best treated as incremental reskinning with preserved interaction model.

---

### 3. Replace risky arbitrary direct links with media-library integration

The original idea was to support arbitrary audio direct URLs.

Updated direction from the user:

- Avoid the browser and URL risk surface of arbitrary direct links.
- Explore integration with:
  - Navidrome
  - Jellyfin
  - possibly WebDAV

#### Current backend suitability

This codebase already has a provider abstraction:

- `IMusicApiService`

That means the correct architecture is:

- Add a new provider such as `navidrome`
- Reuse the existing search -> enqueue -> play -> broadcast pipeline
- Do not replace the app's own synchronization model

#### Recommended path: Navidrome first

Recommendation:

- Prioritize `Navidrome` as the first integration target.
- Treat `Jellyfin` as a second adapter, not first-phase scope.
- Do not use `WebDAV` as the primary first implementation path.

Why Navidrome:

- It is music-focused.
- It provides stable library semantics.
- It exposes compatible API patterns suitable for:
  - search
  - metadata
  - stream access
  - cover art
  - lyrics in some setups

Why not arbitrary direct URL first:

- CORS
- hotlink restrictions
- missing or unreliable duration metadata
- weak seek behavior on some sources
- SSRF/security concerns if arbitrary server-side fetching is added

Why not WebDAV first:

- It is a file access mechanism, not a music-library model.
- It does not naturally solve:
  - search semantics
  - album/artist metadata structure
  - cover art
  - lyrics
  - music-focused indexing

#### Navidrome phase split

Phase 1:

- Add `platform = navidrome`
- Search songs
- Enqueue songs
- Resolve playable stream
- Display cover art
- Reuse current sync and stream-relay pipeline

Phase 2:

- Lyrics
- playlists / collections / imports
- account binding
- richer browsing views

#### Important architectural rule

Navidrome should provide media and metadata only.

This application must remain the owner of:

- room synchronization
- playback authority
- queue behavior
- seek authorization rules
- live stream rebroadcast behavior

Conclusion:

- Navidrome integration is meaningfully more feasible than arbitrary direct-link playback.
- It aligns well with the current provider abstraction.

---

## Windows Local Debugging Without WSL2

User requirement:

- Local deployment should be practical on Windows.
- No WSL2.
- Main purpose is debugging, not production hardening.

### Feasibility conclusion

This is feasible.

This project does not require WSL2 to debug locally.

### Core local dependencies

Minimum useful local stack:

- JDK 21
- Maven 3.x or Maven Wrapper
- Node.js 18+
- npm

For more complete testing:

- FFmpeg in `PATH`
- one running music source backend, currently expected:
  - NeteaseCloudMusicApi instance

### Observed local-friendly defaults

- Backend default port is `8080`
- Vite proxy already points to `http://localhost:8080`
- local media path uses project-local directories such as:
  - `cached_media`
  - `data`
- FFmpeg path defaults to `ffmpeg`, so PATH-based installation works on Windows

### Recommended local modes

#### Mode A: minimal debug mode

Use this first.

- Run frontend locally with Vite
- Run backend locally with Spring Boot
- Skip testing live stream at first
- Skip Bilibili cache-heavy flows at first
- Test only core sync flows

#### Mode B: standard debug mode

- Frontend locally
- Backend locally
- Local `NeteaseCloudMusicApi`
- FFmpeg installed in PATH
- Test live stream and more complete flows

#### Mode C: long-term stable debug mode

- Frontend locally
- Backend locally
- FFmpeg in PATH
- Add local Navidrome server with a real music library
- Reduce dependence on external music source instability during feature development

### Practical recommendation

For this codebase, the most useful long-term Windows debug setup is:

- local frontend
- local backend
- FFmpeg installed normally on Windows
- Navidrome as the stable local media source for future provider work

---

## Recommended Implementation Order

### Phase 1

1. Owner-only seek
2. UI redesign batch 1
3. Confirm a clean Windows-local debug workflow

### Phase 2

1. Navidrome provider phase 1
2. UI redesign batch 2
3. Validate stream relay behavior with Navidrome sources

### Phase 3

1. UI redesign batch 3
2. Navidrome enhancements
3. Reevaluate whether Jellyfin compatibility is worth adding

---

## Seek Implementation Plan

### Backend changes

Likely files:

- `src/main/java/org/thornex/musicparty/controller/MusicSocketController.java`
- `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`
- add new DTO, likely:
  - `src/main/java/org/thornex/musicparty/dto/SeekRequest.java`

Planned work:

1. Add new socket mapping, likely `/control/seek`.
2. Add `SeekRequest(positionMs)` DTO.
3. Add `seekTo(long positionMs, String sessionId)` in `MusicPlayerService`.
4. Validate current song exists.
5. Validate caller is either:
   - current enqueuer token
   - `SYSTEM`
6. Clamp seek target into valid song duration bounds.
7. Update:
   - `positionAnchor`
   - `timestampAnchor`
8. Broadcast full player state.
9. Consider whether seek should be globally rate-limited similarly to pause/skip.

### Frontend changes

Likely files:

- `music-party-web/src/components/PlayerControl.vue`
- `music-party-web/src/stores/player.js`
- `music-party-web/src/constants/api.js`
- `music-party-web/src/composables/useAudio.js`

Planned work:

1. Add new WS destination constant for seek.
2. Add store action `seek(positionMs)`.
3. Add computed permission check:
   - current user token equals `nowPlaying.enqueuedById`
4. Convert progress display into pointer/touch draggable behavior.
5. During drag:
   - pause auto correction
   - preview dragged time locally
6. On release:
   - send one seek command
7. Handle rejection gracefully.

### Seek-specific open decisions

- Whether seek should be blocked while buffering/loading.
- Whether seek should be locally debounced if the user drags repeatedly.
- Whether mobile seek should be identical in phase 1 or desktop-first.

Recommended answers:

- Block while no current track.
- Send only on release.
- Support both mouse and touch in phase 1.

---

## UI Redesign Plan

### Primary objective

Preserve the existing workflow and information architecture while upgrading visual quality.

### Do not change in phase 1

- Room flow
- queue behavior
- search interaction model
- chat model
- player information density

### Design-system tasks

1. Define new semantic color tokens.
2. Define spacing and radius system.
3. Define surface hierarchy:
   - app background
   - sidebar surface
   - primary stage
   - player surface
   - overlays
4. Standardize buttons, inputs, modals, sheets, list items.
5. Simplify visual ornamentation.
6. Introduce controlled motion.

### Candidate component work

Batch 1 details:

- Header surface cleanup
- Sidebar and queue surface unification
- Search modal polish
- Bottom player redesign

Batch 2 details:

- queue row states
- hover patterns
- user chips
- chat panel typography
- toast styling

Batch 3 details:

- central playback visual treatment
- lite mode refinement
- tutorial layer restyling or simplification

### Reference collection task for next agent

The next agent should gather a small curated reference set, not a large moodboard.

Suggested target:

- 2 Spotify desktop references
- 2 Apple Music references
- 1 Vue music-app reference implementation if a useful one exists
- 1 component-level reference for playback controls

Then extract only:

- surface hierarchy
- spacing rhythm
- player layout rules
- list density rules
- modal styling rules

---

## Navidrome Integration Plan

### Probable implementation shape

Backend:

- add `NavidromeMusicApiService` implementing `IMusicApiService`
- add Navidrome configuration block to app properties
- authenticate against Navidrome-compatible API as needed
- adapt search results to existing `Music` / `PlayableMusic` DTOs

Frontend:

- add platform selector option
- reuse existing search modal and queue flow
- optionally add a small platform badge or icon

### Backend tasks

1. Extend config to support:
   - base URL
   - username / password or token approach
2. Implement provider:
   - `getPlatformName() -> "navidrome"`
   - `searchMusic`
   - `getPlayableMusic`
   - possibly minimal `getLyric`
3. Map Navidrome track IDs to local `Music` DTO shape.
4. Ensure stream URLs are compatible with:
   - browser playback
   - FFmpeg relay path
5. Test playback under current sync system.

### Frontend tasks

1. Add platform option in search UI.
2. Handle returned cover art and metadata cleanly.
3. Avoid special-case UX unless necessary.

### Important validation items

The next agent should specifically verify:

1. Authentication model for server-to-server Navidrome calls.
2. How stream URLs are generated and whether tokenized access is required.
3. Whether FFmpeg can read Navidrome streams without additional headers.
4. Whether duration metadata from Navidrome is reliable enough for seek and end-of-track handling.

---

## Windows Local Setup Plan

### Recommended dev stack

Install on Windows directly:

- JDK 21
- Node.js 18 or newer
- FFmpeg
- optional: Maven if not using wrapper

### Recommended run pattern

Backend:

- run with `mvn spring-boot:run` or `mvnw.cmd spring-boot:run`

Frontend:

- run with `npm run dev` inside `music-party-web`

Expected local URLs:

- frontend Vite dev server: default Vite port
- backend: `http://localhost:8080`

### Local service strategy

Short term:

- keep current Netease path working

Long term:

- add Navidrome locally for deterministic media testing

### Windows-local debugging priorities

1. Seek synchronization
2. UI work
3. local playback sanity
4. stream relay only after FFmpeg is in place

---

## Feasibility Assessment

### Owner-only seek

- Feasibility: high
- Good fit for iterative implementation

### UI redesign while preserving UX

- Feasibility: high
- Good fit for iterative implementation
- Requires design discipline more than architecture change

### Navidrome integration

- Feasibility: medium to high
- Stronger path than arbitrary direct-link playback
- Good fit for provider-based extension of the backend

### Windows-local dev without WSL2

- Feasibility: high
- This should be the default debugging path, not a fallback

---

## Suggested Immediate Next Steps for the Next Agent

1. Implement seek end to end using the agreed owner-only authorization model.
2. Validate local Windows run instructions against the actual machine.
3. Prepare a UI redesign spec for batch 1 before changing code.
4. Investigate Navidrome API details and draft a provider contract plan before coding.

---

## Notes

- The current skills visible in this session did not include a clearly available web-design-specific skill entry at the time of planning, so no skill-specific workflow was assumed in this document.
- If a later session exposes additional design skills or UI reference workflows, they can be applied on top of this plan.
