<template>
  <section class="flex h-full flex-col overflow-hidden bg-[var(--surface-0)]">
    <div class="border-b border-[var(--border-default)] bg-[var(--surface-1)] px-4 py-4 flex flex-col gap-4">
      <!-- Search Input -->
      <div class="flex gap-2">
        <div class="relative flex-1">
          <input
            id="mobile-search-input"
            v-model="keyword"
            class="w-full h-11 px-4 bg-[var(--surface-2)] border border-[var(--border-default)] rounded-[var(--radius-sm)] text-base text-[var(--text-primary)] outline-none focus:border-[var(--border-accent)] transition-all placeholder:text-[var(--text-tertiary)]"
            placeholder="Search music..."
            @keyup.enter="runSearch"
          />
        </div>
        <IconButton variant="primary" size="lg" radius="sm" @click="runSearch">
          <Search class="h-5 w-5" />
        </IconButton>
      </div>

      <!-- Platform Selection -->
      <div class="flex gap-2 overflow-x-auto no-scrollbar pb-1">
        <button
          v-for="p in platforms"
          :key="p.id"
          class="flex-shrink-0 px-4 py-2 rounded-[var(--radius-xs)] text-[9px] font-black uppercase tracking-widest transition-all border"
          :class="platform === p.id
            ? 'bg-[var(--accent)] border-[var(--accent)] text-[var(--text-inverse)]'
            : 'bg-[var(--surface-3)] border-[var(--border-default)] text-[var(--text-tertiary)]'"
          @click="platform = p.id"
        >
          {{ p.label }}
        </button>
      </div>

      <!-- Search Type -->
      <SegmentedControl
        v-if="supportsAlbumSearch"
        v-model="searchType"
        :options="[
          { label: '歌曲', value: 'song' },
          { label: '专辑', value: 'album' }
        ]"
      />
    </div>

    <!-- Results -->
    <div class="flex-1 overflow-y-auto px-3 py-3">
      <div v-if="loading" class="flex flex-col items-center justify-center py-20 gap-4 opacity-50">
        <Loader2 class="h-8 w-8 animate-spin text-[var(--accent)]" />
        <span class="text-[10px] font-bold uppercase tracking-widest text-[var(--text-tertiary)]">Searching Room...</span>
      </div>

      <template v-else-if="searchType === 'album' && supportsAlbumSearch">
        <div v-if="albums.length === 0" class="py-20 text-center text-xs font-bold text-[var(--text-tertiary)] uppercase tracking-widest">No albums found</div>
        <div v-for="album in albums" :key="album.id" class="flex items-center gap-4 p-3 bg-[var(--surface-1)] border border-[var(--border-default)] rounded-[var(--radius-md)] mb-2 shadow-sm">
          <div class="h-14 w-14 flex-shrink-0 overflow-hidden rounded-[var(--radius-xs)] bg-[var(--surface-3)]">
            <CoverImage :src="album.coverUrl" class="h-full w-full object-cover" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="text-sm font-bold truncate">{{ album.name }}</div>
            <div class="text-[10px] font-bold text-[var(--text-secondary)] uppercase tracking-wider">{{ album.artistName || 'Unknown Artist' }}</div>
            <div class="text-[9px] text-[var(--text-tertiary)] uppercase font-bold">{{ album.trackCount }} Tracks</div>
          </div>
          <IconButton variant="primary" size="sm" @click="player.enqueueAlbum('netease', album.id)">
            <Plus class="h-4 w-4" />
          </IconButton>
        </div>
      </template>

      <template v-else>
        <div v-if="songs.length === 0" class="py-20 text-center text-xs font-bold text-[var(--text-tertiary)] uppercase tracking-widest">No tracks found</div>
        <div class="space-y-1">
          <TrackListItem
            v-for="song in songs"
            :key="song.id"
            :title="song.name"
            :artist="song.artists.join(' / ')"
            :cover-url="song.coverUrl"
          >
            <template #suffix>
              <IconButton variant="primary" size="sm" @click="player.enqueue(platform, song.id)">
                <Plus class="h-4 w-4" />
              </IconButton>
            </template>
          </TrackListItem>
        </div>
      </template>
    </div>
  </section>
</template>

<script setup>
import { ref, watch } from 'vue';
import { Loader2, Plus, Search } from 'lucide-vue-next';
import { musicApi } from '../../api/music';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';
import { usePlatforms } from '../../composables/usePlatforms';
import { useToast } from '../../composables/useToast';
import { extractErrorMessage } from '../../utils/errors';
import CoverImage from '../CoverImage.vue';

// UI Primitives
import IconButton from '../ui/IconButton.vue';
import SegmentedControl from '../ui/SegmentedControl.vue';
import TrackListItem from '../ui/TrackListItem.vue';

const player = usePlayerStore();
const userStore = useUserStore();
const { error } = useToast();
const platform = ref('netease');
const { platforms, supportsAlbumSearch } = usePlatforms(platform);
const searchType = ref('song');
const keyword = ref('');
const songs = ref([]);
const albums = ref([]);
const loading = ref(false);

const runSearch = async () => {
  const q = keyword.value.trim();
  if (!q) return;
  loading.value = true;
  songs.value = [];
  albums.value = [];
  try {
    if (supportsAlbumSearch.value && searchType.value === 'album') {
      albums.value = await musicApi.searchNeteaseAlbums(q);
    } else {
      songs.value = await musicApi.search(platform.value, q, userStore.userToken);
    }
  } catch (e) {
    console.error('Mobile search failed:', e);
    error(extractErrorMessage(e, '搜索失败'));
  } finally {
    loading.value = false;
  }
};

watch(platform, (next) => {
  if (!supportsAlbumSearch.value) searchType.value = 'song';
});
</script>

<style scoped>
.no-scrollbar::-webkit-scrollbar { display: none; }
.no-scrollbar { -ms-overflow-style: none; scrollbar-width: none; }
</style>
