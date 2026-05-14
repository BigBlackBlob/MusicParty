<template>
  <DialogRoot :open="isOpen" @update:open="val => !val && emit('close')">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[var(--z-overlay)] bg-[var(--overlay-backdrop)] backdrop-blur-md transition-all duration-300" />
      <DialogContent class="fixed inset-0 z-[var(--z-overlay)] flex items-center justify-center p-8 outline-none">
        <!-- New Search UI Container -->
        <div class="w-full max-w-5xl h-[80vh] bg-surface-panel rounded-xl border border-border-default shadow-2xl flex flex-col overflow-hidden">
          <!-- Search Header -->
          <div class="p-6 border-b border-border-default bg-surface-panel">
            <div class="flex items-center justify-between mb-6">
              <div class="flex items-center gap-2">
                <span class="material-symbols-outlined text-primary text-[16px]">search</span>
                <h2 class="text-[10px] font-black uppercase tracking-[0.2em] text-text-tertiary">{{ t('search.discoverMusic') }}</h2>
              </div>
              <button class="text-text-muted transition-colors hover:text-text-primary" @click="emit('close')" :aria-label="t('common.close')" :title="t('common.close')">
                <span class="material-symbols-outlined">close</span>
              </button>
            </div>

            <div class="flex gap-4">
              <input
                type="text"
                v-model="keyword"
                :placeholder="isAdminMode ? t('search.adminPlaceholder') : t('search.placeholder')"
                class="flex-1 bg-surface-overlay border border-border-default rounded-[var(--radius-sm)] px-4 py-3 text-sm focus:border-primary outline-none text-text-primary placeholder:text-text-tertiary transition-colors"
                @keyup.enter="handleSearchAction"
              />
              <button @click="handleSearchAction" class="px-6 bg-primary text-on-primary font-black text-[10px] uppercase tracking-widest rounded-[var(--radius-sm)] hover:bg-[var(--accent-hover)] transition-colors" :aria-label="t('search.searchAria')" :title="t('search.searchAria')">
                {{ t('search.title') }}
              </button>
            </div>

            <!-- Search Type Tabs -->
            <div class="flex gap-2 mt-4">
              <button
                @click="searchType = SONG_SEARCH_TYPE"
                class="px-4 py-1.5 rounded-[var(--radius-sm)] text-[10px] font-black uppercase tracking-widest transition-colors"
                :class="searchType === SONG_SEARCH_TYPE ? 'bg-surface-raised border border-border-default text-primary' : 'bg-transparent border border-transparent text-text-muted hover:text-text-primary'"
              >
                {{ t('search.song') }}
              </button>
              <button
                @click="searchType = ALBUM_SEARCH_TYPE"
                :disabled="!supportsAlbumSearch"
                class="px-4 py-1.5 rounded-[var(--radius-sm)] text-[10px] font-black uppercase tracking-widest transition-colors"
                :class="searchType === ALBUM_SEARCH_TYPE ? 'bg-surface-raised border border-border-default text-primary' : 'bg-transparent border border-transparent text-text-muted hover:text-text-primary disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:text-text-muted'"
              >
                {{ t('search.album') }}
              </button>
            </div>
          </div>

          <!-- Main Results Area -->
          <div class="flex-1 flex overflow-hidden">
            <!-- Sidebar -->
            <aside class="w-64 border-r border-border-default p-4 overflow-y-auto bg-surface-panel">
              <h3 class="text-[10px] font-black uppercase tracking-widest text-text-tertiary mb-4 px-2">{{ t('search.platforms') }}</h3>
              <div class="space-y-1">
                <button
                  v-for="p in platforms" :key="p.id"
                  @click="platform = p.id"
                  class="w-full text-left px-3 py-2 rounded-[var(--radius-sm)] text-[13px] font-bold transition-colors"
                  :class="platform === p.id ? 'bg-surface-raised text-primary' : 'text-text-muted hover:bg-surface-raised hover:text-text-primary'"
                >
                  {{ p.label }}
                </button>
              </div>
            </aside>

            <!-- Results -->
            <main class="flex-1 p-6 overflow-y-auto bg-surface-stage space-y-2">
              <div v-if="loading" class="text-text-muted text-center py-20 font-compact">{{ t('search.loading') }}</div>
              <div v-else-if="displayItems.length === 0" class="flex flex-col items-center justify-center text-center py-20">
                <div class="text-[14px] text-text-primary font-bold">{{ emptyTitle }}</div>
                <div class="text-[12px] text-text-muted mt-1">{{ emptyMessage }}</div>
              </div>

              <div v-else class="space-y-2">
                <div v-for="item in displayItems" :key="`${searchType}:${item.id}`" class="flex cursor-pointer items-center gap-4 rounded-lg border border-transparent p-3 transition-all hover:border-border-subtle hover:bg-[var(--surface-control-hover)]">
                  <CoverImage v-if="item.coverUrl" :src="item.coverUrl" :alt="item.name" class="w-10 h-10 rounded object-cover" />
                  <div v-else class="flex h-10 w-10 items-center justify-center rounded bg-[var(--surface-control)]">
                    <span class="material-symbols-outlined text-[20px] text-text-muted">{{ resultMode === 'album' ? 'album' : 'music_note' }}</span>
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-text-primary font-compact text-[13px]">{{ item.name }}</p>
                    <p class="truncate text-text-muted text-[11px]">{{ formatArtists(item.artists || item.artistName) }}</p>
                  </div>
                  <button @click="handleAddClick(item)" class="text-primary hover:text-text-primary" :title="resultMode === ALBUM_SEARCH_TYPE ? t('search.addAlbum') : t('search.addSong')" :aria-label="resultMode === ALBUM_SEARCH_TYPE ? t('search.addAlbum') : t('search.addSong')">
                    <span class="material-symbols-outlined text-[20px]">{{ resultMode === 'album' ? 'library_add' : 'add_circle' }}</span>
                  </button>
                </div>
              </div>
            </main>
          </div>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup>
import { computed, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { usePlayerStore } from '../stores/player';
import { useSearchLogic } from '../composables/useSearchLogic';
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent } from 'reka-ui';
import CoverImage from './CoverImage.vue';

const props = defineProps(['isOpen']);
const emit = defineEmits(['close']);
const playerStore = usePlayerStore();
const { t } = useI18n();
const SONG_SEARCH_TYPE = 'song';
const ALBUM_SEARCH_TYPE = 'album';

const { platform, platforms, supportsAlbumSearch, keyword, songs, albums, loading, searchType, doSearch, loadPlatforms, isAdminMode, hasSubmittedSearch } = useSearchLogic(emit);
const hasSearched = computed(() => hasSubmittedSearch.value || songs.value.length > 0 || albums.value.length > 0);
const resultMode = computed(() => searchType.value === ALBUM_SEARCH_TYPE && supportsAlbumSearch.value ? ALBUM_SEARCH_TYPE : SONG_SEARCH_TYPE);
const displayItems = computed(() => resultMode.value === ALBUM_SEARCH_TYPE ? albums.value : songs.value);
const emptyTitle = computed(() => resultMode.value === ALBUM_SEARCH_TYPE
  ? (hasSearched.value ? t('search.noAlbums') : t('search.searchAlbums'))
  : (hasSearched.value ? t('search.noResults') : t('search.searchSongs')));
const emptyMessage = computed(() => resultMode.value === ALBUM_SEARCH_TYPE
  ? (hasSearched.value ? t('search.tryDifferent') : t('search.albumHint'))
  : (hasSearched.value ? t('search.noResultsDesc') : t('search.startHint')));

watch(supportsAlbumSearch, (supported) => {
  if (!supported && searchType.value === ALBUM_SEARCH_TYPE) {
    searchType.value = SONG_SEARCH_TYPE;
  }
});

const handleSearchAction = () => doSearch();

const handleAddClick = (song) => {
  if (resultMode.value === ALBUM_SEARCH_TYPE) {
    playerStore.enqueueAlbum(platform.value, song.id);
    return;
  }
  playerStore.enqueue(platform.value, song.id);
};

onMounted(loadPlatforms);

const formatArtists = (artists) => {
  if (Array.isArray(artists)) return artists.join(' / ');
  return artists || t('common.unknownArtist');
};
</script>
