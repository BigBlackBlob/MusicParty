import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { ROOM_SWITCH_PASSWORD_REQUIRED, usePlayerStore } from './player';
import { useUserStore } from './user';
import { useRoomStore } from './room';
import { useToastStore } from './toast';
import { socketService } from '../services/socket';
import { roomApi } from '../api/rooms';
import { WS_DEST } from '../constants/api';

vi.mock('../services/socket', () => ({
  socketService: {
    connected: false,
    send: vi.fn(),
    connect: vi.fn(),
    disconnect: vi.fn(),
    reconnectNow: vi.fn()
  }
}));

vi.mock('../api/rooms', () => ({
  roomApi: {
    list: vi.fn(),
    verify: vi.fn(),
    update: vi.fn(),
    remove: vi.fn()
  }
}));

describe('player controls', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
    socketService.connected = false;
    socketService.send.mockReturnValue(true);
    roomApi.verify.mockResolvedValue({ valid: true, expiresAt: Date.now() + 60_000 });
    vi.useRealTimers();
  });

  it('opens name prompt and does not send controls for guests', () => {
    const user = useUserStore();
    const player = usePlayerStore();

    const sent = player.togglePause();

    expect(sent).toBe(false);
    expect(user.showNameModal).toBe(true);
    expect(socketService.send).not.toHaveBeenCalled();
  });

  it('shows a toast when controls are used before socket connection', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('token', 'u1', 'Alice', false);
    const player = usePlayerStore();
    const toast = useToastStore();

    const sent = player.togglePause();

    expect(sent).toBe(false);
    expect(toast.toasts.at(-1).message).toContain('播放服务尚未连接');
    expect(socketService.send).not.toHaveBeenCalled();
  });

  it('blocks rapid repeated controls locally', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('token', 'u1', 'Alice', false);
    const player = usePlayerStore();
    player.connected = true;
    socketService.connected = true;

    expect(player.togglePause()).toBe(true);
    expect(player.playNext()).toBe(false);
    expect(socketService.send).toHaveBeenCalledTimes(1);
  });

  it('sends controls through the socket when connected and allowed', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('token', 'u1', 'Alice', false);
    const player = usePlayerStore();
    player.connected = true;
    socketService.connected = true;

    const sent = player.toggleShuffle();

    expect(sent).toBe(true);
    expect(socketService.send).toHaveBeenCalledWith(WS_DEST.PLAYER_SHUFFLE, {});
  });

  it('optimistically reorders queue without immediate forced resync', () => {
    vi.useFakeTimers();
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('token', 'u1', 'Alice', false);
    const player = usePlayerStore();
    player.queue = [{ queueId: 'a' }, { queueId: 'b' }, { queueId: 'c' }];

    const sent = player.reorderQueue(0, 2, 'a', 'c', 'after');

    expect(sent).toBe(true);
    expect(player.queue.map(item => item.queueId)).toEqual(['b', 'c', 'a']);
    expect(socketService.send).toHaveBeenCalledWith(WS_DEST.QUEUE_REORDER, expect.objectContaining({
      oldIndex: 0,
      newIndex: 2,
      queueId: 'a',
      targetQueueId: 'c',
      position: 'after',
      mutationId: expect.any(String)
    }));
    expect(socketService.send).not.toHaveBeenCalledWith(WS_DEST.RESYNC, expect.anything());

    vi.advanceTimersByTime(1499);
    expect(socketService.send).not.toHaveBeenCalledWith(WS_DEST.RESYNC, expect.anything());
    vi.advanceTimersByTime(1);
    expect(socketService.send).toHaveBeenCalledWith(WS_DEST.RESYNC, { reason: 'queue-reorder-timeout' });
  });

  it('defers queue broadcasts until a pending reorder is acknowledged', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('token', 'u1', 'Alice', false);
    const player = usePlayerStore();
    player.queue = [{ queueId: 'a' }, { queueId: 'b' }, { queueId: 'c' }];

    player.reorderQueue(0, 2, 'a', 'c', 'after');
    const payload = socketService.send.mock.calls.find(([type]) => type === WS_DEST.QUEUE_REORDER)[1];
    player.setQueue([{ queueId: 'a' }, { queueId: 'b' }, { queueId: 'c' }], 2);

    expect(player.queue.map(item => item.queueId)).toEqual(['b', 'c', 'a']);
    player.settleQueueReorder(payload.mutationId, true);
    expect(player.queue.map(item => item.queueId)).toEqual(['a', 'b', 'c']);
  });

  it('switches public rooms without room access verification', async () => {
    const roomStore = useRoomStore();
    const player = usePlayerStore();
    roomStore.setRooms([
      { roomId: 'lounge', name: 'Lounge' },
      { roomId: 'public', name: 'Public' }
    ]);

    await player.switchRoom('public');

    expect(roomApi.verify).not.toHaveBeenCalled();
    expect(roomStore.currentRoomId).toBe('public');
    expect(socketService.disconnect).toHaveBeenCalled();
  });

  it('requires a password before switching to a private room without access', async () => {
    const roomStore = useRoomStore();
    const player = usePlayerStore();
    roomStore.setRooms([
      { roomId: 'lounge', name: 'Lounge' },
      { roomId: 'private', name: 'Private', privateRoom: true }
    ]);

    await expect(player.switchRoom('private')).rejects.toMatchObject({
      code: ROOM_SWITCH_PASSWORD_REQUIRED,
      roomId: 'private'
    });

    expect(roomApi.verify).not.toHaveBeenCalled();
    expect(roomStore.currentRoomId).toBe('lounge');
    expect(socketService.disconnect).not.toHaveBeenCalled();
  });

  it('verifies private room passwords before switching and reconnecting', async () => {
    const user = useUserStore();
    user.initUser('session-token', 'u1', 'Alice', false);
    const roomStore = useRoomStore();
    const player = usePlayerStore();
    roomStore.setRooms([
      { roomId: 'lounge', name: 'Lounge' },
      { roomId: 'private', name: 'Private', privateRoom: true }
    ]);

    await player.switchRoom('private', 'letmein');

    expect(roomApi.verify).toHaveBeenCalledWith('private', 'letmein');
    expect(roomStore.getRoomAccessToken('private')).toBe('granted');
    expect(roomStore.currentRoomId).toBe('private');
    expect(socketService.disconnect).toHaveBeenCalled();
  });

  it('uses cached private room access without verifying again', async () => {
    const roomStore = useRoomStore();
    const player = usePlayerStore();
    roomStore.setRooms([
      { roomId: 'lounge', name: 'Lounge' },
      { roomId: 'private', name: 'Private', privateRoom: true }
    ]);
    roomStore.setRoomAccessToken('private', 'cached-token', Date.now() + 60_000);

    await player.switchRoom('private');

    expect(roomApi.verify).not.toHaveBeenCalled();
    expect(roomStore.currentRoomId).toBe('private');
    expect(socketService.disconnect).toHaveBeenCalled();
  });
});
