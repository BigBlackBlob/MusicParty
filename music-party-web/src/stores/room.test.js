import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useRoomStore } from './room';
import { roomApi } from '../api/rooms';

vi.mock('../api/rooms', () => ({
  roomApi: {
    list: vi.fn(),
    verify: vi.fn(),
    update: vi.fn(),
    remove: vi.fn()
  }
}));

vi.mock('../services/socket', () => ({
  socketService: {
    send: vi.fn()
  }
}));

describe('room access tokens', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it('does not restore room access credentials from local storage', () => {
    localStorage.setItem('mp_room_access_tokens', JSON.stringify({ private: 'legacy-token' }));
    const roomStore = useRoomStore();

    expect(roomStore.getRoomAccessToken('private')).toBe('');
  });

  it('keeps only an in-memory room access marker', () => {
    const roomStore = useRoomStore();
    roomStore.setRoomAccessToken('private', 'granted');

    expect(roomStore.getRoomAccessToken('private')).toBe('granted');
    expect(localStorage.getItem('mp_room_access_tokens')).toBeNull();
  });

  it('clears a room access marker explicitly', () => {
    const roomStore = useRoomStore();
    roomStore.setRoomAccessToken('private', 'granted');
    roomStore.clearRoomAccessToken('private');

    expect(roomStore.getRoomAccessToken('private')).toBe('');
  });

  it('treats public rooms as accessible and private rooms as token-gated', () => {
    const roomStore = useRoomStore();
    roomStore.setRooms([
      { roomId: 'lounge', name: 'Lounge' },
      { roomId: 'private', name: 'Private', privateRoom: true }
    ]);

    expect(roomStore.hasValidRoomAccess('lounge')).toBe(true);
    expect(roomStore.hasValidRoomAccess('private')).toBe(false);

    roomStore.setRoomAccessToken('private', 'fresh-token', Date.now() + 60_000);

    expect(roomStore.hasValidRoomAccess('private')).toBe(true);
  });

  it('verifies room access without receiving a browser-readable token', async () => {
    const roomStore = useRoomStore();
    roomApi.verify.mockResolvedValue({
      valid: true,
      expiresAt: 123456
    });

    await roomStore.verifyRoomAccess('private', 'letmein');

    expect(roomApi.verify).toHaveBeenCalledWith('private', 'letmein');
    expect(roomStore.roomAccessTokens.private).toBe(true);
  });
});
