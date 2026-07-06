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

const { getConnectionProfile, resetConnectionProfileForTest } = await import('./connectionProfile');
const { isTauriDesktop, startManagedDesktopHost, initializeDesktopHost } = await import('./desktopHost');

describe('desktop host bridge', () => {
  beforeEach(() => {
    localStorage.clear();
    axiosInstance.defaults = {};
    delete globalThis.__TAURI__;
    resetConnectionProfileForTest();
  });

  it('detects when the Tauri invoke bridge is missing', () => {
    expect(isTauriDesktop()).toBe(false);
  });

  it('starts the managed host and switches the frontend to host mode', async () => {
    globalThis.__TAURI__ = {
      core: {
        invoke: vi.fn().mockResolvedValue({
          backendUrl: 'http://127.0.0.1:48120',
          inviteUrl: 'http://192.168.1.10:48120/room/lobby'
        })
      }
    };

    const status = await startManagedDesktopHost({ lanBaseUrl: 'http://192.168.1.10:48120' });

    expect(status.inviteUrl).toBe('http://192.168.1.10:48120/room/lobby');
    expect(getConnectionProfile()).toEqual({ mode: 'host', baseUrl: 'http://127.0.0.1:48120' });
    expect(globalThis.__TAURI__.core.invoke).toHaveBeenCalledWith('start_desktop_host', {
      profileDir: null,
      backendJar: null,
      lanBaseUrl: 'http://192.168.1.10:48120'
    });
  });

  it('initializes the desktop host automatically when running inside Tauri', async () => {
    globalThis.__TAURI__ = {
      core: {
        invoke: vi.fn().mockResolvedValue({
          backendUrl: 'http://127.0.0.1:48120',
          inviteUrl: 'http://127.0.0.1:48120/room/lobby'
        })
      }
    };

    const status = await initializeDesktopHost();

    expect(status.backendUrl).toBe('http://127.0.0.1:48120');
    expect(getConnectionProfile()).toEqual({ mode: 'host', baseUrl: 'http://127.0.0.1:48120' });
    expect(globalThis.__TAURI__.core.invoke).toHaveBeenCalledWith('start_desktop_host', {
      profileDir: null,
      backendJar: null,
      lanBaseUrl: null
    });
  });

  it('keeps browser mode when desktop host startup is unavailable', async () => {
    const status = await initializeDesktopHost();

    expect(status).toBeNull();
    expect(getConnectionProfile()).toEqual({ mode: 'browser', baseUrl: '' });
  });
});
