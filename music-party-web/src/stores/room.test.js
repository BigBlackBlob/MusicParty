import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useRoomStore } from './room';
import { useUserStore } from './user';
import { STORAGE_KEYS } from '../constants/keys';
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

  it('reads legacy string room access tokens', () => {
    localStorage.setItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS, JSON.stringify({
      private: 'legacy-token'
    }));
    const roomStore = useRoomStore();

    expect(roomStore.getRoomAccessToken('private')).toBe('legacy-token');
  });

  it('returns a non-expired room access token', () => {
    localStorage.setItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS, JSON.stringify({
      private: { token: 'fresh-token', expiresAt: Date.now() + 60_000 }
    }));
    const roomStore = useRoomStore();

    expect(roomStore.getRoomAccessToken('private')).toBe('fresh-token');
  });

  it('clears and hides expired room access tokens', () => {
    localStorage.setItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS, JSON.stringify({
      private: { token: 'expired-token', expiresAt: Date.now() - 1 }
    }));
    const roomStore = useRoomStore();

    expect(roomStore.getRoomAccessToken('private')).toBe('');
    expect(JSON.parse(localStorage.getItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS))).toEqual({});
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

  it('verifies room access and stores the returned token with expiry', async () => {
    const userStore = useUserStore();
    userStore.initUser('session-token', 'u1', 'Alice', false);
    const roomStore = useRoomStore();
    roomApi.verify.mockResolvedValue({
      roomAccessToken: 'verified-token',
      expiresAt: 123456
    });

    await roomStore.verifyRoomAccess('private', 'letmein');

    expect(roomApi.verify).toHaveBeenCalledWith('private', 'letmein', 'session-token');
    expect(roomStore.roomAccessTokens.private).toEqual({
      token: 'verified-token',
      expiresAt: 123456
    });
  });
});
