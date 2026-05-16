<template>
  <section class="mobile-work-page">
    <header class="mobile-search-header">
      <form class="mobile-search-box" @submit.prevent="handleSearchAction">
        <input
          id="mobile-search-input"
          v-model="keyword"
          :placeholder="isAdminMode ? t('search.adminPlaceholder') : t('search.placeholder')"
          autocomplete="off"
          enterkeyhint="search"
        />
        <IconButton variant="primary" size="lg" radius="sm" type="submit" :disabled="loading" :aria-label="t('search.searchAria')">
          <Loader2 v-if="loading" class="h-5 w-5 animate-spin" />
          <Search v-else class="h-5 w-5" />
        </IconButton>
      </form>

      <div class="mobile-platforms" :aria-label="t('search.platformAria')">
        <button
          v-for="p in platforms"
          :key="p.id"
          type="button"
          :class="{ 'mobile-platforms__item--active': platform === p.id }"
          @click="platform = p.id"
        >
          {{ p.label }}
        </button>
      </div>

      <SegmentedControl
        v-if="supportsAlbumSearch"
        v-model="searchType"
        :options="[
          { label: t('search.song'), value: 'song' },
          { label: t('search.album'), value: 'album' }
        ]"
      />
    </header>

    <div class="mobile-work-list">
      <div v-if="loading" class="mobile-empty">
        <Loader2 class="h-8 w-8 animate-spin text-[var(--accent)]" />
        <span>{{ t('search.loading') }}</span>
      </div>

      <template v-else-if="resultMode === 'album'">
        <div v-if="albums.length === 0" class="mobile-empty">
          <strong>{{ hasSearched ? t('search.noAlbums') : t('search.searchAlbums') }}</strong>
          <span>{{ hasSearched ? t('search.tryDifferent') : t('search.albumHint') }}</span>
        </div>

        <div v-for="album in albums" :key="album.id" class="flex flex-col gap-1">
          <TrackListItem
            :title="album.name"
            :artist="album.artistName || t('common.unknownArtist')"
            :cover-url="album.coverUrl"
            @click="toggleAlbum(album.id)"
          >
            <template #meta>
              {{ album.trackCount || 0 }} {{ t('queue.tracks') }}
            </template>
            <template #suffix>
              <div class="flex items-center gap-2">
                <IconButton variant="ghost" size="sm" @click.stop="toggleAlbum(album.id)">
                  <ChevronUp v-if="expandedAlbumIds.has(album.id)" class="h-4 w-4" />
                  <ChevronDown v-else class="h-4 w-4" />
                </IconButton>
                <IconButton variant="primary" size="sm" @click.stop="handleAddClick(album)" :aria-label="t('search.addAlbum')">
                  <Plus class="h-4 w-4" />
                </IconButton>
              </div>
            </template>
          </TrackListItem>

          <!-- Album Songs List (Mobile) -->
          <div v-if="expandedAlbumIds.has(album.id)" class="ml-4 space-y-1 mb-4 mt-1 border-l-2 border-surface-glass-border pl-2">
            <div v-if="loadingAlbumIds.has(album.id)" class="py-4 text-center text-text-muted text-[12px]">
              {{ t('search.loading') }}
            </div>
            <div v-else-if="albumSongs[album.id]" class="space-y-1">
              <!-- Album Multi-select Toolbar (Mobile) -->
              <div class="flex items-center justify-between px-2 py-2 mb-2 bg-surface-glass-control rounded-md border border-surface-glass-border">
                <div class="flex items-center gap-4">
                  <button @click="selectAllAlbumSongs(album.id)" class="text-[10px] font-black uppercase tracking-widest text-text-tertiary">
                    {{ t('search.selectAll') }}
                  </button>
                  <button @click="clearAlbumSongSelections(album.id)" class="text-[10px] font-black uppercase tracking-widest text-text-tertiary">
                    {{ t('search.deselectAll') }}
                  </button>
                </div>
                <button 
                  v-if="hasSelectedSongs(album.id)"
                  @click="addSelectedSongs(album.id)"
                  class="px-3 py-1 bg-primary text-on-primary rounded-full text-[10px] font-black uppercase tracking-widest"
                >
                  {{ t('search.addSelected') }}
                </button>
              </div>

              <div 
                v-for="(song, idx) in albumSongs[album.id]" :key="song.id"
                class="flex items-center gap-3 p-2 rounded-md active:bg-surface-glass-control transition-colors"
                @click="toggleSongSelection(album.id, song.id)"
              >
                <div class="flex h-5 w-5 items-center justify-center rounded-full border transition-colors shrink-0" :class="isSongSelected(album.id, song.id) ? 'border-primary bg-primary text-on-primary' : 'border-surface-glass-border bg-transparent text-transparent'">
                  <Check class="h-3 w-3" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="truncate text-[13px] text-text-primary">{{ song.name }}</p>
                  <p class="truncate text-[11px] text-text-muted">{{ formatArtists(song.artists) }}</p>
                </div>
                <IconButton variant="ghost" size="sm" @click.stop="player.enqueue(platform, song.id)">
                  <Plus class="h-4 w-4" />
                </IconButton>
              </div>
            </div>
          </div>
        </div>
      </template>

      <template v-else>
        <div v-if="songs.length === 0" class="mobile-empty">
          <strong>{{ hasSearched ? t('search.noResults') : t('search.searchSongs') }}</strong>
          <span>{{ hasSearched ? t('search.noResultsDesc') : t('search.startHint') }}</span>
        </div>

        <TrackListItem
          v-for="song in songs"
          v-else
          :key="`${song.platform || platform}:${song.id}`"
          :title="song.name"
          :artist="formatArtists(song.artists)"
          :cover-url="song.coverUrl"
        >
          <template #meta>
            {{ song.platform || platform }}
          </template>
          <template #suffix>
            <IconButton variant="primary" size="sm" @click="handleAddClick(song)" :aria-label="t('search.addSong')">
              <Plus class="h-4 w-4" />
            </IconButton>
          </template>
        </TrackListItem>
      </template>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Loader2, Plus, Search, ChevronDown, ChevronUp, Check } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useSearchLogic } from '../../composables/useSearchLogic';
import IconButton from '../ui/IconButton.vue';
import SegmentedControl from '../ui/SegmentedControl.vue';
import TrackListItem from '../ui/TrackListItem.vue';
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
  keyword,
  songs,
  albums,
  loading,
  searchType,
  isAdminMode,
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

const handleAddClick = (item) => {
  if (resultMode.value === 'album') {
    player.enqueueAlbum(platform.value, item.id);
    return;
  }
  player.enqueue(platform.value, item.id);
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
    player.enqueue(platform.value, s.id);
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
.mobile-work-page {
  display: grid;
  height: 100%;
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr);
  overflow: hidden;
  background: transparent;
}

.mobile-search-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
  border-bottom: 1px solid var(--surface-glass-border);
  background: var(--surface-glass-bg);
  backdrop-filter: blur(20px);
  padding: 14px 16px;
}

.mobile-search-box {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 48px;
  gap: 10px;
}

.mobile-search-box input {
  min-width: 0;
  height: 48px;
  border: 1px solid var(--surface-glass-border);
  border-radius: var(--radius-sm);
  background: var(--surface-glass-bg);
  color: var(--text-primary);
  font-size: 16px;
  outline: none;
  padding: 0 14px;
}

.mobile-search-box input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(211, 194, 243, 0.2);
}

.mobile-platforms {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: none;
}

.mobile-platforms::-webkit-scrollbar {
  display: none;
}

.mobile-platforms button {
  flex: 0 0 auto;
  min-height: 34px;
  border: 1px solid var(--surface-glass-border);
  border-radius: var(--radius-xs);
  background: var(--surface-glass-control);
  color: var(--text-tertiary);
  font-size: 10px;
  font-weight: 800;
  padding: 0 12px;
  text-transform: uppercase;
  transition: all 0.2s ease;
}

.mobile-platforms__item--active {
  border-color: var(--accent) !important;
  background: var(--accent) !important;
  color: var(--text-inverse) !important;
}

.mobile-work-list {
  display: flex;
  min-height: 0;
  flex-direction: column;
  gap: 6px;
  overflow-y: auto;
  padding: 10px 12px calc(12px + env(safe-area-inset-bottom));
}

.mobile-empty {
  display: flex;
  min-height: 240px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--text-tertiary);
  text-align: center;
}

.mobile-empty strong {
  color: var(--text-primary);
  font-size: 14px;
}

.mobile-empty span {
  max-width: 230px;
  font-size: 12px;
  line-height: 1.5;
}
</style>
