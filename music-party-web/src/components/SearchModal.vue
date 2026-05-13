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
                <h2 class="text-[10px] font-black uppercase tracking-[0.2em] text-text-tertiary">Discover Music</h2>
              </div>
              <button class="text-text-muted transition-colors hover:text-text-primary" @click="emit('close')">
                <span class="material-symbols-outlined">close</span>
              </button>
            </div>

            <div class="flex gap-4">
              <input
                type="text"
                v-model="keyword"
                placeholder="Search for music..."
                class="flex-1 bg-surface-overlay border border-border-default rounded-[var(--radius-sm)] px-4 py-3 text-sm focus:border-primary outline-none text-text-primary placeholder:text-text-tertiary transition-colors"
                @keyup.enter="handleSearchAction"
              />
              <button @click="handleSearchAction" class="px-6 bg-primary text-on-primary font-black text-[10px] uppercase tracking-widest rounded-[var(--radius-sm)] hover:bg-[var(--accent-hover)] transition-colors">
                Search
              </button>
            </div>

            <!-- Search Type Tabs -->
            <div class="flex gap-2 mt-4">
              <button
                @click="searchType = 'song'"
                class="px-4 py-1.5 rounded-[var(--radius-sm)] text-[10px] font-black uppercase tracking-widest transition-colors"
                :class="searchType === 'song' ? 'bg-surface-raised border border-border-default text-primary' : 'bg-transparent border border-transparent text-text-muted hover:text-text-primary'"
              >
                Tracks
              </button>
              <button
                @click="searchType = 'album'"
                :disabled="!supportsAlbumSearch"
                class="px-4 py-1.5 rounded-[var(--radius-sm)] text-[10px] font-black uppercase tracking-widest transition-colors"
                :class="searchType === 'album' ? 'bg-surface-raised border border-border-default text-primary' : 'bg-transparent border border-transparent text-text-muted hover:text-text-primary disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:text-text-muted'"
              >
                Albums
              </button>
            </div>
          </div>

          <!-- Main Results Area -->
          <div class="flex-1 flex overflow-hidden">
            <!-- Sidebar -->
            <aside class="w-64 border-r border-border-default p-4 overflow-y-auto bg-surface-panel">
              <h3 class="text-[10px] font-black uppercase tracking-widest text-text-tertiary mb-4 px-2">Platforms</h3>
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
              <div v-if="loading" class="text-text-muted text-center py-20 font-compact">Searching...</div>
              <div v-else-if="displayItems.length === 0" class="text-text-muted text-[14px] text-center py-20 font-compact">{{ emptyMessage }}</div>

              <div v-else class="space-y-2">
                <div v-for="item in displayItems" :key="`${searchType}:${item.id}`" class="flex cursor-pointer items-center gap-4 rounded-lg border border-transparent p-3 transition-all hover:border-border-subtle hover:bg-[var(--surface-control-hover)]">
                  <img v-if="item.coverUrl" :src="item.coverUrl" class="w-10 h-10 rounded object-cover" />
                  <div v-else class="flex h-10 w-10 items-center justify-center rounded bg-[var(--surface-control)]">
                    <span class="material-symbols-outlined text-[20px] text-text-muted">{{ resultMode === 'album' ? 'album' : 'music_note' }}</span>
                  </div>
                  <div class="min-w-0 flex-1">
                    <p class="truncate text-text-primary font-compact text-[13px]">{{ item.name }}</p>
                    <p class="truncate text-text-muted text-[11px]">{{ formatArtists(item.artists || item.artistName) }}</p>
                  </div>
                  <button @click="handleAddClick(item)" class="text-primary hover:text-text-primary" :title="resultMode === 'album' ? 'Add album' : 'Add track'">
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
import { computed, watch } from 'vue';
import { usePlayerStore } from '../stores/player';
import { useSearchLogic } from '../composables/useSearchLogic';
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent } from 'reka-ui';

const props = defineProps(['isOpen']);
const emit = defineEmits(['close']);
const playerStore = usePlayerStore();

const { platform, platforms, supportsAlbumSearch, keyword, songs, albums, loading, searchType, doSearch, loadPlatforms } = useSearchLogic(emit);
const hasSearched = computed(() => keyword.value.trim().length > 0 || songs.value.length > 0 || albums.value.length > 0);
const resultMode = computed(() => searchType.value === 'album' && supportsAlbumSearch.value ? 'album' : 'song');
const displayItems = computed(() => resultMode.value === 'album' ? albums.value : songs.value);
const emptyMessage = computed(() => hasSearched.value ? 'No results found.' : 'Enter a keyword to start searching...');

watch(supportsAlbumSearch, (supported) => {
  if (!supported && searchType.value === 'album') {
    searchType.value = 'song';
  }
});

const handleSearchAction = () => doSearch();

const handleAddClick = (song) => {
  if (resultMode.value === 'album') {
    playerStore.enqueueAlbum(platform.value, song.id);
    return;
  }
  playerStore.enqueue(platform.value, song.id);
};

import { onMounted } from 'vue';
onMounted(loadPlatforms);

const formatArtists = (artists) => {
  if (Array.isArray(artists)) return artists.join(' / ');
  return artists || 'Unknown Artist';
};
</script>
