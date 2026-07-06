import { setConnectionProfile } from './connectionProfile';

export function isTauriDesktop() {
  return Boolean(globalThis.__TAURI__?.core?.invoke);
}

export async function startManagedDesktopHost(options = {}) {
  const invoke = globalThis.__TAURI__?.core?.invoke;
  if (!invoke) {
    throw new Error('Tauri desktop host is not available');
  }
  const status = await invoke('start_desktop_host', {
    profileDir: options.profileDir || null,
    backendJar: options.backendJar || null,
    lanBaseUrl: options.lanBaseUrl || null
  });
  if (status?.backendUrl) {
    setConnectionProfile({ mode: 'host', baseUrl: status.backendUrl });
  }
  return status;
}

export async function initializeDesktopHost(options = {}) {
  if (!isTauriDesktop()) return null;
  try {
    return await startManagedDesktopHost(options);
  } catch (error) {
    console.warn('Desktop host startup failed:', error);
    return null;
  }
}

export async function stopManagedDesktopHost() {
  const invoke = globalThis.__TAURI__?.core?.invoke;
  if (!invoke) return;
  await invoke('stop_desktop_host');
}

export async function getManagedDesktopStatus() {
  const invoke = globalThis.__TAURI__?.core?.invoke;
  if (!invoke) return null;
  return invoke('desktop_status');
}
