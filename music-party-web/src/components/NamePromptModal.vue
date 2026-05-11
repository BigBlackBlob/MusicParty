<!-- File: music-party-web/src/components/NamePromptModal.vue -->
<template>
  <div v-if="userStore.showNameModal" class="fixed inset-0 z-[300] bg-[var(--surface-0)]/90 backdrop-blur-xl flex items-center justify-center p-4">
    <div class="bg-[var(--surface-4)] p-6 w-full max-w-sm rounded-2xl shadow-2xl relative border border-[var(--border-default)] overflow-hidden">
      <div class="absolute inset-x-0 top-0 h-1 bg-[var(--accent)]"></div>

      <h2 class="text-xl font-bold text-[var(--text-primary)] mb-2">IDENTIFICATION REQUIRED</h2>
      <p class="text-xs font-sans text-[var(--text-secondary)] mb-6 leading-relaxed">
        在操作之前，先给自己取个名字吧<br>
        也可以在用户列表中点击自己的名字重命名
      </p>

      <input
          v-model="inputName"
          @keyup.enter="confirm"
          placeholder="ENTER CODENAME"
          class="w-full bg-[var(--surface-2)] border border-[var(--border-default)] p-3 outline-none focus:border-[var(--accent)] font-semibold mb-4 text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)] rounded-xl"
          autofocus
      />
      
      <div v-if="errorMsg" class="text-xs text-[var(--error-soft-text)] font-semibold mb-4 animate-pulse">{{ errorMsg }}</div>

      <div class="flex gap-2">
        <button @click="userStore.showNameModal = false" class="flex-1 py-3 text-xs font-semibold text-[var(--text-secondary)] hover:bg-[var(--surface-3)] rounded-xl">
          CANCEL
        </button>
        <button @click="confirm" class="flex-1 bg-[var(--accent)] text-[var(--text-inverse)] font-semibold py-3 hover:bg-[var(--accent-hover)] transition-colors rounded-xl">
          CONFIRM
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue';
import { useUserStore } from '../stores/user';
import { usePlayerStore } from '../stores/player';

const userStore = useUserStore();
const playerStore = usePlayerStore();
const inputName = ref('');
const errorMsg = ref('');

watch(inputName, () => errorMsg.value = '');

const confirm = () => {
  const name = inputName.value.trim();
  if(!name) return;

  if (name.toLowerCase().startsWith('guest') || name.startsWith('游客')) {
    errorMsg.value = '不能使用“游客”作为正式名字';
    return;
  }
  
  // 调用 renameUser，等待后端 socket 确认后关闭
  playerStore.renameUser(name);
};
</script>
