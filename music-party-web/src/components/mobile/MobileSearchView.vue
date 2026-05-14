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

        <TrackListItem
          v-for="album in albums"
          v-else
          :key="album.id"
          :title="album.name"
          :artist="album.artistName || t('common.unknownArtist')"
          :cover-url="album.coverUrl"
        >
          <template #meta>
            {{ album.trackCount || 0 }} {{ t('queue.tracks') }}
          </template>
          <template #suffix>
            <IconButton variant="primary" size="sm" @click="handleAddClick(album)" :aria-label="t('search.addAlbum')">
              <Plus class="h-4 w-4" />
            </IconButton>
          </template>
        </TrackListItem>
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
import { computed, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Loader2, Plus, Search } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useSearchLogic } from '../../composables/useSearchLogic';
import IconButton from '../ui/IconButton.vue';
import SegmentedControl from '../ui/SegmentedControl.vue';
import TrackListItem from '../ui/TrackListItem.vue';

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
  doSearch
} = useSearchLogic();

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
