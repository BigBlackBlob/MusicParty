<template>
  <section class="flex h-full flex-col overflow-hidden">
    <div class="border-b border-[var(--border-default)] px-4 py-3">
      <h2 class="text-lg font-bold">聊天</h2>
      <p class="text-xs text-[var(--text-tertiary)]">{{ chat.messages.length }} 条消息</p>
    </div>

    <div ref="listRef" class="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
      <div v-if="chat.messages.length === 0" class="flex h-full items-center justify-center text-sm text-[var(--text-tertiary)]">
        暂无聊天记录
      </div>
      <div v-for="msg in chat.messages" :key="msg.id" class="flex flex-col" :class="isSelf(msg) ? 'items-end' : 'items-start'">
        <div v-if="msg.type === 'CHAT'" class="max-w-[82%] rounded-3xl px-4 py-2 text-sm leading-relaxed" :class="isSelf(msg) ? 'rounded-br-md bg-[var(--accent)] text-[var(--text-inverse)]' : 'rounded-bl-md bg-[var(--surface-4)] text-[var(--text-primary)]'">
          <div v-if="!isSelf(msg)" class="mb-1 text-[11px] font-semibold text-[var(--text-tertiary)]">{{ msg.userName || user.resolveName(msg.userId) }}</div>
          {{ msg.content }}
        </div>
        <div v-else class="mx-auto max-w-[92%] rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-3 py-1 text-center text-[11px] text-[var(--text-secondary)]">
          {{ msg.content }}
        </div>
      </div>
    </div>

    <div class="border-t border-[var(--border-default)] bg-[var(--surface-1)] p-3 pb-[calc(env(safe-area-inset-bottom)+0.75rem)]">
      <div class="flex gap-2">
        <input
          v-model="input"
          class="min-h-[48px] min-w-0 flex-1 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)] px-4 text-base text-[var(--text-primary)] outline-none focus:border-[var(--border-accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
          placeholder="输入消息"
          @keyup.enter="send"
        />
        <button class="min-h-[48px] min-w-[52px] rounded-2xl bg-[var(--accent)] text-[var(--text-inverse)] active:scale-[0.96]" @click="send" aria-label="发送消息">
          <Send class="mx-auto h-5 w-5" />
        </button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { nextTick, ref, watch } from 'vue';
import { Send } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';

const chat = useChatStore();
const player = usePlayerStore();
const user = useUserStore();
const input = ref('');
const listRef = ref(null);

const isSelf = (msg) => msg.userId === user.userToken;

const scrollToBottom = async () => {
  await nextTick();
  if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight;
};

const send = () => {
  const text = input.value.trim();
  if (!text) return;
  player.sendChatMessage(text);
  input.value = '';
  scrollToBottom();
};

watch(() => chat.messages.length, scrollToBottom);
</script>
