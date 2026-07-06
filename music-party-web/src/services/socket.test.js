import { beforeEach, describe, expect, it, vi } from 'vitest';
import { socketService } from './socket';

const sockets = [];

class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  constructor(url) {
    this.url = url;
    this.readyState = MockWebSocket.CONNECTING;
    this.send = vi.fn();
    this.close = vi.fn(() => {
      this.readyState = MockWebSocket.CLOSED;
    });
    sockets.push(this);
  }

  open() {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.({ type: 'open' });
  }

  message(data) {
    this.onmessage?.({ data });
  }

  closeFromServer(event = { code: 1006 }) {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.(event);
  }
}

describe('socketService', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    sockets.length = 0;
    global.WebSocket = MockWebSocket;
    socketService.disconnect();
    socketService.socketConfig = null;
    socketService.connected = false;
    vi.clearAllMocks();
  });

  it('reconnects active clients when auth context changes', () => {
    socketService.connect({ 'room-id': 'lounge' }, {}, { 'player.state': vi.fn() });
    const firstClient = socketService.client;

    socketService.connect({ 'room-id': 'stage' }, {}, { 'player.state': vi.fn() });

    expect(firstClient.close).toHaveBeenCalled();
    expect(socketService.client).not.toBe(firstClient);
    expect(socketService.client.url).toContain('room-id=stage');
  });

  it('uses the configured backend origin when building socket URLs', async () => {
    const { setConnectionProfile } = await import('../api/connectionProfile');
    setConnectionProfile({ mode: 'join', baseUrl: 'http://192.168.1.10:48120/room/lobby' });

    socketService.connect({ 'room-id': 'lounge' }, {}, {});

    expect(socketService.client.url).toBe('ws://192.168.1.10:48120/ws?room-id=lounge');
  });

  it('returns false instead of silently dropping sends while disconnected', () => {
    const sent = socketService.send('control.toggle-pause');

    expect(sent).toBe(false);
  });

  it('sends JSON envelopes and returns true when connected', () => {
    vi.setSystemTime(1000);
    socketService.connect({ 'room-id': 'lounge' }, {}, {});
    socketService.client.open();

    const sent = socketService.send('control.toggle-pause', { ok: true });

    expect(sent).toBe(true);
    expect(socketService.client.send).toHaveBeenCalledWith(JSON.stringify({
      type: 'control.toggle-pause',
      requestId: 'control.toggle-pause-1000-1',
      payload: { ok: true }
    }));
  });

  it('dispatches received envelopes by type', () => {
    const handler = vi.fn();
    socketService.connect({ 'room-id': 'lounge' }, {}, { 'player.state': handler });
    socketService.client.open();

    socketService.client.message(JSON.stringify({
      type: 'player.state',
      roomId: 'lounge',
      payload: { isPaused: true }
    }));

    expect(handler).toHaveBeenCalledWith({ isPaused: true }, expect.objectContaining({ roomId: 'lounge' }));
  });

  it('marks disconnected and schedules reconnect after unexpected close', () => {
    const onDisconnect = vi.fn();
    socketService.connect({ 'room-id': 'lounge' }, { onDisconnect }, {});
    const firstClient = socketService.client;
    firstClient.open();

    firstClient.closeFromServer({ code: 1006 });

    expect(socketService.connected).toBe(false);
    expect(onDisconnect).toHaveBeenCalled();
    vi.advanceTimersByTime(2000);
    expect(socketService.client).not.toBe(firstClient);
  });

  it('treats policy close as an auth error without reconnecting', () => {
    const onAuthError = vi.fn();
    socketService.connect({ 'room-id': 'private' }, { onAuthError }, {});
    const firstClient = socketService.client;
    firstClient.open();

    firstClient.closeFromServer({ code: 1008 });

    expect(onAuthError).toHaveBeenCalled();
    vi.advanceTimersByTime(2000);
    expect(socketService.client).toBe(firstClient);
  });
});
