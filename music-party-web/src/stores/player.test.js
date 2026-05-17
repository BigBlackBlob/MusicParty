import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { usePlayerStore } from './player';
import { useUserStore } from './user';
import { useToastStore } from './toast';
import { socketService } from '../services/socket';
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

describe('player controls', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
    socketService.connected = false;
    socketService.send.mockReturnValue(true);
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
});
