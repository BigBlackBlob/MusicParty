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
    user.setOnlineUsers([{ publicId: 'u_a', name: 'Alice' }]);
    authApi.updateAccountProfile.mockResolvedValueOnce({
      sessionToken: 'token-a',
      publicId: 'u_a',
      username: 'alice',
      displayName: 'Alice Cooper',
      role: 'USER',
      guest: false
    });

    await user.updateProfile('Alice Cooper');

    expect(authApi.updateAccountProfile).toHaveBeenCalledWith('token-a', 'Alice Cooper');
    expect(user.currentUser.name).toBe('Alice Cooper');
    expect(user.onlineUsers[0].name).toBe('Alice Cooper');
    expect(user.publicId).toBe('u_a');
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

    expect(authApi.logoutAccount).toHaveBeenCalledWith('token-a');
    expect(user.sessionToken).toBe('');
    expect(user.role).toBe('GUEST');
    expect(localStorage.getItem(STORAGE_KEYS.SESSION_TOKEN)).toBeNull();
    expect(localStorage.getItem(STORAGE_KEYS.ACCOUNT_USERNAME)).toBeNull();
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

    expect(user.sessionToken).toBe('');
    expect(user.publicId).toBe('');
    expect(localStorage.getItem(STORAGE_KEYS.SESSION_TOKEN)).toBeNull();
  });
});
