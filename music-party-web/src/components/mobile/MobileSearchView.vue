<template>
  <section class="flex h-full flex-col overflow-hidden">
    <div class="border-b border-[var(--border-default)] bg-[var(--surface-1)] px-4 py-3">
      <label for="mobile-search-input" class="sr-only">搜索音乐</label>
      <div class="flex gap-2">
        <input
          id="mobile-search-input"
          v-model="keyword"
          class="min-h-[48px] min-w-0 flex-1 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)] px-4 text-base text-[var(--text-primary)] outline-none focus:border-[var(--border-accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
          placeholder="搜索歌曲或专辑"
          @keyup.enter="runSearch"
        />
        <button class="min-h-[48px] min-w-[52px] rounded-2xl bg-[var(--accent)] text-[var(--text-inverse)] active:scale-[0.96]" @click="runSearch" aria-label="搜索">
          <Search class="mx-auto h-5 w-5" />
        </button>
      </div>

      <div class="mt-3 grid grid-cols-2 gap-2">
        <button
          v-for="p in ['netease', 'bilibili']"
          :key="p"
          class="min-h-[40px] rounded-xl text-sm font-semibold"
          :class="platform === p ? 'bg-[var(--accent)] text-[var(--text-inverse)]' : 'bg-[var(--surface-2)] text-[var(--text-secondary)]'"
          @click="platform = p"
        >
          {{ p }}
        </button>
      </div>

      <div v-if="platform === 'netease'" class="mt-2 grid grid-cols-2 gap-2 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)] p-1">
        <button class="min-h-[38px] rounded-xl text-sm font-semibold" :class="searchType === 'song' ? 'bg-[var(--surface-4)] text-[var(--text-primary)]' : 'text-[var(--text-tertiary)]'" @click="searchType = 'song'">歌曲</button>
        <button class="min-h-[38px] rounded-xl text-sm font-semibold" :class="searchType === 'album' ? 'bg-[var(--surface-4)] text-[var(--text-primary)]' : 'text-[var(--text-tertiary)]'" @click="searchType = 'album'">专辑</button>
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 py-3">
      <div v-if="loading" class="flex justify-center py-10 text-[var(--accent)]">
        <Loader2 class="h-6 w-6 animate-spin" />
      </div>

      <template v-else-if="searchType === 'album' && platform === 'netease'">
        <div v-if="albums.length === 0" class="py-12 text-center text-sm text-[var(--text-tertiary)]">暂无专辑结果</div>
        <div v-for="album in albums" :key="album.id" class="mb-2 grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] p-3">
          <div class="h-14 w-14 overflow-hidden rounded-2xl bg-[var(--surface-3)]">
            <CoverImage :src="album.coverUrl" :alt="`${album.name} 封面`" loading="lazy" class="h-full w-full" />
          </div>
          <div class="min-w-0">
            <div class="truncate text-sm font-semibold">{{ album.name }}</div>
            <div class="truncate text-xs text-[var(--text-secondary)]">{{ album.artistName || '未知艺术家' }}</div>
            <div class="text-[11px] text-[var(--text-tertiary)]">{{ album.trackCount }} 首</div>
          </div>
          <button class="min-h-[44px] rounded-full bg-[var(--accent)] px-3 text-xs font-semibold text-[var(--text-inverse)]" @click="player.enqueueAlbum('netease', album.id)">导入</button>
        </div>
      </template>

      <template v-else>
        <div v-if="songs.length === 0" class="py-12 text-center text-sm text-[var(--text-tertiary)]">暂无歌曲结果</div>
        <div v-for="song in songs" :key="song.id" class="mb-2 grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] p-3">
          <div class="h-14 w-14 overflow-hidden rounded-2xl bg-[var(--surface-3)]">
            <CoverImage :src="song.coverUrl" :alt="`${song.name} 封面`" loading="lazy" class="h-full w-full" />
          </div>
          <div class="min-w-0">
            <div class="truncate text-sm font-semibold">{{ song.name }}</div>
            <div class="truncate text-xs text-[var(--text-secondary)]">{{ song.artists.join(' / ') }}</div>
          </div>
          <button class="flex min-h-[44px] min-w-[44px] items-center justify-center rounded-full bg-[var(--surface-3)] text-[var(--accent)]" @click="player.enqueue(platform, song.id)" aria-label="添加到队列">
            <Plus class="h-5 w-5" />
          </button>
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
import CoverImage from '../CoverImage.vue';

const player = usePlayerStore();
const platform = ref('netease');
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
    if (platform.value === 'netease' && searchType.value === 'album') {
      albums.value = await musicApi.searchNeteaseAlbums(q);
    } else {
      songs.value = await musicApi.search(platform.value, q);
    }
  } finally {
    loading.value = false;
  }
};

watch(platform, (next) => {
  if (next !== 'netease') searchType.value = 'song';
});
</script>
