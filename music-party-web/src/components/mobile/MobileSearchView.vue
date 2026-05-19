<template>
  <section class="flex flex-col h-full bg-bg-base relative overflow-hidden">
    <!-- Header Section (Fixed Top) -->
    <header class="w-full z-20 bg-bg-base/95 backdrop-blur-md border-b border-border-default pt-4 pb-3 px-panelPadding flex flex-col gap-4 flex-shrink-0 safe-area-top">
      <!-- Search Bar & Cancel -->
      <form class="flex items-center gap-3" @submit.prevent="handleSearchAction">
        <div class="flex-1 flex items-center bg-surface-panel h-[40px] rounded-full px-3 border border-border-default focus-within:border-primary/50 transition-colors group">
          <span class="material-symbols-outlined text-text-secondary text-[20px] group-focus-within:text-primary">search</span>
          <input 
            v-model="keyword"
            class="flex-1 bg-transparent border-none outline-none text-body font-body text-primary placeholder-text-muted px-2 w-full" 
            :placeholder="t('search.placeholder')" 
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

      <!-- Platform Selector -->
      <div class="bg-surface-panel p-[4px] rounded-xl flex items-center justify-between shadow-inner">
        <button 
          v-for="p in platforms"
          :key="p.id"
          type="button"
          @click="platform = p.id"
          class="flex-1 py-1.5 rounded-lg font-section-label text-section-label transition-all"
          :class="platform === p.id ? 'bg-surface-raised text-primary shadow-sm border border-border-strong' : 'text-text-secondary hover:text-primary'"
        >
          <div class="flex items-center justify-center gap-1.5">
            <div 
              class="w-2 h-2 rounded-full" 
              :class="[
                p.id === 'netease' ? 'bg-platform-netease' : '',
                p.id === 'bilibili' ? 'bg-platform-bilibili' : '',
                p.id === 'navidrome' || p.subsonic ? 'bg-primary' : '',
                platform !== p.id ? 'opacity-50' : ''
              ]"
            ></div>
            {{ p.label }}
          </div>
        </button>
      </div>

      <div v-if="supportsAlbumSearch" class="flex p-xs bg-bg-base rounded-lg">
        <button 
          @click="searchType = 'song'"
          class="flex-1 py-1.5 text-center rounded-md font-section-label text-section-label transition-all"
          :class="searchType === 'song' ? 'bg-surface-raised text-primary shadow-sm' : 'text-text-secondary hover:text-text-primary'"
        >
          {{ t('search.song').toUpperCase() }}
        </button>
        <button 
          @click="searchType = 'album'"
          class="flex-1 py-1.5 text-center rounded-md font-section-label text-section-label transition-all"
          :class="searchType === 'album' ? 'bg-surface-raised text-primary shadow-sm' : 'text-text-secondary hover:text-text-primary'"
        >
          {{ t('search.album').toUpperCase() }}
        </button>
      </div>

    </header>

    <!-- Scrollable Results Area -->
    <main class="flex-1 overflow-y-auto px-panelPadding py-2 flex flex-col">
      <div v-if="loading" class="flex flex-col items-center justify-center py-20 text-center opacity-40">
        <span class="material-symbols-outlined text-[48px] animate-spin mb-2">refresh</span>
        <p class="font-compact text-compact uppercase tracking-widest">{{ t('search.loading') }}</p>
      </div>

      <template v-else-if="resultMode === 'album'">
        <div v-if="albums.length === 0" class="flex flex-col items-center justify-center py-20 text-center opacity-40">
          <span class="material-symbols-outlined text-[48px] mb-2">album</span>
          <p class="font-compact text-compact uppercase tracking-widest">{{ hasSearched ? t('search.noAlbums') : t('search.searchAlbums') }}</p>
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
                  <p class="font-compact text-compact text-primary truncate leading-tight">{{ album.name }}</p>
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
        <div v-if="songs.length === 0" class="flex flex-col items-center justify-center py-20 text-center opacity-40">
          <span class="material-symbols-outlined text-[48px] mb-2">music_note</span>
          <p class="font-compact text-compact uppercase tracking-widest">{{ hasSearched ? t('search.noResults') : t('search.searchSongs') }}</p>
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
                <p class="font-compact text-compact text-primary truncate leading-tight">{{ song.name }}</p>
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
  </section>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { usePlayerStore } from '../../stores/player';
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

const { t } = useI18n();
const player = usePlayerStore();
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
  searchType,
  hasSubmittedSearch,
  doSearch,
  albumSongs,
  loadingAlbumIds,
  expandedAlbumIds,
  toggleAlbum
} = useSearchLogic();

const selectedSongs = ref(new Set());

const hasSearched = computed(() => hasSubmittedSearch.value || songs.value.length > 0 || albums.value.length > 0);
const resultMode = computed(() => searchType.value === 'album' && supportsAlbumSearch.value ? 'album' : 'song');

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
.safe-area-top {
  padding-top: calc(env(safe-area-inset-top) + 16px);
}
</style>
