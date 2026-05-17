<template>
  <div class="mobile-shell bg-bg-base overflow-hidden">
    <!-- Ambient Background -->
    <Transition name="mobile-ambient-fade">
      <div
        v-if="currentCover && activeTab === 'now'"
        :key="currentCover"
        class="mobile-ambient"
        :style="{ backgroundImage: `url(${currentCover})` }"
      />
    </Transition>

    <main class="mobile-shell-main">
      <Transition name="fade-slide" mode="out-in">
        <MobileNowPlaying 
          v-if="activeTab === 'now'" 
          :key="'now'" 
          @open-settings="showSettings = true"
          @close="activeTab = 'queue'"
        />
        <MobileQueueView v-else-if="activeTab === 'queue'" :key="'queue'" />
        <MobileSearchView v-else-if="activeTab === 'search'" :key="'search'" />
        <MobileChatView v-else-if="activeTab === 'chat'" :key="'chat'" />
      </Transition>
    </main>

    <MobileBottomNav v-model:active="activeTab" />

    <!-- Settings Dialog (Maintained for functionality) -->
    <DialogRoot :open="showSettings" @update:open="val => showSettings = val">
      <DialogPortal>
        <DialogOverlay class="fixed inset-0 z-[var(--z-modal)] bg-black/60 backdrop-blur-sm transition-all duration-300" />
        <DialogContent class="fixed inset-x-0 bottom-0 z-[var(--z-modal)] flex justify-center outline-none">
          <div class="mobile-members-sheet">
            <div class="flex items-center justify-between px-4 py-4 border-b border-[var(--surface-glass-border)]">
              <div>
                <DialogTitle class="text-sm font-bold text-[var(--text-primary)]">{{ t('settings.title') }}</DialogTitle>
              </div>
              <button 
                @click="showSettings = false"
                class="w-[32px] h-[32px] flex items-center justify-center rounded-full bg-surface-control text-text-primary"
              >
                <span class="material-symbols-outlined text-[18px]">close</span>
              </button>
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

              <AdminSettingsPanel />

              <!-- Members -->
              <div>
                <div class="flex items-center justify-between mb-3">
                  <h3 class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider">{{ t('settings.onlineMembers') }}</h3>
                  <span class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-widest">{{ user.onlineUsers.length }} {{ t('settings.active') }}</span>
                </div>
                <div class="flex flex-col gap-2">
                  <div
                    v-for="member in displayMembers"
                    :key="member.publicId || member.name"
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
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle } from 'reka-ui';
import AdminSettingsPanel from '../AdminSettingsPanel.vue';
import { useChatStore } from '../../stores/chat';
import { usePlayerStore } from '../../stores/player';
import { useUiStore } from '../../stores/ui';
import { useUserStore } from '../../stores/user';
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
  return [{ name: user.currentUser.name, publicId: user.publicId, isGuest: user.isGuest }];
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
}

.mobile-ambient {
  pointer-events: none;
  position: absolute;
  inset: -10%;
  z-index: 0;
  background-position: center;
  background-size: cover;
  filter: blur(60px) saturate(1.2) brightness(0.4);
  opacity: 0.3;
  transform: scale(1.1);
}

.mobile-shell-main {
  position: relative;
  z-index: 1;
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.mobile-members-sheet {
  display: flex;
  width: 100%;
  max-width: 520px;
  max-height: 80dvh;
  flex-direction: column;
  overflow: hidden;
  border-top: 1px solid var(--border-default);
  border-radius: 24px 24px 0 0;
  background: var(--surface-panel);
  backdrop-filter: blur(24px);
  box-shadow: 0 -18px 48px rgba(0,0,0,0.5);
}

.h-safe-bottom {
  height: env(safe-area-inset-bottom);
}

.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: opacity 300ms cubic-bezier(0.16, 1, 0.3, 1), transform 300ms cubic-bezier(0.16, 1, 0.3, 1);
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateX(10px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateX(-10px);
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
