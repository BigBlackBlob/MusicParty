# Mobile Frontend Agent Guardrails

This document is mandatory context for agents touching `music-party-web/src/components/mobile` or shared playback/lyrics UI.

## Non-Negotiables

- Do not rebuild the mobile shell from scratch. Preserve the existing structure: `MobileLayout` owns the shell, `MobileBottomNav` owns tab switching, and each tab page owns only its local content.
- Do not add page-wide scrims, dark overlays, glass layers, or backdrop blur to the mobile root. Ambient cover art must stay behind content, low opacity, and non-interactive.
- Do not hide or move playback controls below the viewport. `MobileNowPlaying` must always leave room for progress, transport controls, and bottom navigation on 390x844 and 390x667 viewports.
- Do not fork playback state logic inside mobile components. Use `useNowPlayingViewModel` for current track, requester, platform, seek permission, like state, and accent updates.
- Do not fork chat filtering or message grouping. Use `useChatViewModel` for chat/system filtering, time separators, read state, and sending.

## Lyrics Rules

- `AppleLyricsPanel` is shared by desktop and mobile. Any mobile-specific sizing must use the `mobile` prop or `.lyrics-shell--mobile`, never global desktop rules.
- Mobile lyrics must support all four controls: alignment toggle, smaller font, larger font, and translation toggle. These controls must remain visible inside the overlay and respect safe-area bottom.
- Primary lyric and translation are a two-line pair. Each part may wrap to two visual lines, but must not overflow horizontally.
- Font scaling must be bounded. Mobile active lyrics should stay within the panel on narrow phones; avoid desktop-sized `vw` clamps in mobile overlays.
- Mini lyrics in `MobileNowPlaying` are only a preview/open affordance. It must not grow enough to push playback controls off-screen.

## Layout Rules

- Keep mobile surfaces practical: solid or lightly translucent panels are acceptable; decorative glassmorphism is not.
- Header and bottom nav should use stable heights and normal borders. Avoid absolute overlays unless they are modal content.
- `MobileLayout .mobile-ambient` must be `pointer-events: none`, stay behind content, and use opacity low enough not to read as a global mask.
- Page content should scroll inside its tab area. The app root must not become document-scrollable.
- Any bottom action bar must reserve list padding so final rows are reachable.

## Verification Required

Before handing off mobile UI changes:

- Run `npm run build` in `music-party-web`.
- Check mobile preview at `http://127.0.0.1:5173/?mobilePreview=1` with at least 390x844.
- Verify these screens: Now Playing, full lyrics overlay, Queue, Search, Chat.
- In Now Playing, confirm the album art, mini lyrics, progress scrubber, transport controls, and bottom nav are all visible.
- In full lyrics, confirm alignment, A-/A+, and translation controls are visible and lyrics do not overflow horizontally.
