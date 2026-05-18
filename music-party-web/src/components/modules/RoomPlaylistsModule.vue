<template>
  <div class="room-playlists-module flex flex-col h-full overflow-hidden bg-surface-panel/30">
    <!-- Header -->
    <header class="flex items-center justify-between px-4 py-3 border-b border-border-subtle bg-surface-panel/50 shrink-0">
      <div v-if="viewMode === 'index'" class="flex flex-col">
        <span class="text-[10px] font-black uppercase tracking-[0.2em] text-text-muted leading-none mb-1">{{ scope === 'room' ? t('roomPlaylists.kicker') : t('personalPlaylists.kicker') }}</span>
        <h2 class="text-sm font-bold uppercase tracking-widest text-text-primary">{{ t('roomPlaylists.title') }}</h2>
      </div>
      
      <div v-else class="flex items-center gap-3 overflow-hidden">
        <button 
          @click="goBack"
          class="flex items-center justify-center w-8 h-8 rounded-full bg-surface-raised border border-border-default text-text-muted hover:text-text-primary hover:border-border-strong transition-all shrink-0"
        >
          <span class="material-symbols-outlined text-[18px]">arrow_back</span>
        </button>
        <div class="flex flex-col min-w-0">
          <span class="text-[9px] font-black uppercase tracking-[0.15em] text-primary truncate leading-none mb-1">{{ t('roomPlaylists.title') }}</span>
          <h2 class="text-sm font-bold text-text-primary truncate">{{ store.selectedPlaylist?.name || 'Loading...' }}</h2>
        </div>
      </div>

      <div class="flex items-center gap-2 shrink-0">
        <button 
          v-if="viewMode === 'detail' && store.selectedTracks.length > 0"
          @click="playSelected"
          class="flex items-center justify-center w-8 h-8 rounded-full bg-primary text-text-inverse shadow-lg shadow-primary/20 hover:scale-105 active:scale-95 transition-all"
          :title="t('roomPlaylists.playAll')"
        >
          <span class="material-symbols-outlined text-[20px]" style="font-variation-settings: 'FILL' 1;">play_arrow</span>
        </button>
      </div>
    </header>

    <!-- Main Content Area -->
    <div class="flex-1 overflow-hidden flex flex-col relative">
      <Transition
        enter-active-class="transition duration-200 ease-out"
        enter-from-class="opacity-0 translate-x-4"
        enter-to-class="opacity-100 translate-x-0"
        leave-active-class="transition duration-150 ease-in"
        leave-from-class="opacity-100 translate-x-0"
        leave-to-class="opacity-0 -translate-x-4"
      >
        <!-- State 1: Playlist Index -->
        <div v-if="viewMode === 'index'" class="absolute inset-0 flex flex-col p-4 space-y-4 overflow-y-auto scrollbar-none">
          <div class="flex rounded-lg border border-border-subtle bg-surface-raised/40 p-1">
            <button type="button" class="flex-1 rounded-md py-1.5 text-[10px] font-bold uppercase tracking-widest" :class="scope === 'room' ? 'bg-primary text-text-inverse' : 'text-text-muted'" @click="switchScope('room')">
              {{ t('roomPlaylists.roomScope') }}
            </button>
            <button type="button" class="flex-1 rounded-md py-1.5 text-[10px] font-bold uppercase tracking-widest" :class="scope === 'personal' ? 'bg-primary text-text-inverse' : 'text-text-muted'" @click="switchScope('personal')">
              {{ t('roomPlaylists.personalScope') }}
            </button>
          </div>
          <!-- Create Playlist Form -->
          <form v-if="scope === 'room' || !userPlaylistsStore.loading" @submit.prevent="create" class="flex gap-2">
            <div class="relative flex-1 group">
              <span class="absolute left-3 top-1/2 -translate-y-1/2 material-symbols-outlined text-[18px] text-text-disabled group-focus-within:text-primary transition-colors">playlist_add</span>
              <input 
                v-model.trim="newName" 
                :placeholder="t('roomPlaylists.newPlaceholder')"
                class="w-full pl-10 pr-4 py-2 bg-surface-raised/50 border border-border-default rounded-xl text-xs text-text-primary placeholder:text-text-disabled outline-none focus:border-primary/50 focus:bg-surface-raised transition-all"
              >
            </div>
            <button 
              type="submit" 
              :disabled="!newName"
              class="px-4 bg-primary text-text-inverse text-[10px] font-bold uppercase tracking-widest rounded-xl disabled:opacity-30 disabled:grayscale transition-all"
            >
              {{ t('rooms.create') }}
            </button>
          </form>

          <!-- Playlist List -->
          <div class="space-y-1">
            <TrackListItem
              v-for="playlist in store.playlists"
              :key="playlist.id"
              :title="playlist.name"
              :artist="`${playlist.trackCount} ${t('queue.title')}`"
              clickable
              @click="select(playlist.id)"
            >
              <template #suffix>
                <span class="material-symbols-outlined text-text-disabled group-hover:text-primary transition-all">chevron_right</span>
              </template>
            </TrackListItem>

            <div v-if="!store.loading && store.playlists.length === 0" class="flex flex-col items-center justify-center py-12 text-center space-y-3 opacity-40">
              <span class="material-symbols-outlined text-[48px]">playlist_add</span>
              <p class="text-xs text-text-muted">{{ t('roomPlaylists.empty') }}</p>
            </div>
          </div>
        </div>

        <!-- State 2: Playlist Detail -->
        <div v-else class="absolute inset-0 flex flex-col overflow-hidden">
          <!-- Import Section (Collapsible or compact) -->
          <div class="px-4 py-3 border-b border-border-subtle bg-surface-panel/20">
            <form @submit.prevent="importExternal" class="flex flex-col gap-3">
              <div class="flex gap-2">
                <select 
                  v-model="importPlatform"
                  class="bg-surface-raised border border-border-default rounded-lg px-2 text-[10px] font-bold uppercase tracking-wider text-text-secondary outline-none focus:border-primary/50 transition-all"
                >
                  <option value="netease">NetEase</option>
                  <option value="bilibili">Bilibili</option>
                </select>
                <div class="relative flex-1 group">
                  <input 
                    v-model.trim="externalPlaylistId" 
                    :placeholder="t('roomPlaylists.externalPlaceholder')"
                    class="w-full px-3 py-1.5 bg-surface-raised/50 border border-border-default rounded-lg text-xs text-text-primary placeholder:text-text-disabled outline-none focus:border-primary/50 focus:bg-surface-raised transition-all"
                  >
                </div>
                <button 
                  type="submit"
                  :disabled="!externalPlaylistId"
                  class="px-3 py-1.5 bg-surface-raised border border-border-default rounded-lg text-[10px] font-bold uppercase tracking-wider text-text-primary hover:bg-primary hover:text-text-inverse hover:border-primary disabled:opacity-30 transition-all"
                >
                  {{ t('search.importPlaylist') }}
                </button>
              </div>
            </form>
            <div class="mt-2 flex justify-end">
              <div class="inline-flex overflow-hidden rounded-md border border-border-default bg-surface-raised">
                <select
                  v-model="exportFormat"
                  class="bg-transparent px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-text-secondary outline-none"
                  :aria-label="t('common.export')"
                >
                  <option v-for="format in exportFormats" :key="format" :value="format">{{ format }}</option>
                </select>
                <button
                  type="button"
                  @click="exportSelected"
                  class="border-l border-border-default px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-text-secondary transition-colors hover:bg-primary hover:text-text-inverse"
                >
                  {{ t('common.export') }}
                </button>
              </div>
            </div>
          </div>

          <!-- Track List -->
          <div class="flex-1 overflow-y-auto p-4 space-y-1 scrollbar-none">
            <TrackListItem
              v-for="track in store.selectedTracks"
              :key="track.id"
              :title="track.music.name"
              :artist="(track.music.artists || []).join(' / ') || track.music.platform"
              :cover-url="track.music.coverUrl"
            >
              <template #suffix>
                <button 
                  @click.stop="deleteTrack(track.id)"
                  class="w-8 h-8 flex items-center justify-center rounded-lg text-text-disabled hover:text-error hover:bg-error/10 transition-all shrink-0"
                  :title="t('queue.remove')"
                >
                  <span class="material-symbols-outlined text-[18px]">delete</span>
                </button>
              </template>
            </TrackListItem>

            <div v-if="store.selectedTracks.length === 0" class="flex flex-col items-center justify-center py-12 text-center space-y-3 opacity-40">
              <span class="material-symbols-outlined text-[48px]">music_off</span>
              <p class="text-xs text-text-muted">{{ t('roomPlaylists.emptyTracks') }}</p>
            </div>
          </div>

          <!-- Detail Footer Actions -->
          <footer class="p-3 border-t border-border-subtle bg-surface-panel/50 flex justify-between gap-3 shrink-0">
            <button 
              v-if="!store.selectedPlaylist?.systemKey"
              @click="rename"
              class="flex-1 py-2 rounded-lg bg-surface-raised border border-border-default text-[10px] font-bold uppercase tracking-widest text-text-secondary hover:text-text-primary hover:border-border-strong transition-all"
            >
              {{ t('roomPlaylists.rename') }}
            </button>
            <button 
              v-if="!store.selectedPlaylist?.systemKey"
              @click="confirmDelete"
              class="flex-1 py-2 rounded-lg bg-error/5 border border-error/20 text-[10px] font-bold uppercase tracking-widest text-error hover:bg-error/10 transition-all"
            >
              {{ t('roomPlaylists.delete') }}
            </button>
          </footer>
        </div>
      </Transition>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoomPlaylistsStore } from '../../stores/roomPlaylists';
import { useUserPlaylistsStore } from '../../stores/userPlaylists';
import TrackListItem from '../ui/TrackListItem.vue';

const roomPlaylistsStore = useRoomPlaylistsStore();
const userPlaylistsStore = useUserPlaylistsStore();
const { t } = useI18n();
const newName = ref('');
const importPlatform = ref('netease');
const externalPlaylistId = ref('');
const viewMode = ref('index'); // 'index' or 'detail'
const scope = ref('room');
const exportFormats = ['txt', 'csv', 'json'];
const exportFormat = ref('txt');
const store = computed(() => scope.value === 'room' ? roomPlaylistsStore : userPlaylistsStore);

const create = async () => {
  if (!newName.value) return;
  await store.value.createPlaylist(newName.value);
  newName.value = '';
};

const select = async (playlistId) => {
  store.value.selectedPlaylistId = playlistId;
  viewMode.value = 'detail';
  await store.value.loadTracks(playlistId);
};

const goBack = () => {
  viewMode.value = 'index';
};

const rename = async () => {
  const nextName = window.prompt(t('roomPlaylists.namePrompt'), store.value.selectedPlaylist?.name || '');
  if (nextName) await store.value.renamePlaylist(store.value.selectedPlaylistId, nextName);
};

const confirmDelete = async () => {
  if (window.confirm(t('roomPlaylists.deleteConfirm') || 'Delete this playlist?')) {
    await store.value.deletePlaylist(store.value.selectedPlaylistId);
    viewMode.value = 'index';
  }
};

const importExternal = async () => {
  if (!externalPlaylistId.value) return;
  if (scope.value === 'room') {
    await roomPlaylistsStore.importExternal(importPlatform.value, externalPlaylistId.value);
  } else {
    await userPlaylistsStore.importPlaylist(userPlaylistsStore.selectedPlaylistId, importPlatform.value, externalPlaylistId.value);
  }
  externalPlaylistId.value = '';
};

const playSelected = () => {
  if (scope.value === 'room') return roomPlaylistsStore.playPlaylist();
  return userPlaylistsStore.enqueue();
};

const deleteTrack = async (trackId) => {
  await store.value.deleteTrack(store.value.selectedPlaylistId, trackId);
};

const exportSelected = async () => {
  const format = exportFormat.value;
  const playlistId = store.value.selectedPlaylistId;
  if (!playlistId) return;
  const content = scope.value === 'room'
    ? await roomPlaylistsStore.exportPlaylist(format)
    : await userPlaylistsStore.exportPlaylist(playlistId, format);
  const blob = new Blob([content], { type: format === 'json' ? 'application/json;charset=utf-8' : 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `musicparty-${scope.value}-playlist-${playlistId}.${format}`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

const switchScope = async (nextScope) => {
  scope.value = nextScope;
  viewMode.value = 'index';
  await store.value.loadPlaylists();
};

onMounted(() => store.value.loadPlaylists());
</script>

<style scoped>
.scrollbar-none::-webkit-scrollbar {
  display: none;
}
.scrollbar-none {
  scrollbar-width: none;
}

/* Fix for transition positioning */
.room-playlists-module > div {
  perspective: 1000px;
}
</style>
