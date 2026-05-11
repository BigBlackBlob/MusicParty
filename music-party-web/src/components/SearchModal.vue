<template>
  <div v-if="isOpen" class="fixed inset-0 z-[60] flex items-center justify-center bg-black/55 p-4 backdrop-blur-md">
    <div class="flex h-[85vh] w-full max-w-5xl flex-col overflow-hidden rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] shadow-2xl md:h-[80vh]">
      <button @click="emit('close')" class="absolute right-4 top-4 z-50 rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] p-2 text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)]">
        <X class="w-5 h-5" />
      </button>

      <div class="flex-shrink-0 border-b border-[var(--border-default)] bg-[var(--surface-1)] p-4 md:p-6">
        <div class="mb-4 flex items-center gap-2 text-xs font-semibold tracking-[0.18em] text-[var(--text-tertiary)]">
          <Search class="w-4 h-4 text-[var(--accent)]" />
          SEARCH
        </div>

        <div class="mb-4 flex gap-2">
          <button
              v-for="p in ['netease', 'bilibili']"
              :key="p"
              @click="platform = p"
              class="rounded-full border px-4 py-2 text-sm font-semibold transition-colors"
              :class="platform === p
                ? 'border-[var(--accent)] bg-[var(--accent)] text-[var(--text-inverse)]'
                : 'border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-secondary)] hover:bg-[var(--surface-3)]'"
          >
            {{ p }}
          </button>
        </div>

        <div class="flex gap-2">
          <input
              v-model="keyword"
              @keyup.enter="doSearch"
              :placeholder="isAdminMode ? '!!! ENTER ADMIN PASSWORD !!!' : '搜索音乐...'"
              class="min-w-0 flex-1 rounded-2xl border px-4 py-3 text-sm outline-none transition-colors placeholder:text-[var(--text-tertiary)]"
              :class="isAdminMode
                ? 'border-[var(--error)] bg-[var(--error-soft-bg)] text-[var(--error-soft-text)] focus:border-[var(--error)]'
                : 'border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-primary)] focus:border-[var(--border-accent)]'"
          />
          <button
              @click="handleSearchAction"
              class="flex-shrink-0 rounded-2xl px-4 py-3 text-sm font-semibold transition-colors"
              :class="isAdminMode ? 'bg-[var(--error)] text-[var(--text-inverse)] hover:opacity-90' : 'bg-[var(--accent)] text-[var(--text-inverse)] hover:bg-[var(--accent-hover)]'"
          >
            {{ isAdminMode ? 'UNLOCK' : 'SEARCH' }}
          </button>
        </div>
      </div>

      <div class="relative flex flex-1 overflow-hidden">
        <div
            class="flex-shrink-0 flex-col border-b border-[var(--border-default)] bg-[var(--surface-1)] md:flex md:w-1/3 md:border-b-0 md:border-r"
            :class="mobileView === 'playlists' ? 'flex w-full h-full' : 'hidden md:flex'"
        >
          <div class="flex items-center justify-between border-b border-[var(--border-default)] px-3 py-2 text-xs font-semibold tracking-[0.14em] text-[var(--text-tertiary)]">
            <span>用户歌单</span>
          </div>

          <div class="flex-1 space-y-3 overflow-y-auto p-3">
            <div v-if="!bindings[platform]" class="rounded-2xl border border-dashed border-[var(--border-default)] bg-[var(--surface-2)] p-4">
              <div class="mb-3 text-center text-xs text-[var(--text-tertiary)]">绑定用户以获取用户歌单</div>
              <div class="flex gap-2">
                <input
                    v-model="searchUserKeyword"
                    @keyup.enter="searchUser"
                    placeholder="搜索用户名"
                    class="min-w-0 flex-1 rounded-xl border border-[var(--border-default)] bg-[var(--surface-4)] px-3 py-2 text-sm outline-none transition-colors placeholder:text-[var(--text-tertiary)] focus:border-[var(--border-accent)]"
                />
                <button @click="searchUser" class="rounded-xl bg-[var(--accent)] px-3 text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)]">
                  <Search class="w-4 h-4" />
                </button>
              </div>

              <div v-if="userSearchResults.length > 0" class="mt-3 space-y-1 border-t border-[var(--border-default)] pt-3">
                <button
                    v-for="user in userSearchResults"
                    :key="user.id"
                    @click="bindUser(user)"
                    class="flex w-full items-center gap-2 rounded-xl px-2 py-2 text-left transition-colors hover:bg-[var(--surface-3)]"
                >
                  <img :src="user.avatarUrl" class="h-7 w-7 rounded-full bg-[var(--surface-3)]" />
                  <span class="min-w-0 flex-1 truncate text-sm font-semibold text-[var(--text-primary)]">{{ user.name }}</span>
                </button>
              </div>
              <div v-if="isSearchingUser" class="py-3 text-center">
                <Loader2 class="mx-auto h-4 w-4 animate-spin text-[var(--accent)]" />
              </div>
            </div>

            <template v-else>
              <div class="flex items-center justify-between rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2">
                <span class="text-[10px] font-mono text-[var(--text-tertiary)]">ID: {{ bindings[platform] }}</span>
                <button @click="playerStore.bindAccount(platform, '')" class="text-[10px] font-semibold text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)]">
                  UNLINK
                </button>
              </div>

              <div v-if="isPlaylistsLoading" class="flex justify-center py-8">
                <Loader2 class="h-6 w-6 animate-spin text-[var(--accent)]" />
              </div>

              <template v-else>
                <button
                    v-for="pl in playlists"
                    :key="pl.id"
                    @click="handleSelectPlaylist(pl.id)"
                    class="flex w-full items-center gap-3 rounded-2xl border border-transparent px-3 py-2 text-left transition-colors hover:border-[var(--border-default)] hover:bg-[var(--surface-3)]"
                >
                  <div class="h-11 w-11 flex-shrink-0 overflow-hidden rounded-xl bg-[var(--surface-3)]">
                    <CoverImage :src="pl.coverImgUrl" class="h-full w-full" />
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ pl.name }}</div>
                    <div class="text-xs text-[var(--text-tertiary)]">{{ pl.trackCount }} TRACKS</div>
                  </div>
                  <ChevronRight class="ml-auto h-4 w-4 text-[var(--text-tertiary)] md:hidden" />
                </button>
              </template>
            </template>
          </div>
        </div>

        <div class="min-h-0 flex-col bg-[var(--surface-2)] md:flex md:flex-1" :class="mobileView === 'songs' ? 'flex w-full h-full' : 'hidden md:flex'">
          <div class="flex-shrink-0 border-b border-[var(--border-default)] bg-[var(--surface-1)] px-4 py-3 md:hidden">
            <button @click="mobileView = 'playlists'" class="mb-2 inline-flex items-center gap-2 text-xs font-semibold text-[var(--text-tertiary)]">
              <ArrowLeft class="w-4 h-4" />
              返回
            </button>
            <div class="text-sm font-semibold text-[var(--text-primary)]">
              {{ listMode === 'search' ? 'SEARCH RESULTS' : 'PLAYLIST DETAILS' }}
            </div>
          </div>

          <div @scroll="handleScroll" class="flex-1 overflow-y-auto p-3 md:p-4">
            <div v-if="loading" class="py-10 text-center text-xs font-mono text-[var(--text-tertiary)]">
              > LOADING DATA STREAM...
            </div>

            <div v-else-if="currentPlaylistId && listMode === 'playlist'" class="mb-4 flex items-center justify-between rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-4 shadow-sm">
              <div class="min-w-0">
                <div class="text-xs text-[var(--text-tertiary)]">用户歌单</div>
                <div class="truncate text-lg font-semibold text-[var(--text-primary)]">{{ currentPlaylistId }}</div>
                <div class="text-xs font-mono text-[var(--text-tertiary)]">{{ songs.length }} LOADED</div>
              </div>
              <button @click="handleImportPlaylist" class="inline-flex items-center gap-2 rounded-2xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)]">
                <ListPlus class="w-4 h-4" />
                <span class="hidden sm:inline">导入全部</span>
              </button>
            </div>

            <div class="space-y-2">
              <div v-if="songs.length === 0 && !loading" class="py-10 text-center text-xs text-[var(--text-tertiary)]">NO DATA FOUND</div>
              <div
                  v-for="song in songs"
                  :key="song.id"
                  class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-2xl border px-3 py-2 transition-colors"
                  :class="isUnplayable(song) ? 'cursor-not-allowed border-transparent bg-[var(--surface-3)]/60 opacity-50 grayscale' : 'border-[var(--border-default)] bg-[var(--surface-4)] hover:bg-[var(--surface-3)]'"
              >
                <div class="h-10 w-10 flex-shrink-0 overflow-hidden rounded-xl bg-[var(--surface-3)]">
                  <CoverImage :src="song.coverUrl" class="h-full w-full" />
                </div>
                <div class="min-w-0">
                  <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ song.name }}</div>
                  <div class="truncate text-xs text-[var(--text-secondary)]">{{ song.artists.join(' / ') }}</div>
                </div>

                <div v-if="isUnplayable(song)" class="flex h-9 min-w-[3.75rem] items-center justify-center rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2 py-0.5 text-[10px] font-mono font-semibold text-[var(--text-tertiary)]">
                  &gt;10MIN
                </div>

                <button
                    v-else
                    @click="handleAddClick(song)"
                    class="flex h-9 w-9 items-center justify-center self-center rounded-full transition-colors"
                    :class="[
                        isInQueue(song.id) ? 'cursor-default text-[var(--success)]' :
                        pendingIds.has(song.id) ? 'cursor-wait text-[var(--accent)]' :
                        'text-[var(--text-tertiary)] hover:text-[var(--accent)]'
                    ]"
                    :disabled="pendingIds.has(song.id) || isInQueue(song.id)"
                >
                  <Loader2 v-if="pendingIds.has(song.id)" class="h-5 w-5 animate-spin" />
                  <Check v-else-if="isInQueue(song.id)" class="h-5 w-5" />
                  <PlusCircle v-else class="h-5 w-5" />
                </button>
              </div>
            </div>

            <div v-if="listMode === 'playlist' && !loading" class="py-4 text-center">
              <div v-if="isLoadingMore" class="flex justify-center text-[var(--accent)]">
                <Loader2 class="h-6 w-6 animate-spin" />
              </div>
              <div v-else-if="!hasMore && songs.length > 0" class="text-xs font-mono text-[var(--text-tertiary)]">-- END OF PLAYLIST --</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue';
import { usePlayerStore } from '../stores/player';
import { useSearchLogic } from '../composables/useSearchLogic';
import { usePlaylistLogic } from '../composables/usePlaylistLogic';
import { X, Search, PlusCircle, ListPlus, Loader2, ArrowLeft, ChevronRight, Check } from 'lucide-vue-next';
import CoverImage from './CoverImage.vue';

const props = defineProps(['isOpen']);
const emit = defineEmits(['close']);
const playerStore = usePlayerStore();

const { platform, keyword, songs, loading, listMode, isAdminMode, doSearch } = useSearchLogic(emit);

const handleSearchAction = async () => {
  await doSearch();
  if (!isAdminMode.value) {
    mobileView.value = 'songs';
  }
};

const {
  playlists, currentPlaylistId, searchUserKeyword, userSearchResults,
  isSearchingUser, isPlaylistsLoading, hasMore, isLoadingMore, bindings,
  searchUser, bindUser, loadPlaylist, handleScroll
} = usePlaylistLogic(platform, songs, listMode, loading);

const mobileView = ref('playlists');
const MAX_DURATION = 10 * 60 * 1000;

const isUnplayable = (song) => platform.value === 'bilibili' && song.duration > MAX_DURATION;

const handleSelectPlaylist = (pid) => {
  loadPlaylist(pid);
  mobileView.value = 'songs';
};

const handleImportPlaylist = () => {
  playerStore.enqueuePlaylist(platform.value, currentPlaylistId.value);
  emit('close');
};

const pendingIds = ref(new Set());

const isInQueue = (songId) => playerStore.queue.some(item => item.music.id === songId);

const handleAddClick = (song) => {
  if (pendingIds.value.has(song.id) || isInQueue(song.id)) return;
  pendingIds.value.add(song.id);
  playerStore.enqueue(platform.value, song.id);
  setTimeout(() => {
    pendingIds.value.delete(song.id);
  }, 2000);
};

watch(listMode, (mode) => {
  if (mode === 'search') mobileView.value = 'songs';
});

watch(() => props.isOpen, (val) => {
  if (val) mobileView.value = 'playlists';
});
</script>
