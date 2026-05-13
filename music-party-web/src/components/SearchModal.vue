<template>
  <div v-if="isOpen" class="fixed inset-0 z-[var(--z-overlay)] flex items-center justify-center bg-black/55 p-4 backdrop-blur-md" @click.self="emit('close')" @keydown.escape="emit('close')" tabindex="-1">
    <div class="flex h-[85vh] w-full max-w-5xl flex-col overflow-hidden rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] shadow-2xl md:h-[80vh]">
      <div class="flex-shrink-0 border-b border-[var(--border-default)] bg-[var(--surface-1)] p-4 md:p-6">
        <div class="mb-4 flex items-center justify-between gap-3">
          <div class="flex items-center gap-2 text-xs font-semibold tracking-[0.18em] text-[var(--text-tertiary)]">
            <Search class="w-4 h-4 text-[var(--accent)]" />
            搜索
          </div>
          <button @click="emit('close')" class="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] p-2 text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)] active:scale-[0.96]" aria-label="关闭搜索">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="mb-3 flex flex-wrap gap-2">
          <button
              v-for="p in platforms"
              :key="p.id"
              @click="platform = p.id"
              class="min-h-[44px] rounded-full border px-4 py-2 text-sm font-semibold transition-colors active:scale-[0.96]"
              :class="platform === p.id
                ? 'border-[var(--accent)] bg-[var(--accent)] text-[var(--text-inverse)]'
                : 'border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-secondary)] hover:bg-[var(--surface-3)]'"
          >
            {{ p.label }}
          </button>
        </div>

        <div v-if="supportsAlbumSearch" class="mb-4 flex gap-2 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)] p-1">
          <button
              type="button"
              @click="searchType = 'song'"
              class="min-h-[40px] flex-1 rounded-xl px-3 text-sm font-semibold transition-colors active:scale-[0.98]"
              :class="searchType === 'song' ? 'bg-[var(--surface-4)] text-[var(--text-primary)] shadow-sm' : 'text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'"
          >
            歌曲
          </button>
          <button
              type="button"
              @click="searchType = 'album'"
              class="min-h-[40px] flex-1 rounded-xl px-3 text-sm font-semibold transition-colors active:scale-[0.98]"
              :class="searchType === 'album' ? 'bg-[var(--surface-4)] text-[var(--text-primary)] shadow-sm' : 'text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'"
          >
            专辑
          </button>
        </div>

        <div class="flex gap-2">
          <label for="music-search-input" class="sr-only">{{ isAdminMode ? '管理员密码' : '搜索音乐' }}</label>
          <input
              id="music-search-input"
              v-model="keyword"
              @keyup.enter="doSearch"
              :aria-label="isAdminMode ? '管理员密码' : '搜索音乐'"
              :placeholder="isAdminMode ? '输入管理员密码' : '搜索音乐...'"
              class="min-w-0 flex-1 rounded-2xl border px-4 py-3 text-sm outline-none transition-colors placeholder:text-[var(--text-tertiary)] focus:ring-2 focus:ring-[var(--accent-muted)]"
              :class="isAdminMode
                ? 'border-[var(--error)] bg-[var(--error-soft-bg)] text-[var(--error-soft-text)] focus:border-[var(--error)]'
                : 'border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-primary)] focus:border-[var(--border-accent)]'"
          />
          <button
              @click="handleSearchAction"
              class="flex-shrink-0 min-h-[44px] rounded-2xl px-4 py-3 text-sm font-semibold transition-colors active:scale-[0.96]"
              :class="isAdminMode ? 'bg-[var(--error)] text-[var(--text-inverse)] hover:opacity-90' : 'bg-[var(--accent)] text-[var(--text-inverse)] hover:bg-[var(--accent-hover)]'"
          >
            {{ isAdminMode ? '解锁' : '搜索' }}
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
              <div class="mb-3 text-center text-xs text-[var(--text-tertiary)]">绑定用户后可查看歌单</div>
              <div class="flex gap-2">
                <label for="playlist-user-search-input" class="sr-only">搜索歌单用户</label>
                <input
                    id="playlist-user-search-input"
                    v-model="searchUserKeyword"
                    @keyup.enter="searchUser"
                    aria-label="搜索歌单用户"
                    placeholder="搜索用户名"
                    class="min-w-0 flex-1 rounded-xl border border-[var(--border-default)] bg-[var(--surface-4)] px-3 py-2 text-sm outline-none transition-colors placeholder:text-[var(--text-tertiary)] focus:border-[var(--border-accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
                />
                <button @click="searchUser" class="min-w-[44px] min-h-[44px] rounded-xl bg-[var(--accent)] px-3 text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)] active:scale-[0.96]" aria-label="搜索用户">
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
                  <img :src="user.avatarUrl" :alt="`${user.name} 头像`" loading="lazy" decoding="async" class="h-7 w-7 rounded-full bg-[var(--surface-3)]" />
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
                <button @click="playerStore.bindAccount(platform, '')" class="min-h-[44px] text-[10px] font-semibold text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)] active:scale-[0.96]" aria-label="解除绑定">
                  解除绑定
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
                    <CoverImage :src="pl.coverImgUrl" :alt="`${pl.name} 封面`" loading="lazy" class="h-full w-full" />
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ pl.name }}</div>
                    <div class="text-xs text-[var(--text-tertiary)]">{{ pl.trackCount }} 首歌曲</div>
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
              {{ getMobileTitle() }}
            </div>
          </div>

          <div @scroll="handleScroll" class="flex-1 overflow-y-auto p-3 md:p-4">
            <div v-if="loading" class="py-10 text-center text-xs font-mono text-[var(--text-tertiary)]">
              正在加载...
            </div>

            <div v-else-if="currentPlaylistId && listMode === 'playlist'" class="mb-4 flex items-center justify-between rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-4 shadow-sm">
              <div class="min-w-0">
                <div class="text-xs text-[var(--text-tertiary)]">{{ listMode === 'album' ? '网易云专辑' : '用户歌单' }}</div>
                <div class="truncate text-lg font-semibold text-[var(--text-primary)]">{{ currentPlaylistId }}</div>
                <div class="text-xs font-mono text-[var(--text-tertiary)]">{{ songs.length }} 首歌曲</div>
              </div>
              <button @click="handleImportCurrentList" class="inline-flex min-h-[44px] items-center gap-2 rounded-2xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)] active:scale-[0.96]">
                <ListPlus class="w-4 h-4" />
                <span class="hidden sm:inline">导入全部</span>
              </button>
            </div>

            <div v-else-if="listMode === 'albumSearch'" class="space-y-2">
              <div v-if="albums.length === 0 && !loading" class="rounded-2xl border border-dashed border-[var(--border-default)] bg-[var(--surface-4)] px-4 py-10 text-center">
                <div class="text-sm font-semibold text-[var(--text-primary)]">未找到相关专辑</div>
                <div class="mt-1 text-xs text-[var(--text-tertiary)]">换个专辑名或歌手名再试试。</div>
              </div>
              <div
                  v-for="album in albums"
                  :key="album.id"
                  class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] px-3 py-2 transition-colors hover:bg-[var(--surface-3)]"
              >
                <button @click="handleSelectAlbum(album)" class="h-12 w-12 flex-shrink-0 overflow-hidden rounded-xl bg-[var(--surface-3)] text-left" :aria-label="`查看 ${album.name} 曲目`">
                  <CoverImage :src="album.coverUrl" :alt="`${album.name} 封面`" loading="lazy" class="h-full w-full" />
                </button>
                <button @click="handleSelectAlbum(album)" class="min-w-0 text-left" :aria-label="`查看 ${album.name} 曲目`">
                  <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ album.name }}</div>
                  <div class="truncate text-xs text-[var(--text-secondary)]">{{ album.artistName || '未知艺术家' }}</div>
                  <div class="text-xs text-[var(--text-tertiary)]">{{ album.trackCount }} 首歌曲</div>
                </button>

                <button
                    @click="handleImportAlbum(album)"
                    class="inline-flex min-h-[40px] items-center justify-center rounded-full bg-[var(--accent)] px-3 text-xs font-semibold text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)] active:scale-[0.96]"
                    :aria-label="`导入专辑 ${album.name}`"
                >
                  导入
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <div v-if="listMode !== 'albumSearch' && songs.length === 0 && !loading" class="rounded-2xl border border-dashed border-[var(--border-default)] bg-[var(--surface-4)] px-4 py-10 text-center">
                <div class="text-sm font-semibold text-[var(--text-primary)]">未找到相关歌曲</div>
                <div class="mt-1 text-xs text-[var(--text-tertiary)]">换个关键词、歌手名或平台再试试。</div>
              </div>
              <div
                  v-for="song in songs"
                  :key="song.id"
                  class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-2xl border px-3 py-2 transition-colors"
                  :class="isUnplayable(song) ? 'cursor-not-allowed border-transparent bg-[var(--surface-3)]/60 opacity-50 grayscale' : 'border-[var(--border-default)] bg-[var(--surface-4)] hover:bg-[var(--surface-3)]'"
              >
                <div class="h-10 w-10 flex-shrink-0 overflow-hidden rounded-xl bg-[var(--surface-3)]">
                  <CoverImage :src="song.coverUrl" :alt="`${song.name} 封面`" loading="lazy" class="h-full w-full" />
                </div>
                <div class="min-w-0">
                  <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ song.name }}</div>
                  <div class="truncate text-xs text-[var(--text-secondary)]">{{ song.artists.join(' / ') }}</div>
                </div>

                <div v-if="isUnplayable(song)" class="flex h-9 min-w-[3.75rem] items-center justify-center rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2 py-0.5 text-xs font-semibold text-[var(--text-tertiary)]" aria-label="超过时长限制，不可播放">
                  不可播放
                </div>

                <button
                    v-else
                    @click="handleAddClick(song)"
                    class="flex h-9 w-9 items-center justify-center self-center rounded-full transition-colors active:scale-[0.96]"
                    :class="[
                        isInQueue(song.id) ? 'cursor-default text-[var(--success)]' :
                        pendingIds.has(song.id) ? 'cursor-wait text-[var(--accent)]' :
                        'text-[var(--text-tertiary)] hover:text-[var(--accent)]'
                    ]"
                    :disabled="pendingIds.has(song.id) || isInQueue(song.id)"
                    :aria-label="isInQueue(song.id) ? '已在队列中' : pendingIds.has(song.id) ? '正在添加到队列' : `添加 ${song.name} 到队列`"
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
              <div v-else-if="!hasMore && songs.length > 0" class="text-xs font-mono text-[var(--text-tertiary)]">歌单已到底</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { usePlayerStore } from '../stores/player';
import { useSearchLogic } from '../composables/useSearchLogic';
import { usePlaylistLogic } from '../composables/usePlaylistLogic';
import { X, Search, PlusCircle, ListPlus, Loader2, ArrowLeft, ChevronRight, Check } from 'lucide-vue-next';
import CoverImage from './CoverImage.vue';
import { useWindowSize } from '@vueuse/core';

const props = defineProps(['isOpen']);
const emit = defineEmits(['close']);
const playerStore = usePlayerStore();
const { width } = useWindowSize();
const isMobile = computed(() => width.value < 768);

const { platform, platforms, supportsAlbumSearch, loadPlatforms, keyword, songs, albums, loading, listMode, searchType, isAdminMode, doSearch } = useSearchLogic(emit);

const handleSearchAction = async () => {
  await doSearch();
  if (!isAdminMode.value && isMobile.value) {
    mobileView.value = 'songs';
  }
};

const {
  playlists, currentPlaylistId, searchUserKeyword, userSearchResults,
  isSearchingUser, isPlaylistsLoading, hasMore, isLoadingMore, bindings,
  searchUser, bindUser, loadPlaylist, handleScroll
} = usePlaylistLogic(platform, songs, listMode, loading);

const mobileView = ref('playlists');
const currentAlbumId = ref(null);
const MAX_DURATION = 10 * 60 * 1000;

const isUnplayable = (song) => platform.value === 'bilibili' && song.duration > MAX_DURATION;

const handleSelectPlaylist = (pid) => {
  loadPlaylist(pid);
  if (isMobile.value) mobileView.value = 'songs';
};

const handleSelectAlbum = async (album) => {
  listMode.value = 'album';
  currentPlaylistId.value = album.name;
  currentAlbumId.value = album.id;
  songs.value = [];
  loading.value = true;
  if (isMobile.value) mobileView.value = 'songs';
  try {
    songs.value = await musicApi.getNeteaseAlbumSongs(album.id);
  } finally {
    loading.value = false;
  }
};

const handleImportPlaylist = () => {
  playerStore.enqueuePlaylist(platform.value, currentPlaylistId.value);
  emit('close');
};

const handleImportAlbum = (album) => {
  playerStore.enqueueAlbum('netease', album.id);
  emit('close');
};

const handleImportCurrentList = () => {
  if (listMode.value === 'album') {
    if (currentAlbumId.value) {
      playerStore.enqueueAlbum('netease', currentAlbumId.value);
      emit('close');
    }
    return;
  }
  handleImportPlaylist();
};

const getMobileTitle = () => {
  if (listMode.value === 'albumSearch') return '专辑结果';
  if (listMode.value === 'album') return '专辑详情';
  return listMode.value === 'search' ? '搜索结果' : '歌单详情';
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
  if (isMobile.value && (mode === 'search' || mode === 'albumSearch')) mobileView.value = 'songs';
  if (mode !== 'album') currentAlbumId.value = null;
});

watch(() => props.isOpen, (val) => {
  if (!val) return;
  loadPlatforms();
  if (isMobile.value) mobileView.value = 'playlists';
});

watch(platform, (nextPlatform) => {
  if (!supportsAlbumSearch.value) {
    searchType.value = 'song';
  }
});
</script>
