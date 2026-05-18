<template>
  <div class="flex h-full flex-col overflow-hidden bg-surface-overlay/50 backdrop-blur-md">
    <div
        class="flex h-11 select-none items-center justify-between border-b border-border-subtle bg-[var(--surface-control)] px-3"
    >
      <div class="flex items-center gap-2 text-xs font-semibold tracking-[0.12em] text-[var(--text-secondary)]">
        <MessageSquare class="w-3.5 h-3.5 text-[var(--accent)]" />
        {{ t('chat.title') }}
      </div>
    </div>

    <div class="flex border-b border-[var(--border-default)] bg-[var(--surface-2)]">
      <button
          v-for="tab in chatTabs"
          :key="tab.value"
          @click="activeTab = tab.value"
          class="relative flex-1 px-3 py-2 text-xs font-semibold tracking-[0.14em] transition-colors"
          :class="activeTab === tab.value ? 'text-[var(--text-primary)] bg-[var(--surface-3)]' : 'text-[var(--text-tertiary)] hover:text-[var(--text-secondary)] hover:bg-[var(--surface-3)]/60'"
          :aria-label="tab.label"
          :title="tab.label"
      >
        {{ tab.label }}
        <span v-if="activeTab === tab.value" class="absolute inset-x-3 bottom-0 h-px bg-[var(--accent)]"></span>
      </button>
    </div>

    <div
        ref="msgListRef"
        @scroll="handleScroll"
        class="chat-scroll flex-1 space-y-4 overflow-y-auto bg-transparent px-3 py-3"
    >
      <div v-if="chatStore.isLoadingMore" class="flex justify-center py-2">
        <Loader2 class="w-4 h-4 animate-spin text-[var(--accent)]/60" />
      </div>

      <div v-if="processedMessages.length === 0" class="py-8 text-center text-xs text-[var(--text-tertiary)]">
        {{ t('chat.empty') }}
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
          <Terminal class="mt-0.5 h-3 w-3 flex-shrink-0 text-[var(--text-tertiary)]" />
          <span class="select-text break-all font-sans text-xs leading-relaxed">
            {{ item.msg.content }}
          </span>
        </div>

        <div v-else-if="item.msg.type === 'LIKE'" class="my-1 flex justify-center">
          <div class="flex items-center gap-1.5 rounded-full border border-[var(--border-default)] bg-[var(--accent-subtle)] px-3 py-1 text-xs font-semibold text-[var(--accent)]">
            <Zap class="h-3 w-3 fill-current" />
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

    <div v-if="activeTab === CHAT_TAB || activeTab === PUBLIC_TAB" class="flex gap-2 border-t border-[var(--border-default)] bg-[var(--surface-1)] p-2">
      <input
          v-model="inputContent"
          @keyup.enter="send"
          :placeholder="activeTab === PUBLIC_TAB ? 'Message public channel...' : t('chat.placeholder')"
          class="min-w-0 flex-1 rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2 text-xs text-[var(--text-primary)] outline-none transition-colors placeholder:text-[var(--text-tertiary)] focus:border-[var(--border-accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
          :aria-label="t('chat.placeholder')"
      />
      <button
          @click="send"
          class="inline-flex min-h-[44px] min-w-[44px] items-center justify-center rounded-xl bg-[var(--accent)] px-3 text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)] active:scale-[0.96]"
          :aria-label="t('chat.send')"
      >
        <Send class="h-4 w-4" />
      </button>
    </div>

    <div v-else class="flex h-8 items-center justify-center border-t border-[var(--border-default)] bg-[var(--surface-1)]">
      <span class="text-xs font-mono text-[var(--text-tertiary)]">{{ t('chat.readOnly') }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useChatStore } from '../../stores/chat';
import { useUserStore } from '../../stores/user';
import { useChatViewModel } from '../../composables/useChatViewModel';
import { MessageSquare, Send, Terminal, Zap, Loader2 } from 'lucide-vue-next';

const chatStore = useChatStore();
const userStore = useUserStore();
const { t } = useI18n();
const CHAT_TAB = 'CHAT';
const SYSTEM_TAB = 'SYSTEM';
const PUBLIC_TAB = 'PUBLIC';

const chatTabs = [
  { value: CHAT_TAB, label: t('chat.tabChat') },
  { value: SYSTEM_TAB, label: t('chat.tabSystem') },
  { value: PUBLIC_TAB, label: 'PUBLIC' }
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
.chat-scroll::-webkit-scrollbar {
  width: 4px;
}

.chat-scroll::-webkit-scrollbar-track {
  background: transparent;
}

.chat-scroll::-webkit-scrollbar-thumb {
  background: var(--accent-muted);
  border-radius: 999px;
}

.chat-scroll::-webkit-scrollbar-thumb:hover {
  background: var(--accent);
}
</style>
