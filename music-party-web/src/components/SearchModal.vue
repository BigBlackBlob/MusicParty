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
                :placeholder="searchPlaceholder"
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
              <button
                @click="searchType = PLAYLIST_SEARCH_TYPE"
                class="px-4 py-1.5 rounded-[var(--radius-sm)] text-[10px] font-black uppercase tracking-widest transition-colors"
                :class="searchType === PLAYLIST_SEARCH_TYPE ? 'bg-surface-raised border border-border-default text-primary' : 'bg-transparent border border-transparent text-text-muted hover:text-text-primary'"
              >
                {{ t('search.playlist') }}
              </button>
            </div>
          </div>

          <!-- Main Results Area -->
          <div class="flex-1 flex overflow-hidden">
            <!-- Sidebar -->
            <aside class="w-64 border-r border-border-default p-4 overflow-y-auto bg-surface-panel" v-if="searchType !== PLAYLIST_SEARCH_TYPE">
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
                <!-- Bulk Add for Playlist -->
                <div v-if="searchType === PLAYLIST_SEARCH_TYPE && playlistSongs.length > 0" class="mb-4 flex items-center justify-between p-3 rounded-lg bg-primary/5 border border-primary/20">
                   <div class="flex items-center gap-3">
                      <span class="material-symbols-outlined text-primary">playlist_add_check</span>
                      <span class="text-[12px] font-bold text-primary">{{ t('search.playlistLoaded', { count: playlistSongs.length }) }}</span>
                   </div>
                   <button 
                    @click="addAllPlaylistSongs"
                    class="px-4 py-1.5 bg-primary text-on-primary rounded-full text-[10px] font-black uppercase tracking-widest hover:bg-[var(--accent-hover)] transition-all shadow-lg shadow-primary/20"
                   >
                     {{ t('search.addAllSongs') }}
                   </button>
                </div>
                
                <div v-for="item in displayItems" :key="`${searchType}:${item.id}`" class="flex flex-col gap-1">
                  <div @click="resultMode === ALBUM_SEARCH_TYPE && toggleAlbum(item.id)" class="flex cursor-pointer items-center gap-4 rounded-lg border border-transparent p-3 transition-all hover:border-border-subtle hover:bg-[var(--surface-control-hover)]">
                    <CoverImage v-if="item.coverUrl" :src="item.coverUrl" :alt="item.name" class="w-10 h-10 rounded object-cover" />
                    <div v-else class="flex h-10 w-10 items-center justify-center rounded bg-[var(--surface-control)]">
                      <span class="material-symbols-outlined text-[20px] text-text-muted">{{ resultMode === 'album' ? 'album' : 'music_note' }}</span>
                    </div>
                    <div class="min-w-0 flex-1">
                      <p class="truncate text-text-primary font-compact text-[13px]">{{ item.name }}</p>
                      <p class="truncate text-text-muted text-[11px]">{{ formatArtists(item.artists || item.artistName) }}</p>
                    </div>
                    <div class="flex items-center gap-2">
                      <button v-if="resultMode === ALBUM_SEARCH_TYPE" @click.stop="toggleAlbum(item.id)" class="text-text-muted hover:text-text-primary transition-colors" :title="t('search.expandAlbum')">
                        <span class="material-symbols-outlined text-[20px]">{{ expandedAlbumIds.has(item.id) ? 'expand_less' : 'expand_more' }}</span>
                      </button>
                      <button @click.stop="handleAddClick(item)" class="text-primary hover:text-text-primary" :title="resultMode === ALBUM_SEARCH_TYPE ? t('search.addAlbum') : t('search.addSong')" :aria-label="resultMode === ALBUM_SEARCH_TYPE ? t('search.addAlbum') : t('search.addSong')">
                        <span class="material-symbols-outlined text-[20px]">{{ resultMode === 'album' ? 'library_add' : 'add_circle' }}</span>
                      </button>
                    </div>
                  </div>

                  <!-- Album Songs List -->
                  <div v-if="resultMode === ALBUM_SEARCH_TYPE && expandedAlbumIds.has(item.id)" class="ml-14 space-y-1 mb-4 mt-1">
                    <div v-if="loadingAlbumIds.has(item.id)" class="py-4 text-center text-text-muted text-[12px] font-compact">
                      {{ t('search.loading') }}
                    </div>
                    <div v-else-if="albumSongs[item.id]" class="space-y-1">
                      <!-- Album Multi-select Toolbar -->
                      <div class="flex items-center justify-between px-2 py-1 mb-2 bg-surface-raised rounded-md border border-border-default">
                         <div class="flex items-center gap-4">
                            <button @click="selectAllAlbumSongs(item.id)" class="text-[10px] font-black uppercase tracking-widest text-text-muted hover:text-primary transition-colors">
                               {{ t('search.selectAll') }}
                            </button>
                            <button @click="clearAlbumSongSelections(item.id)" class="text-[10px] font-black uppercase tracking-widest text-text-muted hover:text-error transition-colors">
                               {{ t('search.deselectAll') }}
                            </button>
                         </div>
                         <button 
                          v-if="hasSelectedSongs(item.id)"
                          @click="addSelectedSongs(item.id)"
                          class="px-3 py-1 bg-primary text-on-primary rounded-full text-[10px] font-black uppercase tracking-widest hover:bg-[var(--accent-hover)] transition-all shadow-md"
                         >
                            {{ t('search.addSelected') }}
                         </button>
                      </div>

                      <div 
                        v-for="(song, idx) in albumSongs[item.id]" :key="song.id"
                        class="group flex items-center gap-3 p-2 rounded-md hover:bg-surface-raised transition-colors cursor-pointer"
                        @click="toggleSongSelection(item.id, song.id)"
                      >
                         <div class="flex h-5 w-5 items-center justify-center rounded-full border transition-colors shrink-0" :class="isSongSelected(item.id, song.id) ? 'border-primary bg-primary text-on-primary' : 'border-border-default bg-transparent text-transparent'">
                            <span class="material-symbols-outlined text-[12px]">check</span>
                         </div>
                         <span class="text-[11px] font-mono text-text-tertiary w-4 shrink-0">{{ idx + 1 }}</span>
                         <div class="min-w-0 flex-1">
                            <p class="truncate text-[12px] text-text-primary font-bold">{{ song.name }}</p>
                            <p class="truncate text-[10px] text-text-muted">{{ formatArtists(song.artists) }}</p>
                         </div>
                         <button @click.stop="playerStore.enqueue(platform, song.id)" class="text-text-muted hover:text-primary transition-colors opacity-0 group-hover:opacity-100">
                            <span class="material-symbols-outlined text-[18px]">add_circle</span>
                         </button>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Pagination Control -->
                <div v-if="(searchType === SONG_SEARCH_TYPE || searchType === PLAYLIST_SEARCH_TYPE) && displayItems.length > 0 && (activeCurrentPage > 1 || activeCanGoNext)" class="pt-6 pb-4 flex items-center justify-center gap-6 border-t border-border-default/50 mt-4">
                   <!-- Prev -->
                   <button 
                    @click="prevPage"
                    :disabled="activeCurrentPage <= 1 || loading"
                    class="group flex items-center justify-center w-10 h-10 rounded-full text-text-muted hover:text-primary hover:bg-surface-raised disabled:opacity-30 disabled:hover:text-text-muted transition-all"
                    :title="t('search.prevPage')"
                   >
                     <span class="material-symbols-outlined text-[24px]">chevron_left</span>
                   </button>

                   <!-- Page Jump -->
                   <div class="flex items-center gap-3">
                      <input 
                        type="number"
                        :value="activeCurrentPage"
                        @keyup.enter="e => jumpToPage(e.target.value)"
                        class="w-12 h-10 flex items-center justify-center rounded bg-surface-raised border border-border-default text-primary font-mono text-sm font-bold text-center outline-none focus:border-primary transition-colors [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                        min="1"
                        step="1"
                      />
                   </div>

                   <!-- Next -->
                   <button 
                    @click="nextPage"
                    :disabled="!activeCanGoNext || loading"
                    class="group flex items-center justify-center w-10 h-10 rounded-full text-text-muted hover:text-primary hover:bg-surface-raised disabled:opacity-30 disabled:hover:text-text-muted transition-all"
                    :title="t('search.nextPage')"
                   >
                     <span class="material-symbols-outlined text-[24px]">chevron_right</span>
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
import { ref, computed, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { usePlayerStore } from '../stores/player';
import { useSearchLogic } from '../composables/useSearchLogic';
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent } from 'reka-ui';
import CoverImage from './CoverImage.vue';
import {
  addAlbumSelections,
  clearAlbumSelections,
  hasAlbumSelections,
  isAlbumSongSelected,
  selectedAlbumSongs,
  toggleAlbumSongSelection
} from '../utils/selection';

defineProps(['isOpen']);
const emit = defineEmits(['close']);
const playerStore = usePlayerStore();
const { t } = useI18n();
const SONG_SEARCH_TYPE = 'song';
const ALBUM_SEARCH_TYPE = 'album';
const PLAYLIST_SEARCH_TYPE = 'playlist';

const { 
  platform, platforms, supportsAlbumSearch, keyword, songs, albums, playlistSongs, loading, searchType, doSearch, loadPlatforms, isAdminMode, hasSubmittedSearch, 
  currentPage, currentPlaylistPage, canGoNext, canGoPlaylistNext, nextPage, prevPage, addAllPlaylistSongs,
  albumSongs, loadingAlbumIds, expandedAlbumIds, toggleAlbum
} = useSearchLogic(emit);

const selectedSongs = ref(new Set());

const hasSearched = computed(() => hasSubmittedSearch.value || songs.value.length > 0 || albums.value.length > 0 || playlistSongs.value.length > 0);
const activeCurrentPage = computed(() => searchType.value === PLAYLIST_SEARCH_TYPE ? currentPlaylistPage.value : currentPage.value);
const activeCanGoNext = computed(() => searchType.value === PLAYLIST_SEARCH_TYPE ? canGoPlaylistNext.value : canGoNext.value);

const resultMode = computed(() => {
  if (searchType.value === PLAYLIST_SEARCH_TYPE) return PLAYLIST_SEARCH_TYPE;
  return searchType.value === ALBUM_SEARCH_TYPE && supportsAlbumSearch.value ? ALBUM_SEARCH_TYPE : SONG_SEARCH_TYPE;
});
const displayItems = computed(() => {
  if (resultMode.value === PLAYLIST_SEARCH_TYPE) return playlistSongs.value;
  return resultMode.value === ALBUM_SEARCH_TYPE ? albums.value : songs.value;
});
const emptyTitle = computed(() => {
  if (resultMode.value === PLAYLIST_SEARCH_TYPE) return (hasSearched.value ? t('search.noPlaylist') : t('search.importPlaylist'));
  return resultMode.value === ALBUM_SEARCH_TYPE
    ? (hasSearched.value ? t('search.noAlbums') : t('search.searchAlbums'))
    : (hasSearched.value ? t('search.noResults') : t('search.searchSongs'));
});
const emptyMessage = computed(() => {
  if (resultMode.value === PLAYLIST_SEARCH_TYPE) return (hasSearched.value ? t('search.tryDifferent') : t('search.playlistHint'));
  return resultMode.value === ALBUM_SEARCH_TYPE
    ? (hasSearched.value ? t('search.tryDifferent') : t('search.albumHint'))
    : (hasSearched.value ? t('search.noResultsDesc') : t('search.startHint'));
});

const searchPlaceholder = computed(() => {
  if (isAdminMode.value) return t('search.adminPlaceholder');
  if (searchType.value === PLAYLIST_SEARCH_TYPE) return t('search.playlistPlaceholder');
  return t('search.placeholder');
});

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
    playerStore.enqueue(platform.value, s.id);
  });
  
  clearAlbumSongSelections(albumId);
};

onMounted(loadPlatforms);

const jumpToPage = (rawValue) => {
  const page = Math.max(1, Number.parseInt(rawValue, 10) || 1);
  doSearch(page);
};

const formatArtists = (artists) => {
  if (Array.isArray(artists)) return artists.join(' / ');
  return artists || t('common.unknownArtist');
};
</script>
