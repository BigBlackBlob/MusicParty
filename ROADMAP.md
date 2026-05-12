# MusicParty NRT Roadmap

## Audio and Synchronization

- Add seek smoothing for the web player:
  - Fade volume down before applying `audio.currentTime`.
  - Wait for `seeked` or `canplay`.
  - Fade volume back in after the target range is playable.
  - Skip hard seeking for very small drift and let the player converge naturally.

- Improve multi-device synchronization:
  - Add ping/pong RTT measurement for each client.
  - Smooth server clock offset instead of using only the latest `serverTimestamp`.
  - Use playback-rate correction for small drift, hard seek only for large drift.
  - Trigger immediate resync after visibility restore, network restore, and reconnect.
  - Add a playback epoch/state version so stale state packets cannot override newer playback state.

- Add web-player track transition fades for NetEase and Bilibili playback:
  - Fade out near the end of the current track.
  - Fade in after the next track is ready.
  - Keep this scoped to browser playback. The HTTP live stream path should not get crossfade unless the server-side ffmpeg pipeline is redesigned.

## Mobile UI/UX

- Redesign the mobile experience separately from the current desktop-first layout:
  - Dedicated mobile now-playing screen.
  - Bottom player sized for thumb use.
  - Separate queue, chat, member, search, and lyric flows.
  - Avoid compressing desktop sidebars into mobile overlays without a mobile-specific information architecture.

## Streaming

- Revisit the HTTP stream broadcaster later:
  - Isolate each listener with its own bounded writer queue.
  - Drop slow clients instead of blocking all listeners.
  - Add better stream keepalive behavior for paused playback if target clients require it.
