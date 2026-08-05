import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useRoomStore } from './room';
import { queryClient } from '../app/providers';
import { roomApi } from '../api/rooms';

vi.mock('../api/rooms', () => ({ roomApi: { list: vi.fn(), verify: vi.fn(), update: vi.fn(), remove: vi.fn() } }));
vi.mock('../transport/realtimeClient', () => ({ realtimeClient: { send: vi.fn() } }));

describe('room access state', () => {
  beforeEach(() => { localStorage.clear(); queryClient.clear(); setActivePinia(createPinia()); vi.clearAllMocks(); });

  it('uses server-provided access state without storing credentials', () => {
    const roomStore = useRoomStore();
    roomStore.setRooms([{ roomId: 'lounge', name: 'Lounge' }, { roomId: 'private', name: 'Private', privateRoom: true, accessGranted: false }]);

    expect(roomStore.hasValidRoomAccess('lounge')).toBe(true);
    expect(roomStore.hasValidRoomAccess('private')).toBe(false);
    expect(roomStore.roomAccessTokens).toBeUndefined();
    expect(localStorage.getItem('mp_room_access_tokens')).toBeNull();

    roomStore.setRoomAccessGranted('private', true);
    expect(roomStore.hasValidRoomAccess('private')).toBe(true);
  });

  it('updates only the non-sensitive access state after verification', async () => {
    const roomStore = useRoomStore();
    roomStore.setRooms([{ roomId: 'private', name: 'Private', privateRoom: true, accessGranted: false }]);
    roomApi.verify.mockResolvedValue({ roomId: 'private', accessGranted: true, expiresAt: 123456 });

    await roomStore.verifyRoomAccess('private', 'letmein');

    expect(roomApi.verify).toHaveBeenCalledWith('private', 'letmein');
    expect(roomStore.currentRoom.accessGranted).toBe(true);
  });
});
