import { beforeEach, describe, expect, it } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useUserStore } from './user';
import { STORAGE_KEYS } from '../constants/keys';

describe('user store identity', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
  });

  it('stores server-issued session token and public id', () => {
    const store = useUserStore();

    store.initUser('session-secret', 'u_public', 'Alice', false);

    expect(store.sessionToken).toBe('session-secret');
    expect(store.publicId).toBe('u_public');
    expect(localStorage.getItem(STORAGE_KEYS.SESSION_TOKEN)).toBe('session-secret');
    expect(store.resolveName('u_public')).toBe('Alice');
  });
});
