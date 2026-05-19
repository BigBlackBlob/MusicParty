<template>
  <section class="flex flex-col h-full bg-bg-base relative overflow-hidden">
    <!-- Header Section -->
    <header class="z-20 bg-surface-panel border-b border-border-default pt-md px-md pb-3 flex flex-col gap-4 flex-shrink-0 safe-area-top">
      <div class="flex justify-between items-center">
        <h1 class="font-title text-title text-primary">{{ t('chat.title') }}</h1>
        <div class="flex items-center gap-2">
          <span v-if="filteredMessages.length > 0" class="px-2 py-0.5 rounded-full bg-accent-subtle border border-primary/20 text-[10px] font-bold text-primary uppercase tracking-widest">
            {{ filteredMessages.length }} {{ t('chat.messages') }}
          </span>
        </div>
      </div>

      <!-- Segmented Tabs -->
      <div class="bg-bg-base p-[4px] rounded-xl flex items-center justify-between shadow-inner">
        <button 
          v-for="tab in ['CHAT', 'SYSTEM']"
          :key="tab"
          type="button"
          @click="activeTab = tab"
          class="flex-1 py-1.5 rounded-lg font-section-label text-section-label transition-all"
          :class="activeTab === tab ? 'bg-surface-raised text-primary shadow-sm border border-border-strong' : 'text-text-secondary hover:text-primary'"
        >
          {{ tab === 'CHAT' ? t('chat.tabChat').toUpperCase() : t('chat.tabSystem').toUpperCase() }}
        </button>
      </div>
    </header>

    <!-- Chat List Area -->
    <div ref="listRef" class="flex-1 overflow-y-auto px-md py-4 flex flex-col gap-4" @scroll="handleScroll">
      <div v-if="chat.isLoadingMore" class="flex justify-center py-2 text-primary">
        <span class="material-symbols-outlined animate-spin">refresh</span>
      </div>

      <div v-if="processedMessages.length === 0" class="flex flex-col items-center justify-center py-20 text-center opacity-40">
        <span class="material-symbols-outlined text-[48px] mb-2">chat_bubble</span>
        <p class="font-compact text-compact uppercase tracking-widest">{{ t('chat.empty') }}</p>
      </div>

      <div
        v-for="item in processedMessages"
        :key="item.msg.id"
        class="flex flex-col"
        :class="[
          isSelf(item.msg) ? 'items-end' : 'items-start',
          item.msg.type !== 'CHAT' ? 'items-center' : ''
        ]"
      >
        <div v-if="item.showTime" class="self-center my-4 px-3 py-1 rounded-full bg-surface-raised border border-border-default text-micro font-micro text-text-muted">
          {{ formatTime(item.msg.timestamp) }}
        </div>

        <template v-if="item.msg.type === 'CHAT'">
          <div v-if="!isSelf(item.msg)" class="mb-1 ml-2 text-micro font-micro text-text-muted uppercase tracking-wider">
            {{ user.resolveName(item.msg.userId, item.msg.userName) }}
          </div>
          <div 
            class="max-w-[85%] px-4 py-2.5 rounded-2xl text-sm leading-relaxed break-words shadow-sm"
            :class="isSelf(item.msg) ? 'bg-primary text-on-primary rounded-tr-none' : 'bg-surface-panel text-text-primary border border-border-default rounded-tl-none'"
          >
            {{ item.msg.content }}
          </div>
        </template>
        
        <div v-else class="max-w-[90%] px-4 py-1.5 rounded-full bg-surface-raised border border-border-default text-[11px] text-text-secondary text-center">
          {{ item.msg.content }}
        </div>
      </div>
    </div>

    <!-- Input Section -->
    <footer class="p-md bg-bg-base/95 backdrop-blur-md border-t border-border-default">
      <form v-if="activeTab === 'CHAT'" class="flex items-center gap-3" @submit.prevent="send">
        <div class="flex-1 flex items-center bg-surface-panel h-[48px] rounded-2xl px-4 border border-border-default focus-within:border-primary/50 transition-colors group">
          <input 
            v-model="input"
            class="flex-1 bg-transparent border-none outline-none text-body font-body text-primary placeholder-text-muted w-full" 
            :placeholder="t('chat.placeholder')" 
            type="text"
            autocomplete="off"
            enterkeyhint="send"
          />
        </div>
        <button 
          type="submit"
          :disabled="!input.trim()"
          class="w-[48px] h-[48px] flex items-center justify-center rounded-2xl bg-primary text-on-primary shadow-lg shadow-primary/20 hover:scale-105 active:scale-95 transition-all disabled:opacity-50 disabled:scale-100"
        >
          <span class="material-symbols-outlined">send</span>
        </button>
      </form>
      <div v-else class="h-[48px] flex items-center justify-center text-micro font-micro text-text-muted uppercase tracking-widest">
        {{ t('chat.readOnly') }}
      </div>
    </footer>
  </section>
</template>

<script setup>
import { nextTick, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
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
.safe-area-top {
  padding-top: calc(env(safe-area-inset-top) + 16px);
}
</style>
