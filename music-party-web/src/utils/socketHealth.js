const DEFAULT_STALE_PONG_MS = 30_000;

export const shouldForceSocketReconnect = ({
  connected,
  lastPongAt = 0,
  now = Date.now(),
  staleAfterMs = DEFAULT_STALE_PONG_MS
}) => !connected || (lastPongAt > 0 && now - lastPongAt > staleAfterMs);
