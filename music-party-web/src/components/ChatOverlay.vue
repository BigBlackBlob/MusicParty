<template>
  <div class="pointer-events-none fixed bottom-5 right-5 z-[var(--z-chat)] flex flex-col items-end gap-3">
    <Transition
        enter-active-class="transition-all duration-300 ease-out"
        enter-from-class="opacity-0 scale-95"
        enter-to-class="opacity-100 scale-100"
        leave-active-class="transition-all duration-200 ease-in"
        leave-from-class="opacity-100 scale-100"
        leave-to-class="opacity-0 scale-95"
        @after-enter="() => scrollToBottom(true)"
    >
      <div
          v-if="chatStore.isOpen"
          class="pointer-events-auto flex flex-col overflow-hidden border border-border-default bg-surface-overlay/95 shadow-lg backdrop-blur-xl"
          :class="dynamicWindowClasses"
          @mousedown.stop
          @touchstart.stop
      >
        <div
            class="flex h-11 select-none items-center justify-between border-b border-border-subtle bg-[var(--surface-control)] px-3"
        >
          <div class="flex items-center gap-2 text-xs font-semibold tracking-[0.12em] text-[var(--text-secondary)]">
            <MessageSquare class="w-3.5 h-3.5 text-[var(--accent)]" />
            {{ t('chat.title') }}
          </div>
          <button @click="chatStore.toggleChat" class="min-w-[44px] min-h-[44px] inline-flex items-center justify-center p-1 text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)] active:scale-[0.96]" :aria-label="t('common.close')" :title="t('common.close')">
            <X class="w-4 h-4" />
          </button>
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
              @mousedown.stop
              @touchstart.stop
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
    </Transition>

    <div
        id="tutorial-chat"
        v-if="!isMobile || !chatStore.isOpen"
        @pointerdown="handlePointerDown"
        @click="handleClick"
        class="pointer-events-auto relative flex h-11 w-11 select-none items-center justify-center overflow-hidden rounded-lg border border-border-default bg-surface-overlay/90 text-[var(--text-secondary)] shadow-lg backdrop-blur-xl transition-colors hover:bg-[var(--surface-3)] hover:text-[var(--text-primary)] active:scale-[0.96]"
        :class="totalUnreadCount > 0 ? 'border-[var(--accent)] text-[var(--accent)]' : ''"
    >
      <span
          v-if="totalUnreadCount > 0"
          class="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_top,var(--accent-muted),transparent_60%)]"
      ></span>

      <span v-if="totalUnreadCount > 0" class="relative z-10 font-mono text-sm font-semibold">
        {{ totalUnreadCount > 99 ? '99+' : totalUnreadCount }}
      </span>
      <MessageSquare v-else class="relative z-10 h-5 w-5" />
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useChatStore } from '../stores/chat';
import { useUserStore } from '../stores/user';
import { useChatViewModel } from '../composables/useChatViewModel';
import { useWindowSize } from '@vueuse/core';
import { MessageSquare, X, Send, Terminal, Zap, Loader2 } from 'lucide-vue-next';

const chatStore = useChatStore();
const userStore = useUserStore();
const { t } = useI18n();
const totalUnreadCount = computed(() => chatStore.unreadCount + chatStore.publicUnreadCount);
const { width: windowWidth } = useWindowSize();
const CHAT_TAB = 'CHAT';
const SYSTEM_TAB = 'SYSTEM';
const PUBLIC_TAB = 'PUBLIC';

const isMobile = computed(() => windowWidth.value < 768);
const chatTabs = computed(() => ([
  { value: CHAT_TAB, label: t('chat.tabChat') },
  { value: SYSTEM_TAB, label: t('chat.tabSystem') },
  { value: PUBLIC_TAB, label: 'PUBLIC' }
]));

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

let startDragPos = { x: 0, y: 0 };
const handlePointerDown = (e) => {
  startDragPos = { x: e.clientX, y: e.clientY };
};
const handleClick = (e) => {
  const dx = Math.abs(e.clientX - startDragPos.x);
  const dy = Math.abs(e.clientY - startDragPos.y);
  if (dx > 5 || dy > 5) return;

  if (userStore.isGuest) {
    userStore.setPostNameAction(() => {
      if (!chatStore.isOpen) chatStore.toggleChat();
    });
    userStore.showNameModal = true;
    return;
  }
  chatStore.toggleChat();
};

const dynamicWindowClasses = computed(() => {
  if (isMobile.value) {
    return ['fixed', 'inset-0', 'm-auto', 'h-[75vh]', 'w-[90vw]', 'max-h-[600px]', 'max-w-[420px]', 'rounded-xl'];
  }
  return ['h-[50vh]', 'w-[340px]', 'md:h-[480px]', 'rounded-lg'];
});

const scrollToBottom = async (force = false) => {
  await nextTick();
  if (msgListRef.value) {
    const el = msgListRef.value;
    const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
    if (isAtBottom || force) {
      el.scrollTop = el.scrollHeight;
      if (force) {
        requestAnimationFrame(() => {
          if (el) el.scrollTop = el.scrollHeight;
        });
      }
    }
  }
};

const handleScroll = (e) => {
  const el = e.target;
  if (activeTab.value === PUBLIC_TAB) {
    if (el.scrollTop < 20 && chatStore.publicHasMore && !chatStore.isLoadingPublicMore) {
      const oldHeight = el.scrollHeight;
      chatStore.loadMorePublicHistory();
      const unwatch = watch(() => chatStore.publicMessages.length, async () => {
        await nextTick();
        const newHeight = el.scrollHeight;
        el.scrollTop = newHeight - oldHeight;
        unwatch();
      });
    }
    return;
  }
  if (el.scrollTop < 20 && chatStore.hasMore && !chatStore.isLoadingMore) {
    const oldHeight = el.scrollHeight;
    chatStore.loadMoreHistory();
    const unwatch = watch(() => chatStore.messages.length, async () => {
      await nextTick();
      const newHeight = el.scrollHeight;
      el.scrollTop = newHeight - oldHeight;
      unwatch();
    });
  }
};

const send = () => {
  if (!sendMessage(inputContent.value)) return;
  inputContent.value = '';
  setTimeout(() => scrollToBottom(true), 100);
};

const toggleChat = () => {
  chatStore.toggleChat();
};

watch([() => chatStore.isOpen, activeTab], async ([isOpen]) => {
  if (isOpen) {
    markAsRead();
    await scrollToBottom(true);
  }
});

watch(() => processedMessages.value.length, (newLen, oldLen) => {
  if (newLen > oldLen) {
    scrollToBottom(false);
  }
});

defineExpose({
  toggleChat
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
