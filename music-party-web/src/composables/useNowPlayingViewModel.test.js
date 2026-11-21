import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import { createApp, h, nextTick } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { i18n } from '../i18n';
import { useNowPlayingViewModel } from './useNowPlayingViewModel';
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore';
import { useAudioPlaybackStore } from '../domains/playback/audioPlaybackStore';
import { useUiStore } from '../stores/ui';

vi.mock('../domains/playback/lyrics', () => ({ useLyricsStore: () => ({ load: vi.fn() }) }));

const source = readFileSync(resolve(process.cwd(), 'src/composables/useNowPlayingViewModel.ts'), 'utf8');

describe('Now playing seek permissions', () => {
  it('uses room capability instead of a role or display-name alias for seek permission', () => {
    expect(source).toContain('user.capabilities.canManageCurrentRoom');
    expect(source).not.toContain('AUTO_DJ');
    expect(source).not.toContain("currentUser.name");
  });
});

it('updates the rendered SoundCloud metadata, progress and cover theme from room state', async () => {
  const pinia = createPinia();
  setActivePinia(pinia);
  const runtime = useRoomRuntimeStore();
  const audio = useAudioPlaybackStore();
  const accent = vi.spyOn(useUiStore(), 'updateAccentFromCover').mockResolvedValue();
  const host = document.createElement('div');
  const app = createApp({ setup() {
    const vm = useNowPlayingViewModel();
    return () => h('div', `${vm.trackTitle.value}|${vm.artistLine.value}|${vm.platformLabel.value}|${vm.progressPercent.value}`);
  } });
  app.use(pinia).use(i18n).mount(host);
  try {
    const coverUrl = 'https://i1.sndcdn.com/artworks-000090810259-rywl9s-t500x500.jpg';
    runtime.nowPlaying = {
      music: { id: '54202938', name: 'Kill them All II', artists: ['Mystical Complex'], platform: 'soundcloud', duration: 377478, coverUrl },
      likedUserIds: [], likeMarkers: [],
    };
    audio.setPlaybackPosition(37747.8);
    await nextTick();
    expect(host.textContent).toBe('Kill them All II|Mystical Complex|SoundCloud|10%');
    expect(accent).toHaveBeenLastCalledWith(coverUrl);
  } finally {
    app.unmount();
    accent.mockRestore();
  }
});
