<template>
  <section class="flex flex-col h-full bg-transparent relative overflow-hidden">
    <header class="z-20 bg-transparent border-b border-border-default pt-md px-md pb-3 flex flex-col gap-3 flex-shrink-0 safe-area-top">
      <div class="flex items-center justify-between gap-3">
        <div class="min-w-0">
          <p class="font-micro text-micro uppercase tracking-wider text-text-muted">
            {{ t('app.lounge') }}
          </p>
          <h1 class="font-title text-title text-text-primary mt-0.5">
            {{ t('settings.onlineMembers') }}
          </h1>
        </div>
        <span
          data-testid="mobile-members-count"
          class="flex-shrink-0 rounded-full border border-border-default bg-surface-raised px-2.5 py-0.5 font-micro text-micro text-text-secondary"
        >
          {{ t('userList.onlineCount', { count: presenceStore.users.length }) }}
        </span>
      </div>

      <div
        v-if="nowPlaying"
        data-testid="mobile-members-dj-spotlight"
        class="flex items-center justify-between gap-3 rounded-[var(--radius-md)] border border-border-default bg-accent-subtle px-3 py-2"
      >
        <div class="min-w-0 flex-1">
          <p class="font-micro text-micro uppercase tracking-wider text-primary truncate">
            {{ t('userList.nowSpinning', { name: djName }) }}
          </p>
          <p class="font-compact text-compact text-text-primary truncate mt-0.5">
            {{ nowPlaying.music?.name }}
          </p>
        </div>
        <div class="flex h-4 w-4 shrink-0 items-end gap-0.5" :aria-label="t('userList.statusDJ')">
          <div class="dj-bar dj-bar-1 bg-primary"></div>
          <div class="dj-bar dj-bar-2 bg-primary"></div>
          <div class="dj-bar dj-bar-3 bg-primary"></div>
        </div>
      </div>
    </header>

    <main class="flex-1 overflow-y-auto px-md py-3 pb-6">
      <UserList grouped hide-header />
    </main>
  </section>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoomPresenceStore } from '../../domains/realtime/roomPresenceStore';
import { useRoomRuntimeStore } from '../../domains/realtime/roomRuntimeStore';
import UserList from '../UserList.vue';

const { t } = useI18n();
const presenceStore = useRoomPresenceStore();
const runtimeStore = useRoomRuntimeStore();

const nowPlaying = computed(() => runtimeStore.nowPlaying);
const djName = computed(() => {
  const current = nowPlaying.value;
  if (!current) return '';
  if (current.enqueuedBy) return current.enqueuedBy;
  const matched = presenceStore.users.find(u => u.publicId === current.enqueuedById);
  return matched?.name || t('userList.dj');
});
</script>

<style scoped>
.safe-area-top {
  padding-top: calc(env(safe-area-inset-top) + 16px);
}

.dj-bar {
  width: 3px;
  border-radius: 1px;
  transform-origin: bottom;
}

.dj-bar-1 {
  animation-duration: 0.6s;
  height: 60%;
}

.dj-bar-2 {
  animation-duration: 0.8s;
  height: 100%;
}

.dj-bar-3 {
  animation-duration: 0.5s;
  height: 40%;
}

</style>
