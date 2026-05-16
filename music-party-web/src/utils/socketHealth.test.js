import { describe, expect, it } from 'vitest';
import { shouldForceSocketReconnect } from './socketHealth';

describe('socket health helpers', () => {
  it('forces reconnect when disconnected or sync heartbeat is stale', () => {
    expect(shouldForceSocketReconnect({ connected: false, now: 10_000 })).toBe(true);
    expect(shouldForceSocketReconnect({ connected: true, lastPongAt: 1_000, now: 20_000 })).toBe(false);
    expect(shouldForceSocketReconnect({ connected: true, lastPongAt: 1_000, now: 40_000 })).toBe(true);
  });
});
