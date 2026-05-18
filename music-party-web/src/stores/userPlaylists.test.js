import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useUserStore } from './user';
import { useUserPlaylistsStore } from './userPlaylists';
import { personalPlaylistsApi } from '../api/personalPlaylists';

vi.mock('../api/personalPlaylists', () => ({
  personalPlaylistsApi: {
    list: vi.fn(),
    create: vi.fn(),
    tracks: vi.fn(),
    addTracks: vi.fn(),
    importNetease: vi.fn(),
    enqueue: vi.fn()
  }
}));

describe('userPlaylists store', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
    const user = useUserStore();
    user.initUser('token-a', 'u_a', 'Alice', false);
  });

  it('creates a playlist before saving when none exists', async () => {
    personalPlaylistsApi.list.mockResolvedValueOnce([]);
    personalPlaylistsApi.create.mockResolvedValueOnce({ id: 'p1', name: 'Mine' });
    personalPlaylistsApi.list.mockResolvedValueOnce([{ id: 'p1', name: 'Mine' }]);
    personalPlaylistsApi.addTracks.mockResolvedValue({ addedCount: 1, skippedCount: 0, tracks: [] });
    personalPlaylistsApi.tracks.mockResolvedValue([]);

    const store = useUserPlaylistsStore();
    const result = await store.addTracksToSelected([{ id: 's1', platform: 'netease', name: 'Song' }], 'Mine');

    expect(result).toMatchObject({ addedCount: 1, skippedCount: 0 });
    expect(personalPlaylistsApi.create).toHaveBeenCalledWith('token-a', 'Mine');
    expect(personalPlaylistsApi.addTracks).toHaveBeenCalledWith('token-a', 'p1', expect.any(Array));
  });

  it('prompts guests to set a name instead of writing', async () => {
    const user = useUserStore();
    user.initUser('token-guest', 'u_guest', '游客', true);
    const store = useUserPlaylistsStore();

    const result = await store.addTracksToSelected([{ id: 's1', platform: 'netease' }]);

    expect(result).toBeNull();
    expect(user.showNameModal).toBe(true);
    expect(personalPlaylistsApi.addTracks).not.toHaveBeenCalled();
  });
});
