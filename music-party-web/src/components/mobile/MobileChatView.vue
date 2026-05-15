<template>
  <section class="mobile-chat-page">
    <header class="mobile-chat-header">
      <div>
        <h2>{{ t('chat.title') }}</h2>
        <p>{{ filteredMessages.length }} {{ t('chat.messages') }}</p>
      </div>
      <div class="mobile-chat-tabs" role="tablist" :aria-label="t('chat.tabAria')">
        <button
          v-for="tab in ['CHAT', 'SYSTEM']"
          :key="tab"
          type="button"
          role="tab"
          :aria-selected="activeTab === tab"
          :class="{ 'mobile-chat-tabs__item--active': activeTab === tab }"
          @click="activeTab = tab"
        >
          {{ tab === 'CHAT' ? t('chat.tabChat') : t('chat.tabSystem') }}
        </button>
      </div>
    </header>

    <div ref="listRef" class="mobile-chat-list" @scroll="handleScroll">
      <div v-if="chat.isLoadingMore" class="mobile-chat-loading">
        <Loader2 class="h-4 w-4 animate-spin" />
      </div>

      <div v-if="processedMessages.length === 0" class="mobile-chat-empty">
        {{ t('chat.empty') }}
      </div>

      <div
        v-for="item in processedMessages"
        :key="item.msg.id"
        class="mobile-message"
        :class="[
          isSelf(item.msg) ? 'mobile-message--self' : 'mobile-message--other',
          item.msg.type !== 'CHAT' ? 'mobile-message--system' : ''
        ]"
      >
        <div v-if="item.showTime" class="mobile-message__time">{{ formatTime(item.msg.timestamp) }}</div>

        <template v-if="item.msg.type === 'CHAT'">
          <div v-if="!isSelf(item.msg)" class="mobile-message__name">{{ user.resolveName(item.msg.userId, item.msg.userName) }}</div>
          <div class="mobile-message__bubble">{{ item.msg.content }}</div>
        </template>
        <div v-else class="mobile-message__system">{{ item.msg.content }}</div>
      </div>
    </div>

    <form v-if="activeTab === 'CHAT'" class="mobile-chat-input" @submit.prevent="send">
      <input
        v-model="input"
        :placeholder="t('chat.placeholder')"
        enterkeyhint="send"
        autocomplete="off"
      />
      <button type="submit" :aria-label="t('chat.send')">
        <Send class="h-5 w-5" />
      </button>
    </form>
    <div v-else class="mobile-chat-readonly">{{ t('chat.readOnly') }}</div>
  </section>
</template>

<script setup>
import { nextTick, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Loader2, Send } from 'lucide-vue-next';
import { useChatViewModel } from '../../composables/useChatViewModel';

const { t } = useI18n();
const input = ref('');
const listRef = ref(null);
const activeTab = ref('CHAT');

const {
  chat,
  user,
  filteredMessages,
  processedMessages,
  isSelf,
  formatTime,
  markAsRead,
  sendMessage,
  scrollToBottom
} = useChatViewModel(activeTab);

const send = () => {
  if (!sendMessage(input.value)) return;
  input.value = '';
  scrollToBottom(listRef, true);
};

const handleScroll = (event) => {
  const el = event.target;
  if (el.scrollTop >= 20 || !chat.hasMore || chat.isLoadingMore) return;
  const oldHeight = el.scrollHeight;
  chat.loadMoreHistory();
  const unwatch = watch(() => chat.messages.length, async () => {
    await nextTick();
    const newHeight = el.scrollHeight;
    el.scrollTop = newHeight - oldHeight;
    unwatch();
  });
};

watch([() => processedMessages.value.length, activeTab], () => {
  markAsRead();
  scrollToBottom(listRef, true);
}, { immediate: true });
</script>

<style scoped>
.mobile-chat-page {
  display: grid;
  height: 100%;
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr) auto;
  overflow: hidden;
  background: transparent;
}

.mobile-chat-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid var(--surface-glass-border);
  background: var(--surface-glass-bg);
  backdrop-filter: blur(20px);
  padding: 14px 16px;
}

.mobile-chat-header h2 {
  color: var(--text-primary);
  font-size: 20px;
  font-weight: 800;
  line-height: 1.15;
}

.mobile-chat-header p {
  margin-top: 2px;
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}

.mobile-chat-tabs {
  display: inline-flex;
  border: 1px solid var(--surface-glass-border);
  border-radius: var(--radius-sm);
  background: var(--surface-glass-bg);
  padding: 2px;
}

.mobile-chat-tabs button {
  min-height: 32px;
  border-radius: var(--radius-xs);
  color: var(--text-tertiary);
  font-size: 12px;
  font-weight: 700;
  padding: 0 10px;
}

.mobile-chat-tabs__item--active {
  background: var(--surface-glass-control);
  color: var(--text-primary) !important;
}

.mobile-chat-list {
  min-height: 0;
  overflow-y: auto;
  padding: 14px 14px 18px;
  overscroll-behavior: contain;
}

.mobile-chat-empty {
  display: flex;
  height: 100%;
  align-items: center;
  justify-content: center;
  color: var(--text-tertiary);
  font-size: 13px;
}

.mobile-chat-loading {
  display: flex;
  justify-content: center;
  padding: 8px 0 12px;
  color: var(--accent);
}

.mobile-message {
  display: flex;
  flex-direction: column;
  margin-bottom: 10px;
}

.mobile-message--self {
  align-items: flex-end;
}

.mobile-message--other {
  align-items: flex-start;
}

.mobile-message__name {
  margin: 0 6px 4px;
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 700;
}

.mobile-message__time {
  align-self: center;
  margin: 4px 0 10px;
  border: 1px solid var(--surface-glass-border);
  border-radius: var(--radius-full);
  background: var(--surface-glass-bg);
  color: var(--text-tertiary);
  padding: 3px 8px;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 10px;
}

.mobile-message__bubble {
  max-width: min(82%, 340px);
  border-radius: 18px;
  padding: 9px 12px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
}

.mobile-message--self .mobile-message__bubble {
  border-bottom-right-radius: var(--radius-sm);
  background: var(--accent);
  color: var(--text-inverse);
}

.mobile-message--other .mobile-message__bubble {
  border-bottom-left-radius: var(--radius-sm);
  background: var(--surface-glass-control);
  color: var(--text-primary);
  backdrop-filter: blur(8px);
}

.mobile-message--system {
  align-items: center;
}

.mobile-message__system {
  max-width: 92%;
  border: 1px solid var(--surface-glass-border);
  border-radius: var(--radius-full);
  background: var(--surface-glass-bg);
  color: var(--text-secondary);
  padding: 5px 10px;
  text-align: center;
  font-size: 11px;
  line-height: 1.4;
}

.mobile-chat-input {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 48px;
  gap: 10px;
  border-top: 1px solid var(--surface-glass-border);
  background: var(--surface-glass-panel);
  backdrop-filter: blur(24px);
  padding: 10px 12px calc(10px + env(safe-area-inset-bottom));
}

.mobile-chat-input input {
  min-width: 0;
  min-height: 48px;
  border: 1px solid var(--surface-glass-border);
  border-radius: var(--radius-md);
  background: var(--surface-glass-bg);
  color: var(--text-primary);
  font-size: 16px;
  outline: none;
  padding: 0 14px;
}

.mobile-chat-input input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px rgba(211, 194, 243, 0.2);
}

.mobile-chat-input button {
  display: inline-flex;
  min-height: 48px;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  background: var(--accent);
  color: var(--text-inverse);
}

.mobile-chat-readonly {
  display: flex;
  min-height: calc(44px + env(safe-area-inset-bottom));
  align-items: center;
  justify-content: center;
  border-top: 1px solid var(--surface-glass-border);
  background: var(--surface-glass-panel);
  backdrop-filter: blur(24px);
  color: var(--text-tertiary);
  font-size: 12px;
  padding-bottom: env(safe-area-inset-bottom);
}
</style>
