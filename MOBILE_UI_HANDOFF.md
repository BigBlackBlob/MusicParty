# Mobile UI Handoff

## Scope Completed

This round introduced a separate mobile UI path while keeping the desktop layout intact.

### Mobile Layout Split

- Added mobile-only layout components under `music-party-web/src/components/mobile/`.
- `App.vue` now switches between desktop `MainLayout` and `MobileLayout`.
- Added `forceMobileLayout` and `mobilePreviewWidth` to `uiStore`.
- Added URL toggles:
  - `?mobilePreview=1` enables desktop mobile preview.
  - `?mobilePreview=0` disables it.
- Added `MobilePreviewShell` for desktop-side mobile testing at 375 / 390 / 430 / 768 widths.

### Mobile Views

- `MobileLayout.vue`
  - Dedicated mobile shell with top bar, member sheet, content area, and bottom navigation.
- `MobileBottomNav.vue`
  - Fixed-height bottom nav with safe-area support.
  - Tabs: playback, queue, search, chat.
- `MobileNowPlaying.vue`
  - Mobile-first now-playing screen.
  - Smaller album art than the first prototype.
  - Mobile seek drag follows desktop permission logic: only the current song enqueuer can seek.
  - Updates dynamic accent from current cover.
- `MobileQueueView.vue`
  - Touch-friendly queue list.
  - Queue actions are always tappable on mobile.
- `MobileSearchView.vue`
  - Full-screen mobile search.
  - Supports NetEase/Bilibili songs and NetEase album import.
- `MobileChatView.vue`
  - Full-screen mobile chat with bottom input.

### Mini Lyrics Widget

- Added `MobileMiniLyrics.vue`.
- Spotify-inspired single-line mini lyrics widget.
- Located between album art and song title in `MobileNowPlaying`.
- Left aligned with the song title and artist.
- Uses current playback progress to map to the active lyric line.
- Uses slide + crossfade transition for line changes.
- Hides entirely when:
  - lyrics are missing,
  - parsed valid lyric lines are fewer than 5,
  - no displayable lyric data exists.
- Reserves two-line height for future translation support so title/progress layout will not jump later.
- Clicking the mini lyrics opens a temporary full-screen lyrics view using existing `AppleLyricsPanel`.

### Earlier Stability/UI Fixes In This Batch

- Auto lite mode now defaults to off.
- Auto lite mode triggers only after 180 seconds in the background.
- Added suppression after returning to foreground to avoid repeated lite-mode toggles.
- Search modal desktop behavior no longer gets affected by mobile-only `mobileView` state.
- Dark-mode dynamic accent color now gets contrast normalization on both frontend and backend.
- Lyrics with fewer than 5 parsed timestamped lines are treated as no lyrics.
- For no-lyrics tracks on mobile, the lyrics widget is not rendered and album art stays centered.

## Validation

- `npm run build` passed after the mobile UI work.
- Backend package was previously validated with:

```powershell
$env:JAVA_HOME='C:\Program Files\Microsoft\jdk-21.0.11.10-hotspot'
$env:PATH="$env:JAVA_HOME\bin;$env:PATH"
.\mvnw.cmd -DskipTests package
```

## Current Dev Preview

Use:

```text
http://localhost:5173/?mobilePreview=1
```

If testing on a phone in the same network, use the Vite network URL and append `?mobilePreview=1`.

## Known Follow-Ups

- Mobile UI is usable but still first-pass:
  - full lyrics page needs a mobile-native design,
  - action sheets can replace some inline queue/search actions,
  - mobile chat still needs polish around system events,
  - member sheet is functional but simple,
  - mobile seek smoothing is not implemented yet.
- Audio seek smoothing, multi-device sync improvements, and web-player crossfade are tracked in `ROADMAP.md`.
- Do not mix desktop and mobile layout logic again. Keep mobile-specific UI under `components/mobile/`.
