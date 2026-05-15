<template>
  <div class="p-4">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-sm font-semibold text-[var(--text-primary)]">{{ t('settings.onlineMembers') }}</h3>
      <div class="rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2.5 py-0.5 text-xs font-mono text-[var(--text-secondary)]">
        {{ users.length }}
      </div>
    </div>

    <div class="space-y-3">
      <!-- 自己 -->
      <div
          id="tutorial-rename"
          class="flex items-center gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2 transition-colors"
          :class="[
              isEnqueuerById(userStore.publicId) ? 'border-[var(--accent)]/30 bg-[var(--accent-subtle)]' :
              isLikedUser(userStore.publicId) ? 'bg-[var(--surface-3)]' : ''
          ]"
      >
        <div
            class="flex h-8 w-8 items-center justify-center rounded-full text-xs font-semibold transition-colors"
            :class="[
                isEnqueuerById(userStore.publicId) ? 'bg-[var(--accent)] text-[var(--text-inverse)]' :
                isLikedUser(userStore.publicId) ? 'bg-[var(--accent)] text-[var(--text-inverse)]' :
                'bg-[var(--surface-3)] text-[var(--text-primary)]'
            ]"
        >
          <span v-if="isEnqueuerById(userStore.publicId)" :aria-label="t('userList.dj')">{{ t('userList.dj') }}</span>
          <Zap v-else-if="isLikedUser(userStore.publicId)" class="w-4 h-4 fill-current text-[var(--text-inverse)]" />
          <span v-else :aria-label="t('userList.me')">{{ t('userList.me') }}</span>
        </div>
        <div class="flex-1 min-w-0">
          <input
              v-model="newName"
              @blur="doRename"
              @keyup.enter="doRename"
              class="w-full bg-transparent text-sm font-semibold outline-none border-b border-transparent focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
              :aria-label="t('userList.rename')"
              :class="isEnqueuerById(userStore.publicId) ? 'text-[var(--accent)]' : 'text-[var(--text-primary)]'"
          />
        </div>

        <div v-if="isEnqueuerById(userStore.publicId)" class="flex gap-0.5 items-end h-4" :aria-label="t('userList.statusDJ')">
          <div class="bar bar-1 bg-[var(--accent)]"></div>
          <div class="bar bar-2 bg-[var(--accent)]"></div>
          <div class="bar bar-3 bg-[var(--accent)]"></div>
        </div>
        <div v-else class="h-2 w-2 rounded-full bg-[var(--accent)] animate-pulse" :aria-label="t('userList.statusOnline')"></div>
      </div>

      <!-- 其他人 -->
      <div
          v-for="u in others"
          :key="u.publicId"
          class="flex items-center gap-3 rounded-2xl border border-transparent bg-[var(--surface-2)] px-3 py-2 transition-colors hover:border-[var(--border-default)]"
          :class="[
              isEnqueuerById(u.publicId) ? 'bg-[var(--accent-subtle)] border-[var(--accent)]/20' :
              isLikedUser(u.publicId) ? 'bg-[var(--surface-3)]' : ''
          ]"
      >
        <div
            class="flex h-8 w-8 items-center justify-center rounded-full text-xs font-semibold transition-colors"
            :class="[
                isEnqueuerById(u.publicId) ? 'bg-[var(--accent)] text-[var(--text-inverse)]' :
                isLikedUser(u.publicId) ? 'bg-[var(--accent)] text-[var(--text-inverse)]' :
                'bg-[var(--surface-3)] text-[var(--text-secondary)]'
            ]"
        >
          <span v-if="isEnqueuerById(u.publicId)" :aria-label="t('userList.dj')">{{ t('userList.dj') }}</span>
          <Zap v-else-if="isLikedUser(u.publicId)" class="w-4 h-4 fill-current text-[var(--text-inverse)]" />
          <span v-else :aria-label="t('settings.member')">{{ t('settings.member') }}</span>
        </div>
        <div
            class="flex-1 truncate text-sm font-semibold"
            :class="isEnqueuerById(u.publicId) ? 'text-[var(--accent)]' : 'text-[var(--text-primary)]'"
        >
          {{ u.name }}
        </div>

        <div v-if="isEnqueuerById(u.publicId)" class="flex gap-0.5 items-end h-4" :aria-label="t('userList.statusDJ')">
          <div class="bar bar-1 bg-[var(--accent)]"></div>
          <div class="bar bar-2 bg-[var(--accent)]"></div>
          <div class="bar bar-3 bg-[var(--accent)]"></div>
        </div>
        <div v-else class="h-2 w-2 rounded-full bg-[var(--accent)]" :aria-label="t('userList.statusOnline')"></div>
      </div>
    </div>

    <!-- 直播流人数 -->
    <div v-if="playerStore.streamListenerCount > 0" class="mt-4 pt-3 border-t border-[var(--border-default)]">
      <div class="flex items-center gap-3 rounded-2xl bg-[var(--surface-2)] px-3 py-2 opacity-70">
        <div class="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--surface-3)] text-[10px] font-semibold text-[var(--text-secondary)]" :aria-label="t('userList.live')">
          {{ t('userList.live') }}
        </div>
        <div class="text-xs font-semibold text-[var(--text-primary)]">
          {{ t('userList.streamListeners', { count: playerStore.streamListenerCount }) }}
        </div>
        <div class="flex-1"></div>
        <div class="h-2 w-2 rounded-full bg-[var(--accent)] animate-pulse"></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useUserStore } from '../stores/user';
import { usePlayerStore } from '../stores/player';
import { Zap } from 'lucide-vue-next';

const { t } = useI18n();
const userStore = useUserStore();
const playerStore = usePlayerStore();
const users = computed(() => userStore.onlineUsers);
const me = computed(() => userStore.currentUser);
const newName = ref(me.value.name);

const isLikedUser = (token) => {
  if (!playerStore.nowPlaying) return false;
  return playerStore.nowPlaying.likedUserIds?.includes(token);
};

watch(() => me.value.name, (n) => newName.value = n);

const others = computed(() => users.value.filter(u => u.publicId !== userStore.publicId));

const doRename = () => {
  if(newName.value && newName.value !== me.value.name) {
    playerStore.renameUser(newName.value);
  }
};

const isEnqueuerById = (token) => {
  if (!playerStore.nowPlaying) return false;
  // 后端 NowPlayingInfo 现在存的是 enqueuedById (Token)
  // 我们比较：这首歌的Token === 列表里该用户的Token
  return playerStore.nowPlaying.enqueuedById === token;
};
</script>

<style scoped>
/* 使用标准 CSS 实现跳动效果 */
.bar {
  width: 3px;
  border-radius: 1px;
  /* 使用 transform 性能更好 */
  transform-origin: bottom;
  animation: bounce infinite ease-in-out;
}

.bar-1 { animation-duration: 0.6s; height: 60%; }
.bar-2 { animation-duration: 0.8s; height: 100%; }
.bar-3 { animation-duration: 0.5s; height: 40%; }

@keyframes bounce {
  0%, 100% { transform: scaleY(0.4); }
  50% { transform: scaleY(1); }
}

@media (prefers-reduced-motion: reduce) {
  .bar {
    animation: none;
    transform: scaleY(0.7);
  }
}
</style>

