# MusicParty Handoff for Collaborators

Project root:

`D:\Pluvllter-MusicParty\MusicParty-master`

Repository state:

- Local work has been moved into a dedicated git branch named `NRT-Base`.
- Remote `origin` points to the user's fork: `https://github.com/Nirotiy/MusicParty.git`.
- Remote `upstream` points to the original project: `https://github.com/PluvIIter/MusicParty.git`.
- GitHub CLI is available and authenticated for the user's account.

This document is the single reference for the next collaborator and their agent. It merges the planning docs, the UI/session handoff notes, and the current implementation state.

## What Is Already Implemented

### UI and playback polish already landed

- Search results now show the correct cover art.
- Search layout overflow issues were hardened.
- Theme-aware toast handling was added.
- Main stage background was split for dark and light modes.
- Lyrics were moved out of the old inline block into `AppleLyricsPanel.vue`.
- Backend cover-color extraction was added.
- Frontend accent color syncing now uses the extracted cover color.

### Lyrics panel direction

The current lyrics direction is custom, not AMLL.

Reason:

- The project already has raw LRC data, parsed lines, and time sync.
- A custom Vue component keeps full control over style, motion, and integration.
- AMLL remains a future spike only if the custom approach stops reaching the target polish.

Current lyrics component:

- `music-party-web/src/components/AppleLyricsPanel.vue`

Relevant integration point:

- `music-party-web/src/components/CenterConsole.vue`

### Cover color extraction

The current dominant-cover-color path is wired end to end:

- backend color extraction service
- API exposure
- frontend accent consumption

Relevant file:

- `src/main/java/org/thornex/musicparty/service/CoverColorService.java`

## Current Product Direction

The project is now moving toward a cleaner music-player layout instead of the earlier control-console look.

The active design direction is:

- Apple Music / Spotify style composition
- less chrome
- more surface hierarchy
- theme-aware accents
- cleaner lyrics presentation

The original handoff docs still matter, but the implementation priority is now:

1. Keep the current UI stable.
2. Preserve the lyric panel behavior.
3. Continue propagating cover-derived accent color to more surfaces if needed.
4. Avoid reverting into the old industrial/HUD styling.

## Important Existing Documents

Read these first:

- `AGENT_HANDOFF_PLAN.md`
- `PROJECT_ROADMAP_DETAILED.md`
- `SESSION_HANDOFF_2026-05-11_DEV_SCRIPT_NETEASE.md`
- `SESSION_HANDOFF_2026-05-11_MAIN_UI.md`
- `UI_REIMPLEMENT.md`

## Current Code Areas Worth Knowing

Frontend:

- `music-party-web/src/components/AppleLyricsPanel.vue`
- `music-party-web/src/components/CenterConsole.vue`
- `music-party-web/src/components/PlayerControl.vue`
- `music-party-web/src/components/SearchModal.vue`
- `music-party-web/src/components/layout/MainLayout.vue`
- `music-party-web/src/stores/player.js`
- `music-party-web/src/stores/ui.js`
- `music-party-web/src/api/music.js`
- `music-party-web/src/composables/useAudio.js`
- `music-party-web/src/utils/parser.js`

Backend:

- `src/main/java/org/thornex/musicparty/controller/ApiController.java`
- `src/main/java/org/thornex/musicparty/service/CoverColorService.java`
- `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`
- `src/main/java/org/thornex/musicparty/service/api/NeteaseMusicApiService.java`
- `src/main/java/org/thornex/musicparty/service/api/BilibiliMusicApiService.java`
- `src/main/java/org/thornex/musicparty/service/stream/LiveStreamService.java`

## What The Next Collaborator Should Focus On

### 1. Keep lyrics quality moving forward

The lyric component should remain:

- theme-aware
- line-based
- centered around the active line
- compatible with current playback timing
- free of the old card-box look

If more visual polish is needed, improve the existing component instead of replacing the whole approach.

### 2. Expand cover-driven accent usage carefully

The cover color extraction already exists. The next useful step is to decide where the accent should apply globally.

Good candidates:

- player controls
- lyric highlight tinting
- selected tab states
- subtle surface accents

Avoid:

- flooding the entire page with the same accent
- making the page visually noisy

### 3. Preserve the current theme logic

Dark/light mode behavior has already been tuned and should not be regressed.

### 4. Keep the branch clean

Work should continue on `NRT-Base`.

Do not:

- reset or rewrite unrelated user changes
- recreate the repository from scratch
- reintroduce ignored build artifacts into git

## Git / Repo Notes

The repository was not originally cloned with git. It was copied locally and then attached to git afterward.

That means:

- the branch history starts from a fresh baseline commit
- the first commit is an import of the current working state
- the existing remote fork is the source of truth for future collaboration

Current remote layout:

- `origin` = `Nirotiy/MusicParty`
- `upstream` = `PluvIIter/MusicParty`

## Recommended Workflow For The Next Agent

1. Pull the current branch state from `origin/NRT-Base`.
2. Read the planning docs and this handoff file.
3. Make only focused changes.
4. Verify with build/run checks before touching adjacent systems.
5. If a UI or lyrics change is needed, keep the existing interaction contract intact.

## Status Snapshot

- Search cover fix: done
- Theme sync: done
- Dominant cover color extraction: done
- Lyrics panel refactor: done
- Git fork wiring: done
- Base branch for collaboration: `NRT-Base`

## Notes For The Agent

- The user values practical, incremental changes.
- The user has already confirmed that the current direction is acceptable.
- If a tradeoff is required, prefer preserving stability over making the UI more ambitious in one pass.
