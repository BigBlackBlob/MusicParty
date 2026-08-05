import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { authApi } from '../api/auth';
import { STORAGE_KEYS } from '../constants/keys';
import { useUserStore } from './user';

vi.mock('../api/auth', () => ({
  authApi: {
    getAccountMe: vi.fn(),
    updateAccountProfile: vi.fn(),
    changeAccountPassword: vi.fn(),
    logoutAccount: vi.fn()
  }
}));

describe('user store account actions', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('refreshes and updates the account profile display name', async () => {
    const user = useUserStore();
    user.initAccount({
      sessionToken: 'token-a',
      publicId: 'u_a',
      username: 'alice',
      displayName: 'Alice',
      role: 'USER',
      guest: false
    });
    authApi.updateAccountProfile.mockResolvedValueOnce({
      sessionToken: 'token-a',
      publicId: 'u_a',
      username: 'alice',
      displayName: 'Alice Cooper',
      role: 'USER',
      guest: false
    });

    await user.updateProfile('Alice Cooper');

    expect(authApi.updateAccountProfile).toHaveBeenCalledWith('Alice Cooper');
    expect(user.currentUser.name).toBe('Alice Cooper');
    expect(user.publicId).toBe('u_a');
  });

  it('ignores corrupt cached bindings during initialization', () => {
    localStorage.setItem(STORAGE_KEYS.BINDINGS, '{bad json');

    const user = useUserStore();

    expect(user.bindings).toEqual({});
    expect(localStorage.getItem(STORAGE_KEYS.BINDINGS)).toBeNull();
  });

  it('recognizes the Go platform administrator role', () => {
    const user = useUserStore();

    user.initAccount({
      sessionToken: '',
      publicId: 'u_admin',
      username: 'admin',
      displayName: 'Admin',
      role: 'PLATFORM_ADMIN',
      guest: false
    });

    expect(user.role).toBe('PLATFORM_ADMIN');
    expect(user.isAdmin).toBe(true);
  });

  it('never exposes a session token in the user store', () => {
    const user = useUserStore();

    user.initUser('u_listener', 'Listener', false, 'MEMBER', false);

    expect(user.sessionToken).toBeUndefined();
    expect(localStorage.getItem('mp_session_token')).toBeNull();
  });

  it('releases pending actions when a guest nickname is confirmed by the server', () => {
    const user = useUserStore();
    const callback = vi.fn();
    user.setPostNameAction(callback);
    user.showNameModal = true;

    user.initUser('guest_a', 'Named guest', true);

    expect(user.isGuest).toBe(true);
    expect(user.hasDisplayName).toBe(true);
    expect(user.showNameModal).toBe(false);
    expect(callback).toHaveBeenCalledOnce();
  });

  it('logs out and clears persisted account identity', async () => {
    const user = useUserStore();
    user.initAccount({
      sessionToken: 'token-a',
      publicId: 'u_a',
      username: 'alice',
      displayName: 'Alice',
      role: 'USER',
      guest: false
    });

    await user.logout();

    expect(authApi.logoutAccount).toHaveBeenCalledWith();
    expect(user.sessionToken).toBeUndefined();
    expect(user.role).toBe('GUEST');
    expect(localStorage.getItem('mp_session_token')).toBeNull();
    expect(localStorage.getItem('mp_account_username')).toBeNull();
  });

  it('clears local account identity even when logout request fails', async () => {
    const user = useUserStore();
    user.initAccount({
      sessionToken: 'token-a',
      publicId: 'u_a',
      username: 'alice',
      displayName: 'Alice',
      role: 'USER',
      guest: false
    });
    authApi.logoutAccount.mockRejectedValueOnce(new Error('offline'));

    await expect(user.logout()).rejects.toThrow('offline');

    expect(user.sessionToken).toBeUndefined();
    expect(user.publicId).toBe('');
    expect(localStorage.getItem('mp_session_token')).toBeNull();
  });
});
