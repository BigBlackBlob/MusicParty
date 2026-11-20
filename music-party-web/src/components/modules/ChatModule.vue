<template>
  <section class="chat-module flex h-full flex-col overflow-hidden" :aria-label="t('chat.title')">
    <!-- Single 46px Flat Underline Header (aligned with QueuePlaylistsModule) -->
    <header class="chat-header">
      <nav class="chat-tabs" role="tablist" :aria-label="t('chat.title')">
        <button
          v-for="tab in chatTabs"
          :key="tab.value"
          type="button"
          role="tab"
          class="chat-tab"
          :class="{ 'is-active': activeTab === tab.value }"
          :aria-selected="activeTab === tab.value"
          :aria-label="tab.label"
          :title="tab.label"
          @click="activeTab = tab.value"
        >
          {{ tab.label }}
        </button>
      </nav>
    </header>

    <div
        ref="msgListRef"
        @scroll="handleScroll"
        class="chat-scroll flex-1 space-y-4 overflow-y-auto bg-transparent px-3 py-3"
    >
      <div v-if="chatStore.isLoadingMore" class="flex justify-center py-2">
        <div class="h-4 w-4 animate-spin rounded-full border-2 border-[var(--surface-control-active)] border-t-[var(--accent)]" />
      </div>

      <div v-if="processedMessages.length === 0" class="flex flex-col items-center gap-2 py-8 text-center">
        <span class="material-symbols-outlined text-[20px] text-[var(--text-tertiary)] opacity-50" aria-hidden="true" style="font-variation-settings:'FILL' 0,'wght' 300">chat</span>
        <span class="text-xs text-[var(--text-tertiary)]">{{ t('chat.empty') }}</span>
      </div>

      <div v-for="item in processedMessages" :key="item.msg.id">
        <div v-if="item.showTime" class="mb-3 flex justify-center">
          <span class="rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2.5 py-0.5 text-[10px] font-mono text-[var(--text-tertiary)]">
            {{ formatTime(item.msg.timestamp) }}
          </span>
        </div>

        <div v-if="item.msg.type === 'CHAT'" class="flex flex-col text-sm" :class="isSelf(item.msg) ? 'items-end' : 'items-start'">
          <div class="mb-1 flex items-center gap-2 text-xs text-[var(--text-tertiary)]">
            <span v-if="!isSelf(item.msg)">{{ userStore.resolveName(item.msg.userId, item.msg.userName) }}</span>
          </div>
          <div
              class="max-w-[90%] select-text rounded-lg px-3 py-2 text-xs leading-relaxed shadow-sm"
              :class="isSelf(item.msg)
                ? 'bg-[var(--accent)] text-[var(--text-inverse)] rounded-br-md'
                : 'border border-[var(--border-default)] bg-[var(--surface-4)] text-[var(--text-primary)] rounded-bl-md'"
          >
            {{ item.msg.content }}
          </div>
        </div>

        <div v-else-if="item.msg.type === 'SYSTEM'" class="flex items-start gap-2 px-1 text-xs text-[var(--text-secondary)]/90">
          <span class="material-symbols-outlined mt-0.5 flex-shrink-0 text-[12px] text-[var(--text-tertiary)]" style="font-variation-settings:'FILL' 0,'wght' 300">terminal</span>
          <span class="select-text break-all font-sans text-xs leading-relaxed">
            {{ item.msg.content }}
          </span>
        </div>

        <div v-else-if="item.msg.type === 'LIKE'" class="my-1 flex justify-center">
          <div class="flex items-center gap-1.5 rounded-full border border-[var(--border-default)] bg-[var(--accent-subtle)] px-3 py-1 text-xs font-semibold text-[var(--accent)]">
            <span class="material-symbols-outlined text-[12px]" style="font-variation-settings:'FILL' 1,'wght' 400">bolt</span>
            <span>{{ item.msg.content }}</span>
          </div>
        </div>

        <div v-else-if="item.msg.type === 'PLAY_START'" class="my-2 flex justify-center">
          <div class="flex items-center gap-2 rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-3 py-1 text-xs font-mono text-[var(--text-secondary)]">
            <span class="h-2 w-2 rounded-full bg-[var(--accent)]"></span>
            <span>{{ item.msg.content }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="activeTab === CHAT_TAB || activeTab === PUBLIC_TAB" class="chat-composer flex items-center gap-2 border-t border-[var(--border-subtle)] bg-[var(--surface-2)] px-2.5 py-2">
      <input
          v-model="inputContent"
          @keyup.enter="send"
          :placeholder="activeTab === PUBLIC_TAB ? t('chat.publicPlaceholder') : t('chat.placeholder')"
          class="min-w-0 flex-1 rounded-lg border border-[var(--border-subtle)] bg-[var(--surface-1)] px-3 py-1.5 text-xs text-[var(--text-primary)] outline-none transition-colors placeholder:text-[var(--text-tertiary)] focus:border-[var(--border-accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
          :aria-label="t('chat.placeholder')"
      />
      <button
          type="button"
          @click="send"
          class="inline-flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-lg bg-[var(--accent)] text-[var(--on-primary)] transition-[background-color,transform] hover:bg-[var(--accent-hover)] active:scale-[0.95]"
          :aria-label="t('chat.send')"
      >
        <span class="material-symbols-outlined text-[17px]" style="font-variation-settings:'FILL' 1,'wght' 400">send</span>
      </button>
    </div>

    <div v-else class="flex h-[38px] items-center justify-center border-t border-[var(--border-subtle)] bg-[var(--surface-2)]">
      <span class="text-[11px] font-mono text-[var(--text-tertiary)]">{{ t('chat.readOnly') }}</span>
    </div>
  </section>
</template>

<script setup>
import { ref, watch, nextTick, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useChatStore } from '../../stores/chat';
import { useUserStore } from '../../stores/user';
import { useChatViewModel } from '../../composables/useChatViewModel';

const chatStore = useChatStore();
const userStore = useUserStore();
const { t } = useI18n();
const CHAT_TAB = 'CHAT';
const SYSTEM_TAB = 'SYSTEM';
const PUBLIC_TAB = 'PUBLIC';

const chatTabs = [
  { value: CHAT_TAB, label: t('chat.tabChat') },
  { value: SYSTEM_TAB, label: t('chat.tabSystem') },
  { value: PUBLIC_TAB, label: t('chat.tabPublic') }
];

const inputContent = ref('');
const msgListRef = ref(null);
const activeTab = ref(CHAT_TAB);
const {
  processedMessages,
  isSelf,
  formatTime,
  markAsRead,
  sendMessage
} = useChatViewModel(activeTab);

const scrollToBottom = async (force = false) => {
  await nextTick();
  if (msgListRef.value) {
    const el = msgListRef.value;
    const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
    if (isAtBottom || force) {
      el.scrollTop = el.scrollHeight;
    }
  }
};

const handleScroll = (e) => {
  const el = e.target;
  if (activeTab.value === PUBLIC_TAB) {
    if (el.scrollTop < 20 && chatStore.publicHasMore && !chatStore.isLoadingPublicMore) {
      chatStore.loadMorePublicHistory();
    }
    return;
  }
  if (el.scrollTop < 20 && chatStore.hasMore && !chatStore.isLoadingMore) {
    chatStore.loadMoreHistory();
  }
};

const send = () => {
  if (!sendMessage(inputContent.value)) return;
  inputContent.value = '';
  setTimeout(() => scrollToBottom(true), 100);
};

onMounted(() => {
  markAsRead();
  scrollToBottom(true);
});

watch(activeTab, () => {
  markAsRead();
  scrollToBottom(true);
});

watch(() => processedMessages.value.length, (newLen, oldLen) => {
  if (newLen > oldLen) {
    scrollToBottom(false);
  }
});
</script>

<style scoped>
.chat-module {
  background: var(--surface-panel);
  color: var(--text-primary);
}

.chat-header {
  height: 46px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-panel);
  flex-shrink: 0;
}

.chat-tabs {
  display: flex;
  align-items: stretch;
  height: 100%;
  width: 100%;
  gap: 4px;
}

.chat-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0 10px;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text-tertiary);
  white-space: nowrap;
  transition: color var(--motion-exit) var(--motion-ease-standard);
}

.chat-tab:hover {
  color: var(--text-secondary);
}

.chat-tab.is-active {
  color: var(--text-primary);
  font-weight: 600;
}

.chat-tab.is-active::after {
  content: '';
  position: absolute;
  left: 6px;
  right: 6px;
  bottom: 0;
  height: 2px;
  background: var(--accent);
  border-radius: var(--radius-xs) var(--radius-xs) 0 0;
}

.chat-scroll::-webkit-scrollbar {
  width: 4px;
}

.chat-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.chat-scroll::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: var(--radius-full);
}

.chat-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--accent-muted);
}
</style>
