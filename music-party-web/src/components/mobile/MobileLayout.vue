<template>
  <div class="mobile-shell">
    <Transition name="mobile-ambient-fade">
      <div
        v-if="currentCover"
        :key="currentCover"
        class="mobile-ambient"
        :style="{ backgroundImage: `url(${currentCover})` }"
      />
    </Transition>

    <header class="mobile-shell-header">
      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <span
            class="h-2 w-2 rounded-full"
            :class="player.connected ? 'bg-[var(--success)]' : 'bg-[var(--error)]'"
          />
          <h1>{{ t('app.lounge') }}</h1>
        </div>
        <p>{{ headerSubtitle }}</p>
      </div>

      <div class="flex items-center gap-1">
        <IconButton size="sm" @click="showSettings = true" :title="t('settings.title')">
          <Settings class="h-5 w-5 text-[var(--text-secondary)]" />
        </IconButton>
      </div>
    </header>

    <main class="mobile-shell-main">
      <Transition name="fade-slide" mode="out-in">
        <MobileNowPlaying v-if="activeTab === 'now'" :key="'now'" />
        <MobileQueueView v-else-if="activeTab === 'queue'" :key="'queue'" />
        <MobileSearchView v-else-if="activeTab === 'search'" :key="'search'" />
        <MobileChatView v-else-if="activeTab === 'chat'" :key="'chat'" />
      </Transition>
    </main>

    <MobileBottomNav v-model:active="activeTab" />

    <DialogRoot :open="showSettings" @update:open="val => showSettings = val">
      <DialogPortal>
        <DialogOverlay class="fixed inset-0 z-[var(--z-modal)] bg-black/60 backdrop-blur-sm transition-all duration-300" />
        <DialogContent class="fixed inset-x-0 bottom-0 z-[var(--z-modal)] flex justify-center outline-none">
          <div class="mobile-members-sheet">
            <div class="flex items-center justify-between px-4 py-4 border-b border-[var(--surface-glass-border)]">
              <div>
                <DialogTitle class="text-sm font-bold text-[var(--text-primary)]">{{ t('settings.title') }}</DialogTitle>
              </div>
              <IconButton variant="secondary" size="sm" @click="showSettings = false" :aria-label="t('common.close')">
                <X class="h-4 w-4" />
              </IconButton>
            </div>

            <div class="flex-1 overflow-y-auto p-4 flex flex-col gap-6">
              <!-- Appearance -->
              <div>
                <h3 class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider mb-3">{{ t('settings.theme') }}</h3>
                <div class="flex gap-2">
                  <button class="flex-1 py-2.5 rounded-md border text-sm font-bold transition-colors"
                    :class="ui.isDarkMode ? 'border-[var(--accent)] bg-[var(--accent-subtle)] text-[var(--text-primary)]' : 'border-[var(--border-subtle)] bg-[var(--surface-control)] text-[var(--text-secondary)]'"
                    @click="!ui.isDarkMode ? ui.toggleDarkMode() : null">
                    {{ t('settings.dark') }}
                  </button>
                  <button class="flex-1 py-2.5 rounded-md border text-sm font-bold transition-colors"
                    :class="!ui.isDarkMode ? 'border-[var(--accent)] bg-[var(--accent-subtle)] text-[var(--text-primary)]' : 'border-[var(--border-subtle)] bg-[var(--surface-control)] text-[var(--text-secondary)]'"
                    @click="ui.isDarkMode ? ui.toggleDarkMode() : null">
                    {{ t('settings.light') }}
                  </button>
                </div>
              </div>

              <!-- Language -->
              <div>
                <h3 class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider mb-3">{{ t('settings.language') }}</h3>
                <div class="flex gap-2">
                  <button class="flex-1 py-2.5 rounded-md border text-sm font-bold transition-colors"
                    :class="i18nLocale === 'en' ? 'border-[var(--accent)] bg-[var(--accent-subtle)] text-[var(--text-primary)]' : 'border-[var(--border-subtle)] bg-[var(--surface-control)] text-[var(--text-secondary)]'"
                    @click="ui.setLocale('en'); i18nLocale = 'en'">
                    {{ t('settings.english') }}
                  </button>
                  <button class="flex-1 py-2.5 rounded-md border text-sm font-bold transition-colors"
                    :class="i18nLocale === 'zh' ? 'border-[var(--accent)] bg-[var(--accent-subtle)] text-[var(--text-primary)]' : 'border-[var(--border-subtle)] bg-[var(--surface-control)] text-[var(--text-secondary)]'"
                    @click="ui.setLocale('zh'); i18nLocale = 'zh'">
                    {{ t('settings.chinese') }}
                  </button>
                </div>
              </div>

              <!-- Members -->
              <div>
                <div class="flex items-center justify-between mb-3">
                  <h3 class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider">{{ t('settings.onlineMembers') }}</h3>
                  <span class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-widest">{{ user.onlineUsers.length }} {{ t('settings.active') }}</span>
                </div>
                <div class="flex flex-col gap-2">
                  <div
                    v-for="member in displayMembers"
                    :key="member.token || member.sessionId || member.name"
                    class="flex items-center gap-3 rounded-[var(--radius-md)] px-3 py-2.5 transition-colors bg-[var(--surface-control)]"
                  >
                    <div class="flex h-9 w-9 items-center justify-center rounded-[var(--radius-sm)] bg-[var(--surface-3)] text-xs font-bold uppercase text-[var(--text-secondary)]">
                      {{ getInitials(member.name) }}
                    </div>
                    <div class="min-w-0 flex-1">
                      <div class="truncate text-sm font-bold text-[var(--text-primary)]">{{ member.name }}</div>
                      <div class="text-[10px] font-bold uppercase tracking-wider text-[var(--text-tertiary)]">{{ member.isGuest ? t('settings.guest') : t('settings.member') }}</div>
                    </div>
                    <div class="h-1.5 w-1.5 rounded-full bg-[var(--success)]" />
                  </div>
                </div>
              </div>
            </div>
            <div class="h-safe-bottom" />
          </div>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Settings, X } from 'lucide-vue-next';
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle } from 'reka-ui';
import { useChatStore } from '../../stores/chat';
import { usePlayerStore } from '../../stores/player';
import { useUiStore } from '../../stores/ui';
import { useUserStore } from '../../stores/user';
import IconButton from '../ui/IconButton.vue';
import MobileBottomNav from './MobileBottomNav.vue';
import MobileChatView from './MobileChatView.vue';
import MobileNowPlaying from './MobileNowPlaying.vue';
import MobileQueueView from './MobileQueueView.vue';
import MobileSearchView from './MobileSearchView.vue';

const { t, locale: i18nLocale } = useI18n();

const showSettings = ref(false);
const chat = useChatStore();
const player = usePlayerStore();
const ui = useUiStore();
const user = useUserStore();
const activeTab = computed({
  get: () => ui.mobileActiveTab,
  set: (tab) => ui.setMobileActiveTab(tab)
});

const currentCover = computed(() => player.nowPlaying?.music?.coverUrl || '');
const displayMembers = computed(() => {
  if (user.onlineUsers.length) return user.onlineUsers;
  return [{ name: user.currentUser.name, token: user.userToken, isGuest: user.isGuest }];
});
const headerSubtitle = computed(() => {
  if (!player.connected) return t('settings.disconnected');
  if (activeTab.value === 'queue') return `${player.queue.length} ${t('queue.tracks')}`;
  if (activeTab.value === 'search') return t('search.searchAndAdd');
  if (activeTab.value === 'chat') return `${chat.messages.length} ${t('chat.messages')}`;
  return player.nowPlaying?.music?.name || t('player.waiting');
});

const getInitials = (name = '') => {
  const normalized = String(name).trim();
  if (!normalized) return '?';
  const parts = normalized.split(/\s+/).filter(Boolean);
  if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
  return normalized.slice(0, 2).toUpperCase();
};

watch(activeTab, (tab) => {
  chat.isOpen = tab === 'chat';
  if (tab === 'chat') chat.unreadCount = 0;
});
</script>

<style scoped>
.mobile-shell {
  position: relative;
  display: flex;
  width: 100%;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  background: var(--surface-0);
  color: var(--text-primary);
}

.mobile-ambient {
  pointer-events: none;
  position: absolute;
  inset: -14%;
  z-index: 0;
  background-position: center;
  background-size: cover;
  filter: var(--ambient-filter);
  opacity: var(--ambient-opacity);
  transform: scale(1.04);
}

.mobile-shell-header {
  position: relative;
  z-index: var(--z-header);
  display: flex;
  min-height: calc(var(--mobile-top-bar-height) + env(safe-area-inset-top));
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid var(--border-default);
  background: color-mix(in srgb, var(--surface-1) 92%, transparent);
  padding: env(safe-area-inset-top) 20px 0;
}

.mobile-shell-header h1 {
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 800;
  line-height: 1;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  text-shadow: 0 2px 4px var(--surface-glass-bg);
}

.mobile-shell-header p {
  max-width: 58vw;
  overflow: hidden;
  margin-top: 4px;
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-shell-main {
  position: relative;
  z-index: var(--z-tutorial);
  min-height: 0;
  flex: 1;
  overflow: hidden;
  background: color-mix(in srgb, var(--surface-0) 88%, transparent);
}

.mobile-members-sheet {
  display: flex;
  width: 100%;
  max-width: 520px;
  max-height: 80dvh;
  flex-direction: column;
  overflow: hidden;
  border-top: 1px solid var(--surface-glass-border);
  border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  background: var(--surface-glass-panel);
  backdrop-filter: blur(24px);
  box-shadow: 0 -18px 48px var(--surface-glass-bg);
}

.h-safe-bottom {
  height: env(safe-area-inset-bottom);
}

.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: opacity 240ms cubic-bezier(0.16, 1, 0.3, 1), transform 240ms cubic-bezier(0.16, 1, 0.3, 1);
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateX(12px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateX(-12px);
}

.mobile-ambient-fade-enter-active,
.mobile-ambient-fade-leave-active {
  transition: opacity 1000ms ease-out;
}

.mobile-ambient-fade-enter-from,
.mobile-ambient-fade-leave-to {
  opacity: 0;
}
</style>
