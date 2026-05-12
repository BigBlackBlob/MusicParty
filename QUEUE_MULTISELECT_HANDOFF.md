# Queue Multiselect and Desktop Favorites Handoff

Date: 2026-05-12

Branch: `NRT-Base`

## Summary

This handoff covers the latest queue and favorites work after `HANDOFF_TO_201.md`.

Implemented areas:

- desktop explicit favorite button and liked-songs panel
- desktop liked-songs `.txt` export
- queue multiselect on desktop and mobile
- backend batch queue top/remove WebSocket endpoints
- mobile floating multiselect action rail
- mobile queue card density adjustments
- final consistency fix for bulk delete queue updates

## New Backend Queue Batch API

New DTO:

- `src/main/java/org/thornex/musicparty/dto/QueueBatchActionRequest.java`

New WebSocket destinations:

- `/app/queue/batch-top`
- `/app/queue/batch-remove`

Controller:

- `src/main/java/org/thornex/musicparty/controller/MusicSocketController.java`

Service:

- `src/main/java/org/thornex/musicparty/service/MusicPlayerService.java`

Queue manager:

- `src/main/java/org/thornex/musicparty/service/MusicQueueManager.java`

Important behavior:

- Batch top uses global `TOP-` priority even when shuffle is enabled.
- Batch top preserves current queue order.
- Batch remove removes all matching queue IDs in one backend operation.
- Batch remove now calls both:
  - `broadcastQueueUpdate()`
  - `broadcastFullPlayerState()`

Reason for the extra full state broadcast:

- Looping single-item removes caused many queue broadcasts.
- The frontend could apply an older intermediate queue snapshot last.
- This produced UI residue where some items appeared undeleted until a later resync or track change.
- Batch remove plus full state broadcast plus frontend resync is the current consistency fix.

## Frontend Store Changes

File:

- `music-party-web/src/stores/player.js`

Added/updated actions:

- `topSongs(queueIds)`
- `removeSongs(queueIds)`
- `topSongsCompat(queueIds)`
- `removeSongsCompat(queueIds)`

Current important detail:

- `removeSongs(queueIds)` sends `/app/queue/batch-remove`, then schedules `RESYNC` after 250ms.
- Do not change it back to looping single-item removes unless deliberately accepting queue snapshot races.

WebSocket constants:

- `music-party-web/src/constants/api.js`
  - `QUEUE_BATCH_TOP`
  - `QUEUE_BATCH_REMOVE`

## Shared Selection Logic

New composable:

- `music-party-web/src/composables/useQueueSelection.js`

Used by:

- `music-party-web/src/components/QueueList.vue`
- `music-party-web/src/components/mobile/MobileQueueView.vue`

Provides:

- `selectionMode`
- `selectedCount`
- `selectedIds`
- `hasSelection`
- `enterSelectionMode(queueId)`
- `exitSelectionMode()`
- `toggleSelected(queueId)`
- `isSelected(queueId)`
- `selectAll()`
- stale selection cleanup when queue changes

## Desktop Queue and Favorites

Files:

- `music-party-web/src/components/PlayerControl.vue`
- `music-party-web/src/components/QueueList.vue`
- `music-party-web/src/components/QueueItem.vue`

Desktop favorite work:

- `PlayerControl.vue` has an explicit heart button.
- It reuses `player.sendLike()`.
- It writes into shared `player.likedSongs`.
- `QueueList.vue` has `队列 / 喜欢` tabs.
- Liked songs can be removed individually and exported as `.txt`.
- Export formatting comes from `music-party-web/src/utils/likedSongs.js`.

Desktop multiselect:

- Queue tab has a `选择` button.
- Selection mode supports:
  - row click toggle
  - select all
  - batch top
  - batch delete
  - cancel
- Batch delete has inline confirmation.
- `QueueItem.vue` now accepts:
  - `selectionMode`
  - `selected`
  - emits `toggle-select`

## Mobile Queue Multiselect

File:

- `music-party-web/src/components/mobile/MobileQueueView.vue`

Entry behavior:

- Long press a queue item to enter selection mode.
- Tap rows to toggle selection after selection mode is active.
- Guest users cannot enter selection mode.

Action UI:

- Multiselect actions are now a floating right-side vertical rail over the list.
- The rail does not reserve or squeeze list width.
- It appears lower than center (`top: 64%`) for easier one-handed reach.
- Buttons:
  - select all
  - top selected
  - delete selected
  - cancel

Delete confirmation:

- Floating bottom confirmation card.
- Does not consume list height.

Density changes:

- Queue and liked-song cards are more compact.
- Current item row baseline:
  - `min-h-[52px]`
  - 36px cover
  - tighter text line-height
  - narrow fixed grid columns
- Right-side single-item action buttons are compact at about 35px.

Note:

- The compact action buttons are below the ideal 44px touch target. This was an intentional density tradeoff requested during mobile queue tuning.

## Validation Run

After the final queue consistency fix:

```powershell
cd music-party-web
npm run build
```

Passed.

```powershell
$env:JAVA_HOME='C:\Program Files\Microsoft\jdk-21.0.11.10-hotspot'
$env:PATH="$env:JAVA_HOME\bin;$env:PATH"
.\mvnw.cmd -DskipTests package
```

Passed.

`git diff --check` passed for the touched files, with only Windows line-ending warnings.

## Known Local Non-Code State

At handoff time, these local items may still exist and should not be blindly committed:

- `data/queue-data.json`
- `cached_media/`
- `packaging/`

`data/queue-data.json` is runtime queue state.

## Suggested Follow-up QA

Desktop:

1. Add at least 10 queue items.
2. Enter `选择` mode.
3. Select several non-adjacent rows.
4. Batch top, confirm order moves to top.
5. Select many rows and batch delete.
6. Verify the rows disappear without waiting for a track change.
7. Check liked tab export still works.

Mobile:

1. Open `?mobilePreview=1`.
2. Long press a queue item.
3. Confirm floating rail appears over the list, lower than center.
4. Select several items.
5. Batch delete and confirm.
6. Verify deleted rows disappear immediately and do not reappear.
7. Confirm liked tab is unaffected.

## Caution for Next Agent

Do not reintroduce looped single-item deletion for `removeSongs`.

If batch delete appears flaky again, inspect:

- whether the running backend includes `/app/queue/batch-remove`
- STOMP error frames in browser console
- queue update event ordering
- whether `RESYNC` returns a final `PlayerState.queue`

The intended stable path is:

1. frontend sends `/app/queue/batch-remove`
2. backend removes all selected IDs in one synchronized operation
3. backend broadcasts queue update once
4. backend broadcasts full player state once
5. frontend sends delayed `RESYNC` as a fallback
