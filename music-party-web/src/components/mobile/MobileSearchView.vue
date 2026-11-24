<template>
  <section class="flex flex-col h-full bg-transparent relative overflow-hidden">
    <!-- Header Section (Fixed Top) -->
    <header class="w-full z-20 bg-transparent border-b border-border-default pt-3 pb-2 px-panelPadding flex flex-col gap-3 flex-shrink-0 safe-area-top">
      <!-- Search Bar & Cancel -->
      <form class="flex items-center gap-3" @submit.prevent="handleSearchAction">
        <div class="flex-1 flex items-center bg-surface-panel h-[40px] rounded-full px-3 border border-border-default focus-within:border-primary/50 transition-colors group">
          <span class="material-symbols-outlined text-text-secondary text-[20px] group-focus-within:text-primary">search</span>
          <input 
            v-model="keyword"
            class="flex-1 bg-transparent border-none outline-none text-body font-body text-text-primary placeholder-text-muted px-2 w-full"
            :placeholder="searchPlaceholder"
            type="text"
            autocomplete="off"
            enterkeyhint="search"
          />
          <button 
            v-if="keyword"
            type="button"
            @click="keyword = ''"
            class="text-text-secondary hover:text-text-primary flex items-center justify-center w-10 h-10 -mr-1 rounded-full transition-colors"
          >
            <span class="material-symbols-outlined text-[18px]">cancel</span>
          </button>
        </div>
        <button 
          type="button"
          @click="refreshSearchSources"
          class="text-text-secondary hover:text-primary font-compact text-compact transition-colors whitespace-nowrap px-2 py-2 -my-2"
          :title="t('search.refreshSources')"
        >
          <span class="material-symbols-outlined text-[20px]">sync</span>
        </button>
      </form>

      <!-- Platform Selector: horizontally scrollable capsule rail + All (N) drawer trigger -->
      <div class="mobile-source-rail-row flex items-center gap-2 min-w-0">
        <div
          ref="sourceRailRef"
          class="mobile-segment mobile-source-rail flex-1 min-w-0"
          role="tablist"
          :aria-label="t('search.platformAria')"
        >
          <button
            v-for="p in platforms"
            :key="p.id"
            :ref="(el) => setSourcePillRef(p.id, el)"
            type="button"
            role="tab"
            :aria-selected="platform === p.id"
            :data-platform-id="p.id"
            @click="selectPlatform(p.id)"
            class="mobile-segment__option mobile-source-rail__pill font-section-label text-section-label transition-all"
            :class="{ 'mobile-segment__option--active': platform === p.id }"
          >
            <span class="flex items-center justify-center gap-1.5 whitespace-nowrap">
              <span
                class="w-2 h-2 rounded-full flex-shrink-0"
                :class="[getPlatformDotClass(p), platform !== p.id ? 'opacity-50' : '']"
              ></span>
              <span>{{ getPlatformLabel(p) }}</span>
            </span>
          </button>
        </div>

        <button
          type="button"
          data-testid="source-drawer-trigger"
          @click="isSourceDrawerOpen = true"
          class="mobile-source-all-btn flex-shrink-0 font-micro text-micro text-text-secondary hover:text-text-primary border border-border-default bg-surface-panel rounded-[var(--radius-sm)] px-2.5 h-[36px] inline-flex items-center gap-1 whitespace-nowrap transition-colors"
          :aria-expanded="isSourceDrawerOpen"
          aria-haspopup="dialog"
        >
          <span>{{ t('search.allSources', { count: platforms.length }) }}</span>
        </button>
      </div>

      <div v-if="supportsAlbumSearch" class="mobile-segment mobile-search-types">
        <button 
          @click="searchType = 'song'"
          class="mobile-segment__option font-section-label text-section-label transition-all"
          :class="{ 'mobile-segment__option--active': searchType === 'song' }"
        >
          {{ t('search.song').toUpperCase() }}
        </button>
        <button 
          @click="searchType = 'album'"
          class="mobile-segment__option font-section-label text-section-label transition-all"
          :class="{ 'mobile-segment__option--active': searchType === 'album' }"
        >
          {{ t('search.album').toUpperCase() }}
        </button>
      </div>

    </header>

    <!-- Scrollable Results Area -->
    <main class="flex-1 overflow-y-auto px-panelPadding py-2 flex flex-col">
      <div v-if="loading" class="mobile-content-state">
        <span class="material-symbols-outlined mobile-content-state__icon">refresh</span>
        <p class="font-compact text-compact">{{ t('search.loading') }}</p>
      </div>

      <div v-else-if="searchError" class="mobile-content-state" role="status">
        <span class="material-symbols-outlined mobile-content-state__icon">cloud_off</span>
        <p class="font-compact text-compact">{{ t('search.failed') }}</p>
        <p class="text-xs text-text-secondary">{{ t('search.noResultsDesc') }}</p>
      </div>

      <template v-else-if="resultMode === 'album'">
        <div v-if="albums.length === 0" class="mobile-content-state">
          <span class="material-symbols-outlined mobile-content-state__icon">album</span>
          <p class="font-compact text-compact">{{ hasSearched ? t('search.noAlbums') : t('search.searchAlbums') }}</p>
        </div>

        <div v-else>
          <h2 class="font-section-label text-section-label text-text-secondary uppercase tracking-widest mb-3 px-1">{{ t('search.album') }}</h2>
          <div class="flex flex-col gap-1">
            <div v-for="album in albums" :key="album.id" class="flex flex-col gap-1">
              <div 
                class="group flex items-center gap-3 p-1.5 rounded hover:bg-surface-panel active:bg-accent-subtle transition-colors cursor-pointer border border-transparent hover:border-border-default"
                @click="toggleAlbum(album.id)"
              >
                <div class="w-[44px] h-[44px] rounded bg-surface-elevated overflow-hidden flex-shrink-0 relative shadow-[0_2px_4px_rgba(0,0,0,0.3)]">
                  <CoverImage :src="album.coverUrl" class="w-full h-full object-cover" />
                </div>
                <div class="flex-1 min-w-0 flex flex-col justify-center">
                  <p class="font-compact text-compact text-text-primary truncate leading-tight">{{ album.name }}</p>
                  <p class="font-caption text-caption text-text-secondary truncate leading-tight mt-0.5">
                    {{ album.artistName || t('common.unknownArtist') }} • {{ album.trackCount || 0 }} {{ t('queue.tracks') }}
                  </p>
                </div>
                <button 
                  @click.stop="handleAddClick(album)"
                  class="w-[36px] h-[36px] rounded flex items-center justify-center text-text-muted hover:text-primary hover:bg-surface-raised active:scale-95 transition-all flex-shrink-0"
                >
                  <span class="material-symbols-outlined text-[22px]">add</span>
                </button>
              </div>

              <!-- Album Songs List -->
              <div v-if="expandedAlbumIds.has(album.id)" class="ml-4 space-y-1 mb-4 mt-1 border-l-2 border-border-default pl-2">
                <div v-if="loadingAlbumIds.has(album.id)" class="py-4 text-center text-text-muted text-[10px] uppercase tracking-widest">
                  {{ t('search.loading') }}
                </div>
                <div v-else-if="albumSongs[album.id]" class="space-y-1">
                  <div class="flex items-center justify-between px-2 py-2 mb-2 bg-surface-panel rounded-md border border-border-default">
                    <div class="flex items-center gap-4">
                      <button @click="selectAllAlbumSongs(album.id)" class="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                        {{ t('search.selectAll') }}
                      </button>
                      <button @click="clearAlbumSongSelections(album.id)" class="text-[10px] font-bold uppercase tracking-widest text-text-secondary">
                        {{ t('search.deselectAll') }}
                      </button>
                    </div>
                    <button 
                      v-if="hasSelectedSongs(album.id)"
                      @click="addSelectedSongs(album.id)"
                      class="px-3 py-1 bg-primary text-on-primary rounded-full text-[10px] font-bold uppercase tracking-widest"
                    >
                      {{ t('search.addSelected') }}
                    </button>
                  </div>

                  <div 
                    v-for="song in albumSongs[album.id]" :key="song.id"
                    class="flex items-center gap-3 p-2 rounded-md active:bg-accent-subtle transition-colors"
                    @click="toggleSongSelection(album.id, song.id)"
                  >
                    <div class="flex h-5 w-5 items-center justify-center rounded-full border transition-colors shrink-0" :class="isSongSelected(album.id, song.id) ? 'border-primary bg-primary text-on-primary' : 'border-border-default bg-transparent text-transparent'">
                      <span v-if="isSongSelected(album.id, song.id)" class="material-symbols-outlined text-[14px]">check</span>
                    </div>
                    <div class="min-w-0 flex-1">
                      <p class="truncate text-compact font-compact" :class="isSongSelected(album.id, song.id) ? 'text-primary' : 'text-text-primary'">{{ song.name }}</p>
                      <p class="truncate text-caption font-caption text-text-secondary">{{ formatArtists(song.artists) }}</p>
                    </div>
                    <button 
                      @click.stop="player.enqueue(song.platform || platform, song.id)"
                      class="w-[32px] h-[32px] flex items-center justify-center rounded-full hover:bg-surface-raised text-text-secondary hover:text-primary"
                    >
                      <span class="material-symbols-outlined text-[20px]">add</span>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <template v-else>
        <div v-if="songs.length === 0" class="mobile-content-state">
          <span class="material-symbols-outlined mobile-content-state__icon">{{ emptySongIcon }}</span>
          <p class="font-compact text-compact">{{ emptySongTitle }}</p>
          <p v-if="emptySongHint" class="text-caption font-caption text-text-secondary max-w-[280px]">{{ emptySongHint }}</p>
        </div>

        <div v-else>
          <h2 class="font-section-label text-section-label text-text-secondary uppercase tracking-widest mb-3 px-1">{{ t('search.results') }}</h2>
          <div class="flex flex-col gap-1">
            <div 
              v-for="song in songs" 
              :key="`${song.platform || platform}:${song.id}`"
              class="group flex items-center gap-3 p-1.5 rounded hover:bg-surface-panel active:bg-accent-subtle transition-colors cursor-pointer border border-transparent hover:border-border-default"
              @click="handleAddClick(song)"
            >
              <div class="w-[44px] h-[44px] rounded bg-surface-elevated overflow-hidden flex-shrink-0 relative shadow-[0_2px_4px_rgba(0,0,0,0.3)]">
                <CoverImage :src="song.coverUrl" class="w-full h-full object-cover group-hover:opacity-60 transition-opacity duration-200" />
                <div class="absolute inset-0 hidden group-hover:flex items-center justify-center">
                  <span class="material-symbols-outlined text-primary text-[24px]" style="font-variation-settings: 'FILL' 1;">play_arrow</span>
                </div>
              </div>
              <div class="flex-1 min-w-0 flex flex-col justify-center">
                <p class="mobile-search-track-title font-compact text-compact text-text-primary leading-tight">{{ song.name }}</p>
                <p class="font-caption text-caption text-text-secondary truncate leading-tight mt-0.5">
                  {{ formatArtists(song.artists) }} • {{ song.platform || platform }}
                </p>
              </div>
              <button 
                @click.stop="handleAddClick(song)"
                class="w-[44px] h-[44px] rounded flex items-center justify-center text-text-muted hover:text-primary hover:bg-surface-raised active:scale-95 transition-all flex-shrink-0"
              >
                <span class="material-symbols-outlined text-[22px]">add</span>
              </button>
            </div>
          </div>
        </div>
      </template>
    </main>
    <!-- All Sources Grouped Drawer -->
    <DialogRoot :open="isSourceDrawerOpen" @update:open="isSourceDrawerOpen = $event">
      <DialogPortal>
        <DialogOverlay class="mp-dialog-overlay fixed inset-0 z-[var(--z-overlay)] bg-[var(--overlay-backdrop)] backdrop-blur-sm" />
        <DialogContent
          data-testid="source-drawer"
          class="mp-dialog-content fixed inset-x-0 bottom-0 z-[var(--z-modal)] mx-auto max-w-[520px] rounded-t-[var(--radius-lg)] bg-surface-overlay border-t border-x border-border-default shadow-2xl outline-none flex flex-col max-h-[75dvh]"
        >
          <div class="flex justify-center pt-2 pb-1">
            <div class="w-9 h-1 rounded-full bg-border-strong" aria-hidden="true"></div>
          </div>

          <div class="flex items-center justify-between px-md py-2.5 border-b border-border-subtle">
            <DialogTitle class="font-compact text-compact font-semibold text-text-primary">
              {{ t('search.sourceDrawerTitle') }}
            </DialogTitle>
            <DialogClose as-child>
              <button
                type="button"
                class="w-8 h-8 rounded-[var(--radius-sm)] border border-border-default bg-surface-panel text-text-secondary hover:text-text-primary flex items-center justify-center transition-colors"
                :aria-label="t('common.close')"
              >
                <span class="material-symbols-outlined text-[18px]">close</span>
              </button>
            </DialogClose>
          </div>

          <div class="overflow-y-auto px-md py-3 pb-[calc(env(safe-area-inset-bottom)+16px)] flex flex-col gap-4">
            <div v-if="streamingPlatforms.length > 0" data-testid="source-group-streaming">
              <div class="font-micro text-micro uppercase tracking-wider text-text-muted mb-2">
                {{ t('search.sourceGroupStreaming') }} ({{ streamingPlatforms.length }})
              </div>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="p in streamingPlatforms"
                  :key="p.id"
                  type="button"
                  :data-drawer-platform-id="p.id"
                  @click="pickPlatformFromDrawer(p.id)"
                  class="flex flex-col items-start gap-1 p-2.5 rounded-[var(--radius-md)] border text-left transition-colors"
                  :class="platform === p.id
                    ? 'border-primary bg-accent-subtle text-text-primary'
                    : 'border-border-default bg-surface-panel text-text-secondary hover:text-text-primary'"
                >
                  <div class="w-full flex items-center justify-between gap-2">
                    <span class="inline-flex items-center gap-1.5 min-w-0 font-compact text-compact text-text-primary">
                      <span
                        class="w-2 h-2 rounded-full flex-shrink-0"
                        :class="[getPlatformDotClass(p), platform !== p.id ? 'opacity-50' : '']"
                      ></span>
                      <span class="truncate">{{ getPlatformLabel(p) }}</span>
                    </span>
                    <span v-if="platform === p.id" class="material-symbols-outlined text-primary text-[16px] flex-shrink-0">check</span>
                  </div>
                  <span class="font-micro text-micro text-text-muted truncate w-full">
                    {{ getPlatformCapability(p) }}
                  </span>
                </button>
              </div>
            </div>

            <div v-if="selfHostedPlatforms.length > 0" data-testid="source-group-self-hosted">
              <div class="font-micro text-micro uppercase tracking-wider text-text-muted mb-2">
                {{ t('search.sourceGroupSelfHosted') }} ({{ selfHostedPlatforms.length }})
              </div>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="p in selfHostedPlatforms"
                  :key="p.id"
                  type="button"
                  :data-drawer-platform-id="p.id"
                  @click="pickPlatformFromDrawer(p.id)"
                  class="flex flex-col items-start gap-1 p-2.5 rounded-[var(--radius-md)] border text-left transition-colors"
                  :class="platform === p.id
                    ? 'border-primary bg-accent-subtle text-text-primary'
                    : 'border-border-default bg-surface-panel text-text-secondary hover:text-text-primary'"
                >
                  <div class="w-full flex items-center justify-between gap-2">
                    <span class="inline-flex items-center gap-1.5 min-w-0 font-compact text-compact text-text-primary">
                      <span
                        class="w-2 h-2 rounded-full flex-shrink-0"
                        :class="[getPlatformDotClass(p), platform !== p.id ? 'opacity-50' : '']"
                      ></span>
                      <span class="truncate">{{ getPlatformLabel(p) }}</span>
                    </span>
                    <span v-if="platform === p.id" class="material-symbols-outlined text-primary text-[16px] flex-shrink-0">check</span>
                  </div>
                  <span class="font-micro text-micro text-text-muted truncate w-full">
                    {{ getPlatformCapability(p) }}
                  </span>
                </button>
              </div>
            </div>
          </div>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </section>
</template>

<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue';
import { useI18n } from 'vue-i18n';
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogClose } from 'reka-ui';
import { useRoomCommandStore } from '../../domains/realtime/roomCommandStore';
import { useSearchLogic } from '../../composables/useSearchLogic';
import CoverImage from '../CoverImage.vue';
import {
  addAlbumSelections,
  clearAlbumSelections,
  hasAlbumSelections,
  isAlbumSongSelected,
  selectedAlbumSongs,
  toggleAlbumSongSelection
} from '../../utils/selection';

const { t, te } = useI18n();
const player = useRoomCommandStore();
const {
  platform,
  platforms,
  supportsAlbumSearch,
  loadPlatforms,
  refreshPlatformsAndClear,
  keyword,
  songs,
  albums,
  loading,
  searchError,
  searchType,
  hasSubmittedSearch,
  doSearch,
  albumSongs,
  loadingAlbumIds,
  expandedAlbumIds,
  toggleAlbum
} = useSearchLogic();

const selectedSongs = ref(new Set());
const isSourceDrawerOpen = ref(false);
const sourceRailRef = ref(null);
const sourcePillRefs = new Map();

const setSourcePillRef = (id, el) => {
  if (el) {
    sourcePillRefs.set(id, el);
  } else {
    sourcePillRefs.delete(id);
  }
};

const hasSearched = computed(() => hasSubmittedSearch.value || songs.value.length > 0 || albums.value.length > 0);
const resultMode = computed(() => searchType.value === 'album' && supportsAlbumSearch.value ? 'album' : 'song');

const isSelfHostedPlatform = (p) => p.subsonic || p.id === 'local' || p.id === 'navidrome';

const streamingPlatforms = computed(() => platforms.value.filter(p => !isSelfHostedPlatform(p)));
const selfHostedPlatforms = computed(() => platforms.value.filter(p => isSelfHostedPlatform(p)));

const getPlatformLabel = (p) => {
  const key = `platforms.${p.id}`;
  return te(key) ? t(key) : p.label;
};

const getPlatformDotClass = (p) => {
  if (p.id === 'netease') return 'bg-platform-netease';
  if (p.id === 'bilibili') return 'bg-platform-bilibili';
  if (p.id === 'navidrome' || p.subsonic) return 'bg-primary';
  return 'bg-text-muted';
};

const getPlatformCapability = (p) => {
  if (p.id === 'soundcloud') return t('search.sourceCapUrlOnly');
  if (p.id === 'local') return t('search.sourceCapLocalBrowse');
  if (p.subsonic || p.id === 'navidrome') return t('search.sourceCapSubsonic');
  if (p.supportsAlbumSearch) return t('search.sourceCapAlbumAndSong');
  return t('search.sourceCapSongOnly');
};

const searchPlaceholder = computed(() => {
  if (platform.value === 'soundcloud') return t('search.soundcloudPlaceholder');
  return t('search.placeholder');
});

const emptySongIcon = computed(() => {
  if (platform.value === 'soundcloud' && !hasSearched.value) return 'link';
  if (platform.value === 'local' && !hasSearched.value) return 'folder_open';
  return 'music_note';
});

const emptySongTitle = computed(() => {
  if (platform.value === 'soundcloud' && !hasSearched.value) return t('search.soundcloudTitle');
  if (platform.value === 'local' && !hasSearched.value) return t('search.localBrowseTitle');
  return hasSearched.value ? t('search.noResults') : t('search.searchSongs');
});

const emptySongHint = computed(() => {
  if (platform.value === 'soundcloud' && !hasSearched.value) return t('search.soundcloudHint');
  if (platform.value === 'local' && !hasSearched.value) return t('search.localBrowseHint');
  return '';
});

const scrollActivePillIntoView = async (id) => {
  await nextTick();
  const el = sourcePillRefs.get(id);
  if (el && typeof el.scrollIntoView === 'function') {
    el.scrollIntoView({ behavior: 'smooth', inline: 'center', block: 'nearest' });
  }
};

const selectPlatform = (id) => {
  platform.value = id;
};

const pickPlatformFromDrawer = (id) => {
  platform.value = id;
  isSourceDrawerOpen.value = false;
  scrollActivePillIntoView(id);
};

watch(platform, (nextId) => {
  scrollActivePillIntoView(nextId);
});

const handleSearchAction = () => doSearch();

const refreshSearchSources = async () => {
  keyword.value = '';
  await refreshPlatformsAndClear();
};

const handleAddClick = (item) => {
  if (resultMode.value === 'album') {
    player.enqueueAlbum(platform.value, item.id);
    return;
  }
  player.enqueue(item.platform || platform.value, item.id);
};

const toggleSongSelection = (albumId, songId) => {
  toggleAlbumSongSelection(selectedSongs.value, platform.value, albumId, songId);
};

const isSongSelected = (albumId, songId) => {
  return isAlbumSongSelected(selectedSongs.value, platform.value, albumId, songId);
};

const selectAllAlbumSongs = (albumId) => {
  const songs = albumSongs.value[albumId] || [];
  addAlbumSelections(selectedSongs.value, platform.value, albumId, songs);
};

const clearAlbumSongSelections = (albumId) => {
  const songs = albumSongs.value[albumId] || [];
  clearAlbumSelections(selectedSongs.value, platform.value, albumId, songs);
};

const hasSelectedSongs = (albumId) => {
  const songs = albumSongs.value[albumId] || [];
  return hasAlbumSelections(selectedSongs.value, platform.value, albumId, songs);
};

const addSelectedSongs = (albumId) => {
  const songs = albumSongs.value[albumId] || [];
  const toAdd = selectedAlbumSongs(selectedSongs.value, platform.value, albumId, songs);
  if (toAdd.length === 0) return;
  
  toAdd.forEach(s => {
    player.enqueue(s.platform || platform.value, s.id);
  });
  
  clearAlbumSongSelections(albumId);
};

const formatArtists = (artists) => Array.isArray(artists) && artists.length ? artists.join(' / ') : t('common.unknownArtist');

watch(supportsAlbumSearch, (supported) => {
  if (!supported && searchType.value === 'album') searchType.value = 'song';
});

onMounted(loadPlatforms);
</script>

<style scoped>
.mobile-source-rail {
  padding: 2px;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
  -webkit-overflow-scrolling: touch;
  flex-wrap: nowrap;
}

.mobile-source-rail::-webkit-scrollbar {
  display: none;
}

.mobile-source-rail__pill {
  flex: 0 0 auto;
  min-width: auto;
  min-height: 32px;
  padding: 6px 12px;
  white-space: nowrap;
}

.mobile-search-types {
  padding: 2px;
}

.mobile-search-types .mobile-segment__option {
  min-height: 32px;
}

.mobile-search-track-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 360px) {
  .mobile-search-track-title {
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    white-space: normal;
  }
}

.safe-area-top {
  padding-top: calc(env(safe-area-inset-top) + 16px);
}
</style>
