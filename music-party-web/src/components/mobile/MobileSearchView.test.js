import { createApp, nextTick, ref } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import en from '../../i18n/en';
import zh from '../../i18n/zh';
import MobileSearchView from './MobileSearchView.vue';

const mockPlatforms = ref([
  { id: 'netease', label: 'Netease', supportsAlbumSearch: true, subsonic: false },
  { id: 'bilibili', label: 'Bilibili', supportsAlbumSearch: false, subsonic: false },
  { id: 'youtube', label: 'YouTube', supportsAlbumSearch: false, subsonic: false },
  { id: 'soundcloud', label: 'SoundCloud', supportsAlbumSearch: false, subsonic: false },
  { id: 'qq', label: 'QQ Music', supportsAlbumSearch: false, subsonic: false },
  { id: 'squidify', label: 'Squidify', supportsAlbumSearch: true, subsonic: false },
  { id: 'local', label: 'Local', supportsAlbumSearch: false, subsonic: false },
  { id: 'navidrome', label: 'Navidrome', supportsAlbumSearch: true, subsonic: true },
  { id: 'subsonic-home', label: 'Home NAS', supportsAlbumSearch: true, subsonic: true }
]);

const mockPlatform = ref('netease');
const mockKeyword = ref('');
const mockSongs = ref([]);
const mockAlbums = ref([]);
const mockLoading = ref(false);
const mockSearchError = ref(false);
const mockSearchType = ref('song');
const mockHasSubmittedSearch = ref(false);
const mockLoadPlatforms = vi.fn().mockResolvedValue(undefined);
const mockRefreshPlatformsAndClear = vi.fn().mockResolvedValue(undefined);
const mockDoSearch = vi.fn().mockResolvedValue(undefined);

vi.mock('../../composables/useSearchLogic', async () => {
  const { computed } = await import('vue');
  return {
    useSearchLogic: () => ({
      platform: mockPlatform,
      platforms: mockPlatforms,
      supportsAlbumSearch: computed(() => {
        const current = mockPlatforms.value.find(p => p.id === mockPlatform.value);
        return Boolean(current?.supportsAlbumSearch);
      }),
      loadPlatforms: mockLoadPlatforms,
      refreshPlatformsAndClear: mockRefreshPlatformsAndClear,
      keyword: mockKeyword,
      songs: mockSongs,
      albums: mockAlbums,
      loading: mockLoading,
      searchError: mockSearchError,
      searchType: mockSearchType,
      hasSubmittedSearch: mockHasSubmittedSearch,
      doSearch: mockDoSearch,
      albumSongs: ref({}),
      loadingAlbumIds: ref(new Set()),
      expandedAlbumIds: ref(new Set()),
      toggleAlbum: vi.fn()
    })
  };
});

vi.mock('../../domains/realtime/roomCommandStore', () => ({
  useRoomCommandStore: () => ({
    enqueue: vi.fn(),
    enqueueAlbum: vi.fn()
  })
}));

let app;
let host;

const mountView = (locale = 'en') => {
  const pinia = createPinia();
  setActivePinia(pinia);
  const i18n = createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'en',
    messages: { en, zh }
  });
  host = document.createElement('div');
  document.body.appendChild(host);
  app = createApp(MobileSearchView);
  app.use(pinia);
  app.use(i18n);
  app.mount(host);
  return host;
};

describe('MobileSearchView source capsule rail & grouped drawer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPlatform.value = 'netease';
    mockKeyword.value = '';
    mockSongs.value = [];
    mockAlbums.value = [];
    mockLoading.value = false;
    mockSearchError.value = false;
    mockSearchType.value = 'song';
    mockHasSubmittedSearch.value = false;
  });

  afterEach(() => {
    app?.unmount();
    host?.remove();
    document.body.innerHTML = '';
  });

  it('renders all built-in and custom Subsonic sources in a scrollable non-shrinking capsule rail', async () => {
    const root = mountView('en');
    await nextTick();

    const rail = root.querySelector('.mobile-source-rail');
    expect(rail).not.toBeNull();

    const pills = Array.from(root.querySelectorAll('.mobile-source-rail__pill'));
    expect(pills).toHaveLength(mockPlatforms.value.length);

    // Every source renders its full label without being omitted
    const labels = pills.map(btn => btn.textContent?.trim());
    expect(labels).toContain('Netease');
    expect(labels).toContain('SoundCloud');
    expect(labels).toContain('Navidrome');
    expect(labels).toContain('Home NAS');

    // Active state and platform dot colour classes are preserved
    const neteasePill = root.querySelector('[data-platform-id="netease"]');
    const bilibiliPill = root.querySelector('[data-platform-id="bilibili"]');
    const subsonicPill = root.querySelector('[data-platform-id="subsonic-home"]');

    expect(neteasePill?.classList.contains('mobile-segment__option--active')).toBe(true);
    expect(neteasePill?.getAttribute('aria-selected')).toBe('true');
    expect(neteasePill?.querySelector('.bg-platform-netease')).not.toBeNull();
    expect(bilibiliPill?.querySelector('.bg-platform-bilibili')).not.toBeNull();
    expect(bilibiliPill?.querySelector('.opacity-50')).not.toBeNull();
    expect(subsonicPill?.querySelector('.bg-primary')).not.toBeNull();

    // Selecting another pill updates active state
    bilibiliPill?.click();
    await nextTick();
    expect(mockPlatform.value).toBe('bilibili');
    expect(bilibiliPill?.classList.contains('mobile-segment__option--active')).toBe(true);
    expect(neteasePill?.classList.contains('mobile-segment__option--active')).toBe(false);
  });

  it('opens the All (N) grouped two-column source drawer and switches platforms from it', async () => {
    const root = mountView('zh');
    await nextTick();

    const trigger = root.querySelector('[data-testid="source-drawer-trigger"]');
    expect(trigger?.textContent?.trim()).toBe('全部 (9)');

    trigger?.click();
    await nextTick();

    const drawer = document.body.querySelector('[data-testid="source-drawer"]');
    expect(drawer).not.toBeNull();

    const streamingGroup = drawer?.querySelector('[data-testid="source-group-streaming"]');
    const selfHostedGroup = drawer?.querySelector('[data-testid="source-group-self-hosted"]');
    expect(streamingGroup).not.toBeNull();
    expect(selfHostedGroup).not.toBeNull();

    const streamingButtons = streamingGroup?.querySelectorAll('[data-drawer-platform-id]') ?? [];
    const selfHostedButtons = selfHostedGroup?.querySelectorAll('[data-drawer-platform-id]') ?? [];
    expect(streamingButtons).toHaveLength(6);
    expect(selfHostedButtons).toHaveLength(3);

    const customSubsonicBtn = selfHostedGroup?.querySelector('[data-drawer-platform-id="subsonic-home"]');
    expect(customSubsonicBtn?.textContent).toContain('Home NAS');
    customSubsonicBtn?.click();
    await nextTick();

    expect(mockPlatform.value).toBe('subsonic-home');
    expect(trigger?.getAttribute('aria-expanded')).toBe('false');
    const drawerAfterPick = document.body.querySelector('[data-testid="source-drawer"]');
    expect(drawerAfterPick === null || drawerAfterPick.getAttribute('data-state') === 'closed').toBe(true);
  });

  it('keeps SoundCloud URL-only hint and Local library state intact', async () => {
    const root = mountView('en');
    await nextTick();

    mockPlatform.value = 'soundcloud';
    await nextTick();

    const input = root.querySelector('input');
    expect(input?.getAttribute('placeholder')).toBe(en.search.soundcloudPlaceholder);
    expect(root.textContent).toContain(en.search.soundcloudTitle);
    expect(root.textContent).toContain(en.search.soundcloudHint);

    mockPlatform.value = 'local';
    await nextTick();

    expect(input?.getAttribute('placeholder')).toBe(en.search.placeholder);
    expect(root.textContent).toContain(en.search.localBrowseTitle);
    expect(root.textContent).toContain(en.search.localBrowseHint);
  });
});
