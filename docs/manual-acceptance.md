# Go-only manual acceptance

Use a disposable local database or an approved non-production environment. Never place test passwords or platform credentials in this document or the repository.

## Identity and access

- Administrator login succeeds and account/security state updates immediately after login and logout.
- A room owner or platform administrator can create an invite; its link can be copied once without exposing old invite secrets.
- A new visitor can enter as a guest when that entry is enabled, or redeem a valid invite through `/join/<secret>`.
- Invalid, expired and revoked invitations show distinct errors without creating a session.

## Rooms and presence

- Public-room switching commits only after the authoritative room snapshot arrives.
- A private room rejects a wrong password and accepts the correct password without losing the previous connection on failure.
- Joining, leaving, reconnecting and renaming update the member list and online count consistently in two browser contexts.
- A visibility restore or temporary network interruption reconnects and resynchronizes without duplicating presence.

## Playback and chat

- Enqueue success receives an ACK; rejection or timeout rolls back optimistic queue state.
- Queue reorder is consistent in two clients and a version gap triggers resync.
- Room and public chat remain isolated and unread state follows the selected channel.
- Netease search, playback and lyrics work for a known track with lyrics.

## Administration

- Platform credentials can be updated without values appearing in logs or responses.
- Netease, Bilibili, each enabled Subsonic/Navidrome source and YouTube (when enabled) pass their connection checks.
- Invite creation/revocation and room-member management follow the current capability rules.

## Layouts

- Desktop free layout still drags, saves and restores modules.
- Mobile Now Playing, Queue and primary navigation work at 390×844.
- Lite Mode remains usable and exposes no management-only navigation.
- Visual baselines are reviewed manually; do not update snapshots automatically.
