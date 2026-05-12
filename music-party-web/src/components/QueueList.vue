<template>
  <div id="tutorial-queue" class="flex h-full flex-col bg-[var(--surface-1)]">
    <div class="border-b border-[var(--border-default)] px-4 py-3">
      <div class="flex items-center justify-between gap-3">
        <h3 class="text-sm font-semibold text-[var(--text-primary)]">{{ activeView === 'queue' ? '播放队列' : '喜欢的歌' }}</h3>
        <button
          v-if="activeView === 'liked'"
          class="inline-flex min-h-9 items-center gap-2 rounded-full bg-[var(--accent)] px-3 text-xs font-bold text-[var(--text-inverse)] disabled:cursor-not-allowed disabled:opacity-45"
          :disabled="player.likedSongs.length === 0"
          @click="exportLikedSongs"
          aria-label="导出喜欢的歌"
        >
          <Download class="h-4 w-4" />
          导出
        </button>
        <div v-else-if="selectionMode" class="flex items-center gap-1">
          <span class="rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2.5 py-1 text-xs font-semibold text-[var(--text-secondary)]">已选 {{ selectedCount }}</span>
          <button class="bulk-action" @click="selectAll" :disabled="queue.length === 0">全选</button>
          <button class="bulk-action" @click="topSelected" :disabled="!hasSelection">置顶</button>
          <button class="bulk-action bulk-action--danger" @click="requestDeleteSelected" :disabled="!hasSelection">删除</button>
          <button class="bulk-action" @click="cancelSelection">取消</button>
        </div>
        <div v-else class="flex items-center gap-2">
          <button
            v-if="!userStore.isGuest && queue.length > 0"
            class="inline-flex min-h-8 items-center rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-3 text-xs font-bold text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            @click="selectionMode = true"
          >
            选择
          </button>
          <div class="rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2.5 py-0.5 text-xs font-mono text-[var(--text-secondary)]">
            {{ queue.length }}
          </div>
        </div>
      </div>

      <div class="mt-3 grid grid-cols-2 rounded-2xl bg-[var(--surface-3)] p-1">
        <button
          class="queue-view-toggle"
          :class="activeView === 'queue' ? 'queue-view-toggle--active' : ''"
          @click="activeView = 'queue'"
        >
          队列
        </button>
        <button
          class="queue-view-toggle"
          :class="activeView === 'liked' ? 'queue-view-toggle--active' : ''"
          @click="activeView = 'liked'"
        >
          喜欢
        </button>
      </div>
      <div v-if="selectionMode && pendingDelete" class="mt-3 rounded-2xl border border-[var(--error)]/35 bg-[var(--surface-3)] p-3">
        <div class="mb-2 text-xs font-semibold text-[var(--text-primary)]">删除已选 {{ selectedCount }} 首？</div>
        <div class="flex gap-2">
          <button class="confirm-delete" @click="confirmDeleteSelected">确认删除</button>
          <button class="cancel-delete" @click="pendingDelete = false">取消</button>
        </div>
      </div>
    </div>

    <div v-if="activeView === 'liked'" class="flex-1 overflow-y-auto p-3">
      <div v-if="player.likedSongs.length === 0" class="flex h-full flex-col items-center justify-center px-4 py-10 text-center">
        <div class="mb-2 flex h-12 w-12 items-center justify-center rounded-2xl border border-dashed border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-tertiary)]">
          <Heart class="h-5 w-5" />
        </div>
        <div class="text-sm font-semibold text-[var(--text-primary)]">还没有喜欢的歌</div>
        <div class="mt-1 max-w-[13rem] text-xs leading-relaxed text-[var(--text-tertiary)]">点击底部播放栏的红心，会把当前歌曲保存到这里。</div>
      </div>

      <div v-else class="space-y-2">
        <div v-for="song in player.likedSongs" :key="song.key" class="flex min-h-[4.25rem] items-center gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2">
          <div class="h-11 w-11 flex-shrink-0 overflow-hidden rounded-xl bg-[var(--surface-3)]">
            <CoverImage :src="song.coverUrl" :alt="`${song.name} 封面`" loading="lazy" class="h-full w-full" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ song.name }}</div>
            <div class="truncate text-xs text-[var(--text-secondary)]">{{ song.artists.join(' / ') }}</div>
            <div class="mt-0.5 truncate text-[10px] uppercase tracking-wide text-[var(--text-tertiary)]">{{ song.platform }}</div>
          </div>
          <button class="liked-remove" @click="player.removeLikedSong(song.key)" aria-label="从喜欢的歌移除">
            <Trash2 class="h-4 w-4" />
          </button>
        </div>
      </div>
    </div>

    <div v-else-if="queue.length === 0" class="flex flex-1 flex-col items-center justify-center px-4 py-10 text-center">
      <div class="mb-2 flex h-12 w-12 items-center justify-center rounded-2xl border border-dashed border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-tertiary)]">
        <Music2 class="h-5 w-5" />
      </div>
      <div class="text-sm font-semibold text-[var(--text-primary)]">播放队列为空</div>
      <div class="mt-1 max-w-[12rem] text-xs leading-relaxed text-[var(--text-tertiary)]">打开搜索，添加第一首适合当前房间的歌。</div>
    </div>

    <div v-else-if="player.isShuffle" class="flex-1 overflow-y-auto p-3">
      <div class="mb-4 flex items-center gap-2 rounded-2xl border border-[var(--border-default)] bg-[var(--accent-subtle)] px-3 py-2 text-xs font-semibold text-[var(--accent)]">
        <Shuffle class="w-4 h-4" />
        <span>随机播放中</span>
      </div>

      <div v-if="topItems.length > 0" class="mb-4">
        <div class="mb-2 pl-1 text-xs font-mono text-[var(--text-tertiary)]">优先播放</div>
        <QueueItem
            v-for="(item, idx) in topItems"
            :key="item.queueId"
            :item="item"
            :index="idx"
            :selection-mode="selectionMode"
            :selected="isSelected(item.queueId)"
            @toggle-select="toggleSelected(item.queueId)"
        />
      </div>

      <div class="space-y-2">
        <div v-for="group in userGroups" :key="group.token" class="overflow-hidden rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)]">
          <button
              @click="toggleUser(group.token)"
              class="flex w-full items-center justify-between px-4 py-3 text-left transition-colors hover:bg-[var(--surface-3)]"
          >
            <div class="flex items-center gap-3">
              <div class="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--surface-3)] text-[var(--text-secondary)]">
                <User class="w-4 h-4" />
              </div>
              <div class="text-left">
                <div class="text-sm font-semibold text-[var(--text-primary)]">
                  {{ userStore.resolveName(group.token, group.name) }}
                </div>
                <div class="font-mono text-[10px] text-[var(--text-tertiary)]">
                  {{ group.items.length }} 首歌曲
                </div>
              </div>
            </div>
            <component :is="expandedUsers[group.token] ? ChevronDown : ChevronRight" class="w-4 h-4 text-[var(--text-tertiary)]" />
          </button>

          <div v-show="expandedUsers[group.token]" class="border-t border-[var(--border-default)] bg-[var(--surface-1)] p-2">
            <QueueItem
                v-for="(item, idx) in group.items"
                :key="item.queueId"
                :item="item"
                :selection-mode="selectionMode"
                :selected="isSelected(item.queueId)"
                @toggle-select="toggleSelected(item.queueId)"
            />
          </div>
        </div>
      </div>
    </div>

    <div v-else v-bind="containerProps" class="flex-1 overflow-y-auto p-3">
      <div v-bind="wrapperProps" class="space-y-0">
        <QueueItem
            v-for="{ data: item, index } in list"
            :key="item.queueId"
            :item="item"
            :index="index"
            :selection-mode="selectionMode"
            :selected="isSelected(item.queueId)"
            @toggle-select="toggleSelected(item.queueId)"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useVirtualList } from '@vueuse/core';
import { usePlayerStore } from '../stores/player';
import { Shuffle, ChevronDown, ChevronRight, User, Music2, Heart, Download, Trash2 } from 'lucide-vue-next';
import { useUserStore } from '../stores/user';
import QueueItem from './QueueItem.vue';
import CoverImage from './CoverImage.vue';
import { createLikedSongsFilename, createLikedSongsText } from '../utils/likedSongs';
import { useQueueSelection } from '../composables/useQueueSelection';

const player = usePlayerStore();
const queue = computed(() => player.queue);
const userStore = useUserStore();
const activeView = ref('queue');
const pendingDelete = ref(false);
const {
  selectionMode,
  selectedCount,
  selectedIds,
  hasSelection,
  exitSelectionMode,
  toggleSelected,
  isSelected,
  selectAll
} = useQueueSelection(queue);

// Virtual List Setup (used for normal mode)
const { list, containerProps, wrapperProps } = useVirtualList(queue, {
  itemHeight: 64,
  overscan: 10,
});

// --- Shuffle Mode Logic ---

const topItems = computed(() => {
  return queue.value.filter(item => item.queueId.startsWith('TOP-'));
});

const userGroups = computed(() => {
  const normalItems = queue.value.filter(item => !item.queueId.startsWith('TOP-'));
  const groupsMap = new Map();

  normalItems.forEach(item => {
    const token = item.enqueuedBy.token;
    if (!groupsMap.has(token)) {
      groupsMap.set(token, {
        token: token,
        name: item.enqueuedBy.name,
        items: []
      });
    }
    groupsMap.get(token).items.push(item);
  });

  // 对每个组内的歌曲进行排序：个人置顶 (USERTOP-) 放在最前面
  return Array.from(groupsMap.values()).map(group => ({
    ...group,
    items: [...group.items].sort((a, b) => {
      const aIsUserTop = a.queueId.startsWith('USERTOP-');
      const bIsUserTop = b.queueId.startsWith('USERTOP-');
      if (aIsUserTop && !bIsUserTop) return -1;
      if (!aIsUserTop && bIsUserTop) return 1;
      return 0; // 保持原有相对顺序
    })
  }));
});

const expandedUsers = ref({});

const toggleUser = (token) => {
  // Use simple boolean toggle. Need to ensure reactivity works.
  // Directly setting property on object might require spread if it was not initialized.
  expandedUsers.value = {
    ...expandedUsers.value,
    [token]: !expandedUsers.value[token]
  };
};

const exportLikedSongs = () => {
  if (!player.likedSongs.length) return;
  const blob = new Blob([createLikedSongsText(player.likedSongs)], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = createLikedSongsFilename();
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

const cancelSelection = () => {
  pendingDelete.value = false;
  exitSelectionMode();
};

const topSelected = () => {
  if (!hasSelection.value) return;
  player.topSongs(selectedIds.value);
  cancelSelection();
};

const requestDeleteSelected = () => {
  if (!hasSelection.value) return;
  pendingDelete.value = true;
};

const confirmDeleteSelected = () => {
  if (!hasSelection.value) return;
  player.removeSongs(selectedIds.value);
  cancelSelection();
};
</script>

<style scoped>
.queue-view-toggle {
  min-height: 2.25rem;
  border-radius: 0.85rem;
  color: var(--text-secondary);
  font-size: 0.78rem;
  font-weight: 700;
  transition: background-color 0.18s ease, color 0.18s ease;
}

.queue-view-toggle--active {
  background: var(--surface-1);
  color: var(--text-primary);
  box-shadow: 0 8px 22px rgba(0, 0, 0, 0.12);
}

.liked-remove {
  display: inline-flex;
  min-width: 2.5rem;
  min-height: 2.5rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: var(--surface-3);
  color: var(--error);
}

.bulk-action {
  min-height: 2rem;
  border-radius: 9999px;
  background: var(--surface-3);
  padding: 0 0.65rem;
  color: var(--text-secondary);
  font-size: 0.72rem;
  font-weight: 800;
}

.bulk-action:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.bulk-action--danger {
  color: var(--error);
}

.confirm-delete,
.cancel-delete {
  min-height: 2.25rem;
  flex: 1;
  border-radius: 9999px;
  font-size: 0.78rem;
  font-weight: 800;
}

.confirm-delete {
  background: var(--error);
  color: var(--text-inverse);
}

.cancel-delete {
  background: var(--surface-2);
  color: var(--text-secondary);
}
</style>
