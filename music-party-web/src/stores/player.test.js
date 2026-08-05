import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { ROOM_SWITCH_PASSWORD_REQUIRED, useRoomRealtimeCoordinator } from '../domains/realtime/roomRealtimeCoordinator';
import { useRoomCommandStore } from '../domains/realtime/roomCommandStore';
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore';
import { useRealtimeConnectionStore } from '../domains/realtime/realtimeConnectionStore';
import { useUserStore } from './user';
import { useRoomStore } from './room';
import { queryClient } from '../app/providers';
import { useToastStore } from './toast';
import { realtimeClient as socketService } from '../transport/realtimeClient';
import { roomApi } from '../api/rooms';
import { WS_DEST } from '../constants/api';

vi.mock('../transport/realtimeClient', () => ({
  realtimeClient: {
    connected: false,
    send: vi.fn(),
    sendEnvelope: vi.fn(),
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

const usePlayerStore = () => {
  const commands = useRoomCommandStore();
  const runtime = useRoomRuntimeStore();
  const connection = useRealtimeConnectionStore();
  const coordinator = useRoomRealtimeCoordinator();
  return {
    get queue() { return runtime.queue; },
    set queue(value) { runtime.queue = value; },
    get connected() { return connection.connected; },
    set connected(value) { connection.connected = value; },
    togglePause: commands.togglePause,
    toggleShuffle: commands.toggleShuffle,
    playNext: commands.playNext,
    enqueue: commands.enqueue,
    reorderQueue: commands.reorderQueue,
    setQueue: commands.setQueue,
    applyQueuePatch: commands.applyQueuePatch,
    settleQueueReorder: commands.settleQueueReorder,
    switchRoom: coordinator.switchRoom
  };
};

describe('player controls', () => {
  beforeEach(() => {
    localStorage.clear();
    queryClient.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
    socketService.connected = false;
    socketService.send.mockReturnValue(true);
    socketService.sendEnvelope.mockReturnValue(true);
    roomApi.verify.mockResolvedValue({ accessGranted: true, expiresAt: Date.now() + 60_000 });
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

  it('allows named guests to send controls when connected', () => {
    const user = useUserStore();
    user.initUser('guest_a', 'Named guest', true);
    const player = usePlayerStore();
    player.connected = true;
    socketService.connected = true;

    const sent = player.toggleShuffle();

    expect(sent).toBe(true);
    expect(socketService.sendEnvelope).toHaveBeenCalledWith({ type: WS_DEST.PLAYER_SHUFFLE, payload: {} });
  });

  it('shows a toast when controls are used before socket connection', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('u1', 'Alice', false);
    const player = usePlayerStore();
    const toast = useToastStore();

    const sent = player.togglePause();

    expect(sent).toBe(false);
    expect(toast.toasts.at(-1).message).toContain('播放服务尚未连接');
    expect(socketService.sendEnvelope).not.toHaveBeenCalled();
  });

  it('blocks rapid repeated controls locally', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('u1', 'Alice', false);
    const player = usePlayerStore();
    player.connected = true;
    socketService.connected = true;

    expect(player.togglePause()).toBe(true);
    expect(player.playNext()).toBe(false);
    expect(socketService.sendEnvelope).toHaveBeenCalledTimes(1);
  });

  it('sends controls through the socket when connected and allowed', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('u1', 'Alice', false);
    const player = usePlayerStore();
    player.connected = true;
    socketService.connected = true;

    const sent = player.toggleShuffle();

    expect(sent).toBe(true);
    expect(socketService.sendEnvelope).toHaveBeenCalledWith({ type: WS_DEST.PLAYER_SHUFFLE, payload: {} });
  });

  it('optimistically reorders queue without immediate forced resync', () => {
    vi.useFakeTimers();
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('u1', 'Alice', false);
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
    expect(socketService.send).toHaveBeenCalledWith(WS_DEST.RESYNC, {});
  });

  it('defers queue broadcasts until a pending reorder is acknowledged', () => {
    localStorage.setItem('mp_username', 'Alice');
    const user = useUserStore();
    user.initUser('u1', 'Alice', false);
    const player = usePlayerStore();
    player.queue = [{ queueId: 'a' }, { queueId: 'b' }, { queueId: 'c' }];

    player.reorderQueue(0, 2, 'a', 'c', 'after');
    const payload = socketService.send.mock.calls.find(([type]) => type === WS_DEST.QUEUE_REORDER)[1];
    player.setQueue([{ queueId: 'a' }, { queueId: 'b' }, { queueId: 'c' }], 2);

    expect(player.queue.map(item => item.queueId)).toEqual(['b', 'c', 'a']);
    player.settleQueueReorder(payload.mutationId, true);
    expect(player.queue.map(item => item.queueId)).toEqual(['a', 'b', 'c']);
  });

  it('tracks enqueue commands with the mutation id required by the Go contract', () => {
    vi.useFakeTimers();
    const user = useUserStore();
    user.initUser('u1', 'Alice', false);
    const player = usePlayerStore();
    player.connected = true;
    socketService.connected = true;

    expect(player.enqueue('netease', 'track-1')).toBe(true);
    expect(socketService.sendEnvelope).toHaveBeenCalledWith({
      type: WS_DEST.ENQUEUE,
      payload: {
        platform: 'netease',
        musicId: 'track-1',
        mutationId: expect.any(String)
      }
    });

    vi.advanceTimersByTime(3000);
    expect(socketService.send).toHaveBeenCalledWith(WS_DEST.RESYNC, {});
  });

  it('applies ordered queue patches and resyncs on a version gap', () => {
    const player = usePlayerStore();
    player.setQueue([{ queueId: 'a' }], 1, { snapshot: true });

    expect(player.applyQueuePatch({ operation: 'append', queueVersion: 2, items: [{ queueId: 'b' }] })).toBe(true);
    expect(player.queue.map(item => item.queueId)).toEqual(['a', 'b']);
    expect(player.applyQueuePatch({ operation: 'remove', queueVersion: 3, queueIds: ['a'] })).toBe(true);
    expect(player.queue.map(item => item.queueId)).toEqual(['b']);

    expect(player.applyQueuePatch({ operation: 'append', queueVersion: 5, items: [{ queueId: 'c' }] })).toBe(false);
    expect(socketService.send).toHaveBeenCalledWith(WS_DEST.RESYNC, {});
  });

  it('applies status and clear patches without replacing unchanged queue items', () => {
    const player = usePlayerStore();
    const first = { queueId: 'a', status: 'PENDING' };
    const second = { queueId: 'b', status: 'READY' };
    player.setQueue([first, second], 1, { snapshot: true });
    const unchangedItem = player.queue[1];

    expect(player.applyQueuePatch({ operation: 'status', queueVersion: 2, queueId: 'a', status: 'DOWNLOADING' })).toBe(true);
    expect(player.queue[0]).toMatchObject({ queueId: 'a', status: 'DOWNLOADING' });
    expect(player.queue[1]).toBe(unchangedItem);
    expect(player.applyQueuePatch({ operation: 'clear', queueVersion: 3 })).toBe(true);
    expect(player.queue).toEqual([]);
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
    expect(roomStore.currentRoomId).toBe('lounge');
    expect(roomStore.transition).toEqual({ status: 'connecting', roomId: 'public' });
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
    user.initUser('u1', 'Alice', false);
    const roomStore = useRoomStore();
    const player = usePlayerStore();
    roomStore.setRooms([
      { roomId: 'lounge', name: 'Lounge' },
      { roomId: 'private', name: 'Private', privateRoom: true }
    ]);

    await player.switchRoom('private', 'letmein');

    expect(roomApi.verify).toHaveBeenCalledWith('private', 'letmein');
    expect(roomStore.rooms.find(room => room.roomId === 'private').accessGranted).toBe(true);
    expect(roomStore.currentRoomId).toBe('lounge');
    expect(roomStore.transition).toEqual({ status: 'connecting', roomId: 'private' });
    expect(socketService.disconnect).toHaveBeenCalled();
  });

  it('uses cached private room access without verifying again', async () => {
    const roomStore = useRoomStore();
    const player = usePlayerStore();
    roomStore.setRooms([
      { roomId: 'lounge', name: 'Lounge' },
      { roomId: 'private', name: 'Private', privateRoom: true, accessGranted: true }
    ]);
    await player.switchRoom('private');

    expect(roomApi.verify).not.toHaveBeenCalled();
    expect(roomStore.currentRoomId).toBe('lounge');
    expect(roomStore.transition).toEqual({ status: 'connecting', roomId: 'private' });
    expect(socketService.disconnect).toHaveBeenCalled();
  });
});
