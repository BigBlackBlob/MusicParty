<template>
  <div id="tutorial-queue" class="flex h-full flex-col bg-[var(--surface-1)]">
    <div class="flex items-center justify-between border-b border-[var(--border-default)] px-4 py-3">
      <h3 class="text-sm font-semibold text-[var(--text-primary)]">播放队列</h3>
      <div class="rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2.5 py-0.5 text-xs font-mono text-[var(--text-secondary)]">
        {{ queue.length }}
      </div>
    </div>

    <div v-if="queue.length === 0" class="flex flex-1 flex-col items-center justify-center px-4 py-10 text-center">
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
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useVirtualList } from '@vueuse/core';
import { usePlayerStore } from '../stores/player';
import { Shuffle, ChevronDown, ChevronRight, User, Music2 } from 'lucide-vue-next';
import { useUserStore } from '../stores/user';
import QueueItem from './QueueItem.vue';

const player = usePlayerStore();
const queue = computed(() => player.queue);
const userStore = useUserStore();

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
</script>
