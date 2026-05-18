<template>
  <div class="glass-panel flex h-full min-h-0 w-full flex-col overflow-hidden rounded-lg p-4">
    <div class="mb-3 flex flex-shrink-0 items-center justify-between gap-3 px-1">
      <div v-if="viewMode === 'index'" class="flex items-center rounded-md border border-border-subtle bg-[var(--surface-control)] p-1">
        <button type="button" class="rounded px-3 py-1 font-section-label text-section-label uppercase tracking-widest transition-colors" :class="scope === 'room' ? 'bg-[var(--surface-control-active)] text-primary' : 'text-text-muted hover:text-text-primary'" @click="switchScope('room')">
          {{ t('roomPlaylists.roomScope') }}
        </button>
        <button type="button" class="rounded px-3 py-1 font-section-label text-section-label uppercase tracking-widest transition-colors" :class="scope === 'personal' ? 'bg-[var(--surface-control-active)] text-primary' : 'text-text-muted hover:text-text-primary'" @click="switchScope('personal')">
          {{ t('roomPlaylists.personalScope') }}
        </button>
      </div>

      <div v-else class="flex min-w-0 items-center gap-2">
        <button
          @click="goBack"
          class="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-text-muted hover:bg-[var(--surface-control-hover)] hover:text-text-primary"
          :title="t('common.back')"
        >
          <span class="material-symbols-outlined text-[18px]">arrow_back</span>
        </button>
        <div class="min-w-0">
          <h2 class="truncate text-sm font-semibold text-text-primary">{{ store.selectedPlaylist?.name || 'Loading...' }}</h2>
          <p class="font-micro text-micro text-text-muted uppercase">{{ store.selectedTracks.length }} {{ t('queue.tracks') }}</p>
        </div>
      </div>

      <div class="flex min-w-0 items-center gap-2">
        <span v-if="viewMode === 'index'" class="font-micro text-micro text-text-muted uppercase">{{ store.playlists.length }} {{ t('roomPlaylists.title') }}</span>
        <button
          v-if="viewMode === 'detail' && store.selectedTracks.length > 0"
          @click="playSelected"
          class="flex h-8 w-8 items-center justify-center rounded-md bg-primary text-on-primary transition-colors hover:bg-[var(--accent-hover)]"
          :title="t('roomPlaylists.playAll')"
        >
          <span class="material-symbols-outlined text-[18px]" style="font-variation-settings: 'FILL' 1;">play_arrow</span>
        </button>
      </div>
    </div>

    <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
      <div v-if="viewMode === 'index'" class="flex min-h-0 flex-1 flex-col gap-3">
        <form v-if="scope === 'room' || !userPlaylistsStore.loading" @submit.prevent="create" class="flex flex-shrink-0 gap-2">
          <div class="relative flex-1">
            <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-text-muted">playlist_add</span>
            <input
              v-model.trim="newName"
              :placeholder="t('roomPlaylists.newPlaceholder')"
              class="h-9 w-full rounded-md border border-border-subtle bg-[var(--surface-control)] pl-10 pr-3 text-xs text-text-primary outline-none placeholder:text-text-muted transition-colors focus:border-border-strong focus:bg-[var(--surface-control-active)]"
            >
          </div>
          <button
            type="submit"
            :disabled="!newName"
            class="h-9 rounded-md bg-primary px-3 text-xs font-semibold text-on-primary transition-colors hover:bg-[var(--accent-hover)] disabled:cursor-not-allowed disabled:opacity-40"
          >
            {{ t('rooms.create') }}
          </button>
        </form>

        <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pr-1">
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

            <div v-if="!store.loading && store.playlists.length === 0" class="py-16 text-center">
              <div class="text-sm font-bold text-text-primary">{{ t('roomPlaylists.empty') }}</div>
            </div>
        </div>
      </div>

      <div v-else class="flex min-h-0 flex-1 flex-col gap-3">
        <form @submit.prevent="importExternal" class="flex flex-shrink-0 gap-2 rounded-md border border-border-subtle bg-[var(--surface-control)] p-2">
          <select
            v-model="importPlatform"
            class="h-8 rounded-md border border-border-subtle bg-[var(--surface-control)] px-2 font-micro text-micro uppercase text-text-secondary outline-none focus:border-border-strong"
          >
            <option value="netease">NetEase</option>
            <option value="bilibili">Bilibili</option>
          </select>
          <input
            v-model.trim="externalPlaylistId"
            :placeholder="t('roomPlaylists.externalPlaceholder')"
            class="h-8 min-w-0 flex-1 rounded-md border border-border-subtle bg-transparent px-2 text-xs text-text-primary outline-none placeholder:text-text-muted focus:border-border-strong"
          >
          <button
            type="submit"
            :disabled="!externalPlaylistId"
            class="h-8 rounded-md px-2 text-xs font-semibold text-text-secondary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-text-primary disabled:cursor-not-allowed disabled:opacity-40"
          >
            {{ t('search.importPlaylist') }}
          </button>
        </form>

        <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pr-1">
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

            <div v-if="store.selectedTracks.length === 0" class="py-16 text-center">
              <div class="text-sm font-bold text-text-primary">{{ t('roomPlaylists.emptyTracks') }}</div>
            </div>
        </div>

        <footer class="flex flex-shrink-0 items-center justify-between gap-2 rounded-md border border-border-subtle bg-[var(--surface-control)] p-2">
          <div class="inline-flex overflow-hidden rounded-md border border-border-subtle">
            <select
              v-model="exportFormat"
              class="h-8 bg-transparent px-2 font-micro text-micro uppercase text-text-secondary outline-none"
              :aria-label="t('common.export')"
            >
              <option v-for="format in exportFormats" :key="format" :value="format">{{ format }}</option>
            </select>
            <button
              type="button"
              @click="exportSelected"
              class="h-8 border-l border-border-subtle px-2 text-xs font-semibold text-text-secondary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-text-primary"
            >
              {{ t('common.export') }}
            </button>
          </div>
          <div v-if="!store.selectedPlaylist?.systemKey" class="flex items-center gap-2">
            <button
              @click="rename"
              class="h-8 rounded-md px-2 text-xs font-semibold text-text-secondary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-text-primary"
            >
              {{ t('roomPlaylists.rename') }}
            </button>
            <button
              @click="confirmDelete"
              class="h-8 rounded-md px-2 text-xs font-semibold text-error transition-colors hover:bg-[var(--error-soft-bg)]"
            >
              {{ t('roomPlaylists.delete') }}
            </button>
          </div>
        </footer>
      </div>
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
