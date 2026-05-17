import { beforeEach, describe, expect, it, vi } from 'vitest';
import { socketService } from './socket';

vi.mock('@stomp/stompjs', () => {
  class Client {
    constructor(config) {
      this.config = config;
      this.active = false;
      this.publish = vi.fn();
      this.subscribe = vi.fn((topic, handler) => ({ id: topic, unsubscribe: vi.fn(), handler }));
      this.deactivate = vi.fn(() => {
        this.active = false;
      });
    }

    activate() {
      this.active = true;
    }
  }

  return { Client };
});

describe('socketService', () => {
  beforeEach(() => {
    socketService.disconnect();
    socketService.stompConfig = null;
    socketService.connected = false;
    vi.clearAllMocks();
  });

  it('reconnects active clients when auth context changes', () => {
    socketService.connect({ 'room-id': 'lounge' }, {}, { '/topic/a': vi.fn() });
    const firstClient = socketService.client;

    socketService.connect({ 'room-id': 'stage' }, {}, { '/topic/a': vi.fn() });

    expect(firstClient.deactivate).toHaveBeenCalled();
    expect(socketService.client).not.toBe(firstClient);
  });

  it('returns false instead of silently dropping sends while disconnected', () => {
    const sent = socketService.send('/app/control/toggle-pause');

    expect(sent).toBe(false);
  });

  it('publishes and returns true when connected', () => {
    socketService.connect({ 'room-id': 'lounge' }, {}, {});
    socketService.connected = true;

    const sent = socketService.send('/app/control/toggle-pause', { ok: true });

    expect(sent).toBe(true);
    expect(socketService.client.publish).toHaveBeenCalledWith({
      destination: '/app/control/toggle-pause',
      body: JSON.stringify({ ok: true })
    });
  });
});
