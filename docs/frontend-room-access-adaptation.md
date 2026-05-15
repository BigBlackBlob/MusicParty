# Frontend Room Access Adaptation Notes

## Scope

This note captures frontend UI/UX work still needed after backend support for:

- private/public room switching
- room password rotation
- room access token invalidation via `password_version`
- room settings updates by owner/admin

The current codebase now has the backend and minimal frontend logic hooks, but the full product UX is not yet complete.

## Already Added

- Room-level `verify` API usage is available in frontend logic.
- Room access tokens can be cached per room in local storage.
- WebSocket connect headers now support `room-access-token`.
- Room metadata now exposes `privateRoom`.

## Required UI/UX Work

### 1. Room List / Room Switcher

Current room list should visually distinguish:

- public rooms
- private rooms
- rooms owned by current user

Recommended UI changes:

- add a lock icon for private rooms
- add an owner badge or subtle “mine” indicator
- show active room and protected room state without relying on text only

### 2. Private Room Join Flow

Current frontend still lacks a dedicated join-password UX.

Needed behavior:

- when user clicks a private room without a valid cached token, open a password prompt
- on success, cache `roomAccessToken` for that room and reconnect
- on failure, keep user in current room and show a clear error
- if a cached token becomes invalid after password rotation, prompt again instead of silent reconnect loops

Recommended UX:

- modal or sheet with room name, password field, submit/cancel
- explicit invalid-password message
- loading state while verifying

### 3. Create Room Flow

Current create-room UI still only supports room name.

Needed additions:

- public/private toggle
- password field shown only for private rooms
- validation for empty private-room password

Recommended UX:

- replace single inline input with a small create-room form or modal
- keep default path optimized for public room creation

### 4. Room Settings UI

There is currently no owner-facing room settings UI.

Needed owner/admin controls:

- rename room
- switch public/private
- set new password
- keep existing password when staying private but changing only name

Recommended UX:

- settings entry on owned room row or inside room menu
- modal with:
  - room name
  - visibility toggle
  - optional password field
  - “keep existing password” checkbox when editing a private room

### 5. Token Invalidation UX

`password_version` now invalidates old room access tokens. Frontend must handle this explicitly.

Needed behavior:

- if WebSocket connect fails because `room-access-token` is stale, clear cached token for that room
- prompt for password again
- avoid repeated auto-reconnect with the same invalid token

Recommended handling points:

- STOMP error callback
- room switch flow
- app reload reconnect flow

### 6. Room Deletion / Fallback UX

When a room is deleted or loses access:

- current room should fall back to `lounge`
- user should receive a clear notice
- stale room token should be cleared

### 7. Legacy Global Auth Overlay Cleanup

Frontend still contains the old global room-password/auth overlay path.

This needs review because the product model is now room-scoped access, not single global-room protection.

Needed decision:

- either retire the old global auth overlay entirely
- or narrow it to admin/setup use only

## Suggested Implementation Order

1. Private room join modal
2. Create room public/private form
3. Room settings modal for owner/admin
4. STOMP invalid-token recovery path
5. Cleanup old global auth overlay model

## Risk Notes

- Do not keep sending stale `room-access-token` forever on reconnect.
- Do not silently drop user into `lounge` when private room access fails unless the UI clearly explains why.
- Avoid overloading the compact room dropdown with too many inline controls; a modal is likely cleaner for settings.
