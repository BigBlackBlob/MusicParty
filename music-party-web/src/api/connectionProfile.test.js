import { beforeEach, describe, expect, it, vi } from 'vitest';

const axiosInstance = {
  defaults: {},
  interceptors: {
    response: {
      use: vi.fn()
    }
  }
};

vi.mock('axios', () => ({
  default: {
    create: vi.fn(() => axiosInstance)
  }
}));

const { getConnectionProfile, setConnectionProfile, resetConnectionProfileForTest } = await import('./connectionProfile');
const { default: client } = await import('./client');

describe('connection profile', () => {
  beforeEach(() => {
    localStorage.clear();
    axiosInstance.defaults = {};
    resetConnectionProfileForTest();
  });

  it('keeps browser deployments on the current origin by default', () => {
    expect(getConnectionProfile()).toEqual({ mode: 'browser', baseUrl: '' });
    expect(client.defaults.baseURL).toBe('');
  });

  it('applies a desktop host backend URL to axios defaults', () => {
    setConnectionProfile({ mode: 'host', baseUrl: 'http://127.0.0.1:48120/' });

    expect(getConnectionProfile()).toEqual({ mode: 'host', baseUrl: 'http://127.0.0.1:48120' });
    expect(client.defaults.baseURL).toBe('http://127.0.0.1:48120');
  });

  it('persists join mode backend URLs', () => {
    setConnectionProfile({ mode: 'join', baseUrl: 'http://192.168.1.10:48120/room/lobby' });
    resetConnectionProfileForTest();

    expect(getConnectionProfile()).toEqual({ mode: 'join', baseUrl: 'http://192.168.1.10:48120' });
  });
});
