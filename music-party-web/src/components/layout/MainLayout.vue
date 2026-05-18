<template>
  <div class="h-[var(--app-height)] w-screen relative flex flex-col overflow-hidden bg-bg-base text-text-primary font-body text-body selection:bg-primary selection:text-on-primary" style="--top-bar-height: 64px;">
    <!-- Full-Bleed Ambient Canvas -->
    <div class="fixed inset-0 ambient-canvas z-0 opacity-40"></div>
    <Transition name="desktop-cover-fade" mode="out-in">
      <div
        v-if="currentCover"
        :key="currentCover"
        class="pointer-events-none fixed inset-0 z-[1] overflow-hidden"
      >
        <div
          class="desktop-page-cover-blur absolute"
          :style="{ backgroundImage: `url(${currentCover})` }"
        />
        <div class="absolute inset-0 desktop-page-cover-wash"></div>
      </div>
    </Transition>

    <!-- Background Vignette/Masking for depth -->
    <div class="pointer-events-none fixed inset-0 z-10 shadow-[inset_0_0_200px_var(--surface-scrim)]"></div>

    <!-- Lite Mode Overlay -->
    <LiteModeView
      v-if="uiStore.isLiteMode"
      :now-playing="playerStore.nowPlaying"
      v-model:volume="uiStore.volume"
      v-model:auto-lite="uiStore.autoLiteMode"
      @exit="uiStore.toggleLiteMode"
      class="z-[var(--z-overlay)]"
    />

    <template v-else>
      <!-- Top Navigation Bar -->
      <header class="fixed top-0 z-50 flex h-[64px] w-full items-center justify-between bg-transparent px-5">
        <div class="flex min-w-0 items-center gap-5">
          <div class="flex items-center gap-2">
            <button
              class="font-display flex items-center gap-2 text-[24px] font-black leading-none tracking-tighter text-primary transition-opacity hover:opacity-80"
              @click="toggleRoomMenu"
              :title="`Current room: ${roomStore.currentRoom?.name || 'Lounge'}`"
            >
              <span>{{ roomStore.currentRoom?.name || t('app.lounge') }}</span>
              <span class="material-symbols-outlined text-[18px]">expand_more</span>
            </button>
            <span
              class="h-2 w-2 rounded-full"
              :class="playerStore.connected ? 'bg-[#22C55E] shadow-[0_0_14px_rgba(34,197,94,0.45)]' : 'bg-error'"
              :title="playerStore.connected ? t('settings.connected') : t('settings.disconnected')"
            />
          </div>
          <div
            v-if="isRoomMenuOpen"
            class="absolute left-5 top-14 z-50 w-[340px] rounded-lg border border-border-default bg-surface-panel p-3 shadow-2xl backdrop-blur-xl"
          >
            <div class="mb-3 flex items-center justify-between">
              <span class="font-micro text-[10px] font-black uppercase tracking-[0.18em] text-text-tertiary">{{ t('rooms.title') }}</span>
              <button class="text-[10px] font-bold text-text-muted hover:text-primary transition-colors" @click="roomStore.fetchRooms">{{ t('common.refresh') }}</button>
            </div>
            <div class="max-h-[320px] space-y-1 overflow-y-auto pr-1 custom-scrollbar">
              <button
                v-for="room in roomStore.rooms"
                :key="room.roomId"
                class="flex w-full items-center justify-between rounded-md px-3 py-2.5 text-left transition-all group"
                :class="roomStore.currentRoomId === room.roomId ? 'bg-primary/15 text-primary' : 'text-text-secondary hover:bg-surface-raised hover:text-text-primary'"
                @click="switchRoom(room.roomId)"
              >
                <div class="flex flex-col min-w-0 flex-1">
                   <span class="truncate font-compact text-sm font-bold">{{ room.name }}</span>
                   <span class="text-[10px] opacity-60 font-mono tracking-tight">{{ room.onlineCount || 0 }} {{ t('settings.active') }}</span>
                </div>
                
                <div class="ml-3 flex items-center gap-2">
                  <span v-if="roomStore.currentRoomId === room.roomId" class="h-1.5 w-1.5 rounded-full bg-primary shadow-[0_0_8px_var(--primary)]"></span>
                  <button 
                    v-if="canDeleteRoom(room)" 
                    class="material-symbols-outlined text-[18px] opacity-0 group-hover:opacity-60 hover:!opacity-100 hover:text-error transition-all" 
                    @click.stop="deleteRoom(room)"
                  >
                    delete
                  </button>
                </div>
              </button>
            </div>
            <div class="mt-3 flex gap-2 border-t border-border-default pt-3">
              <input
                v-model="newRoomName"
                class="min-w-0 flex-1 rounded-md border border-border-default bg-surface-raised px-3 py-2 text-sm outline-none focus:border-primary text-text-primary placeholder:text-text-tertiary transition-colors"
                :placeholder="t('rooms.newRoomPlaceholder')"
                @keyup.enter="createRoom"
              />
              <button class="rounded-md bg-primary px-4 text-xs font-black uppercase tracking-widest text-on-primary hover:bg-[var(--accent-hover)] transition-colors" @click="createRoom">{{ t('rooms.create') }}</button>
            </div>
          </div>
          <div
            class="hidden min-w-[220px] cursor-pointer items-center gap-3 rounded-md border border-border-subtle bg-[var(--surface-control)] px-4 py-2 transition-colors hover:bg-[var(--surface-control-hover)] md:flex"
            :title="t('search.searchAndAdd')"
            @click="handleSearchClick"
          >
            <span class="material-symbols-outlined text-text-muted text-[18px]">search</span>
            <span class="text-text-muted font-compact text-compact flex items-center tracking-tight">{{ t('search.placeholder') }}</span>
          </div>
        </div>
        <div class="flex flex-shrink-0 items-center gap-3">
          <div class="relative">
            <div class="mr-4 flex -space-x-2 items-center cursor-pointer transition-transform hover:scale-105" @click="toggleUserList">
              <div
                v-for="user in visibleUsers"
                :key="user.publicId || user.name"
                class="flex h-8 w-8 items-center justify-center rounded-full border-2 border-surface-overlay bg-accent-subtle text-primary shadow-lg"
                :title="user.name"
              >
                <span class="font-micro text-micro uppercase">{{ getInitials(user.name) }}</span>
              </div>
              <div
                v-if="extraUserCount > 0"
                class="flex h-8 min-w-8 items-center justify-center rounded-full border-2 border-surface-overlay bg-accent-subtle px-2 text-primary shadow-lg"
                :title="t('settings.moreActiveUsers', { count: extraUserCount })"
              >
                <span class="font-micro text-micro">+{{ extraUserCount }}</span>
              </div>
            </div>

            <div
              v-if="isUserListOpen"
              class="absolute right-4 top-12 z-50 w-[320px] overflow-y-auto rounded-lg border border-border-default bg-surface-overlay/95 shadow-lg backdrop-blur-xl"
              style="max-height: min(520px, calc(var(--app-height) - 96px));"
            >
              <UserList />
            </div>
          </div>

          <button @click="uiStore.toggleDarkMode" class="flex h-10 w-10 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-text-primary" :title="t('settings.toggleTheme')">
            <span class="material-symbols-outlined">{{ uiStore.isDarkMode ? 'light_mode' : 'dark_mode' }}</span>
          </button>

          <button
            @click="layoutStore.enterEditMode"
            class="flex h-10 w-10 items-center justify-center rounded-md transition-all hover:bg-[var(--surface-control-hover)] active:scale-95"
            :class="layoutStore.isEditMode ? 'text-primary bg-primary/10' : 'text-text-secondary hover:text-text-primary'"
            :title="t('layout.editLayout')"
          >
            <span class="material-symbols-outlined">grid_view</span>
          </button>

          <div class="relative">
            <button @click="toggleSettings" class="flex h-10 w-10 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-text-primary" :title="t('settings.title')">
              <span class="material-symbols-outlined">settings</span>
            </button>

            <div
              v-if="isSettingsOpen"
              class="absolute right-0 top-12 w-[280px] rounded-lg border border-border-default bg-surface-overlay/95 p-3 text-sm shadow-lg backdrop-blur-xl"
            >
              <div class="mb-3 flex items-center justify-between border-b border-border-subtle pb-3">
                <span class="font-compact text-text-primary">{{ t('settings.title') }}</span>
                <span class="text-xs text-text-muted">{{ userStore.isGuest ? t('settings.guest') : userStore.currentUser.name }}</span>
              </div>
              <button class="flex w-full items-center justify-between rounded-md px-2 py-2 text-left hover:bg-[var(--surface-control-hover)]" @click="uiStore.toggleDarkMode">
                <span class="text-text-secondary">{{ t('settings.theme') }}</span>
                <span class="text-text-primary">{{ uiStore.isDarkMode ? t('settings.dark') : t('settings.light') }}</span>
              </button>
              <button class="flex w-full items-center justify-between rounded-md px-2 py-2 text-left hover:bg-[var(--surface-control-hover)]" @click="uiStore.setLocale(uiStore.locale === 'en' ? 'zh' : 'en')">
                <span class="text-text-secondary">{{ t('settings.language') }}</span>
                <span class="text-text-primary">{{ localeLabel }}</span>
              </button>
              <button class="flex w-full items-center justify-between rounded-md px-2 py-2 text-left hover:bg-[var(--surface-control-hover)]" @click="uiStore.toggleLiteMode">
                <span class="text-text-secondary">{{ t('settings.liteMode') }}</span>
                <span class="text-text-primary">{{ uiStore.isLiteMode ? t('settings.on') : t('settings.off') }}</span>
              </button>
              <label class="flex w-full cursor-pointer items-center justify-between rounded-md px-2 py-2 hover:bg-[var(--surface-control-hover)]">
                <span class="text-text-secondary">{{ t('settings.autoLiteMode') }}</span>
                <input v-model="uiStore.autoLiteMode" class="h-4 w-4 accent-[var(--accent)]" type="checkbox" />
              </label>

              <!-- Scale Controls -->
              <div class="mt-2 border-t border-border-subtle pt-3">
                <div class="mb-2 flex items-center justify-between px-2">
                  <span class="text-text-secondary">{{ t('settings.stageDensity') }}</span>
                  <span class="text-xs text-text-primary">{{ Math.round(uiStore.mainStageScale * 100) }}%</span>
                </div>
                <div class="flex items-center gap-1 px-2 pb-2">
                  <button class="flex-1 rounded py-1 text-xs transition-colors" :class="isScaleActive(0.92) ? 'bg-primary text-on-primary' : 'bg-[var(--surface-control)] text-text-secondary hover:text-text-primary hover:bg-[var(--surface-control-hover)]'" @click="uiStore.setMainStageScale(0.92)">{{ t('settings.compact') }}</button>
                  <button class="flex-1 rounded py-1 text-xs transition-colors" :class="isScaleActive(1.00) ? 'bg-primary text-on-primary' : 'bg-[var(--surface-control)] text-text-secondary hover:text-text-primary hover:bg-[var(--surface-control-hover)]'" @click="uiStore.setMainStageScale(1.00)">{{ t('settings.standard') }}</button>
                  <button class="flex-1 rounded py-1 text-xs transition-colors" :class="isScaleActive(1.12) ? 'bg-primary text-on-primary' : 'bg-[var(--surface-control)] text-text-secondary hover:text-text-primary hover:bg-[var(--surface-control-hover)]'" @click="uiStore.setMainStageScale(1.12)">{{ t('settings.relaxed') }}</button>
                </div>
                <div class="px-2 pb-2">
                  <input
                    :value="uiStore.mainStageScale"
                    @input="e => uiStore.setMainStageScale(e.target.value)"
                    type="range"
                    min="0.90"
                    max="1.20"
                    step="0.02"
                    class="w-full accent-[var(--accent)] cursor-pointer"
                  />
                </div>
              </div>

              <!-- Zoom Controls -->
              <div class="mt-2 border-t border-border-subtle pt-3">
                <div class="mb-2 flex items-center justify-between px-2">
                  <span class="text-text-secondary">{{ t('settings.globalZoom') }}</span>
                  <span class="text-xs text-text-primary">{{ Math.round(uiStore.globalZoomLevel * 100) }}%</span>
                </div>
                <div class="flex items-center gap-1 px-2 pb-2">
                  <button class="flex-1 rounded py-1 text-xs transition-colors" :class="isZoomActive(1.00) ? 'bg-primary text-on-primary' : 'bg-[var(--surface-control)] text-text-secondary hover:text-text-primary hover:bg-[var(--surface-control-hover)]'" @click="uiStore.setGlobalZoomLevel(1.00)">1.0x</button>
                  <button class="flex-1 rounded py-1 text-xs transition-colors" :class="isZoomActive(1.25) ? 'bg-primary text-on-primary' : 'bg-[var(--surface-control)] text-text-secondary hover:text-text-primary hover:bg-[var(--surface-control-hover)]'" @click="uiStore.setGlobalZoomLevel(1.25)">1.25x</button>
                  <button class="flex-1 rounded py-1 text-xs transition-colors" :class="isZoomActive(1.50) ? 'bg-primary text-on-primary' : 'bg-[var(--surface-control)] text-text-secondary hover:text-text-primary hover:bg-[var(--surface-control-hover)]'" @click="uiStore.setGlobalZoomLevel(1.50)">1.5x</button>
                </div>
                <div class="px-2 pb-2">
                  <input
                    :value="uiStore.globalZoomLevel"
                    @input="e => uiStore.setGlobalZoomLevel(e.target.value)"
                    type="range"
                    min="1.0"
                    max="1.5"
                    step="0.05"
                    class="w-full accent-[var(--accent)] cursor-pointer"
                  />
                </div>
              </div>

              <AdminSettingsPanel />

              <div class="mt-1 border-t border-border-subtle px-2 pt-3 text-xs text-text-muted">
                {{ t('settings.connection') }}: <span :class="playerStore.connected ? 'text-success' : 'text-error'">{{ playerStore.connected ? t('settings.connected') : t('settings.disconnected') }}</span>
              </div>
            </div>
          </div>
        </div>
      </header>

      <!-- Main Immersive Canvas -->
      <main class="relative z-20 flex w-full items-center justify-center px-5 pt-[var(--top-bar-height)]" style="height: var(--app-height);">
        <div
          class="flex w-full min-h-0 items-stretch overflow-hidden transition-all duration-300"
          :style="{
            '--stage-scale': uiStore.mainStageScale,
            '--stage-height': 'calc(var(--app-height) / var(--global-zoom) - var(--top-bar-height) / var(--global-zoom) - 40px)',
            maxWidth: `min(calc(1520px * ${uiStore.mainStageScale}), calc(100vw / var(--global-zoom) - 40px))`,
            zoom: uiStore.globalZoomLevel,
            '--global-zoom': uiStore.globalZoomLevel,
            height: `min(var(--stage-height), calc(708px * ${uiStore.mainStageScale}))`
          }"
          id="main-content-grid"
        >
          <slot />
        </div>
      </main>
    </template>
  </div>
</template>


<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import AdminSettingsPanel from '../AdminSettingsPanel.vue';
import LiteModeView from './LiteModeView.vue';
import UserList from '../UserList.vue';
import { useUserStore } from '../../stores/user';
import { useUiStore } from '../../stores/ui';
import { usePlayerStore } from '../../stores/player';
import { useRoomStore } from '../../stores/room';
import { useLayoutStore } from '../../stores/layout';

const emit = defineEmits(['search', 'toggle-mobile-chat']);
const { t } = useI18n();
const userStore = useUserStore();
const uiStore = useUiStore();
const playerStore = usePlayerStore();
const roomStore = useRoomStore();
const layoutStore = useLayoutStore();


const isSettingsOpen = ref(false);
const isUserListOpen = ref(false);
const isRoomMenuOpen = ref(false);
const newRoomName = ref('');
const currentMusic = computed(() => playerStore.nowPlaying?.music || null);
const currentCover = computed(() => currentMusic.value?.coverUrl || '');
const visibleUsers = computed(() => {
  const users = userStore.onlineUsers.length
    ? userStore.onlineUsers
    : [{ name: userStore.currentUser.name, publicId: userStore.publicId }];
  return users.slice(0, 3);
});
const extraUserCount = computed(() => Math.max(0, userStore.onlineUsers.length - visibleUsers.value.length));
const localeLabel = computed(() => uiStore.locale === 'en' ? t('settings.english') : t('settings.chinese'));

onMounted(() => {
  uiStore.fetchConfig();
});

const handleSearchClick = () => {
  emit('search');
}

const toggleSettings = () => {
  isSettingsOpen.value = !isSettingsOpen.value;
  if (isSettingsOpen.value) isUserListOpen.value = false;
  if (isSettingsOpen.value) isRoomMenuOpen.value = false;
};

const toggleUserList = () => {
  isUserListOpen.value = !isUserListOpen.value;
  if (isUserListOpen.value) isSettingsOpen.value = false;
  if (isUserListOpen.value) isRoomMenuOpen.value = false;
};

const toggleRoomMenu = () => {
  isRoomMenuOpen.value = !isRoomMenuOpen.value;
  if (isRoomMenuOpen.value) {
    isSettingsOpen.value = false;
    isUserListOpen.value = false;
    roomStore.fetchRooms();
  }
};

const switchRoom = (roomId) => {
  playerStore.switchRoom(roomId);
  isRoomMenuOpen.value = false;
};

const createRoom = () => {
  const name = newRoomName.value.trim();
  if (!name) return;
  if (userStore.isGuest) {
    userStore.setPostNameAction(() => roomStore.createRoom(name));
    userStore.showNameModal = true;
    return;
  }
  roomStore.createRoom(name);
  newRoomName.value = '';
};

const canDeleteRoom = (room) => !room.system && room.creatorPublicId && room.creatorPublicId === userStore.publicId;

const deleteRoom = (room) => {
  roomStore.deleteRoom(room.roomId);
};

const isScaleActive = (value) => Math.abs(uiStore.mainStageScale - value) < 0.011;

const isZoomActive = (value) => Math.abs(uiStore.globalZoomLevel - value) < 0.011;

const getInitials = (name = '') => {
  const normalized = String(name).trim();
  if (!normalized) return '?';
  const parts = normalized.split(/\s+/).filter(Boolean);
  if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
  return normalized.slice(0, 2).toUpperCase();
};
</script>

<style scoped>
.desktop-page-cover-blur {
  left: clamp(-120px, 5vw, 96px);
  top: 50%;
  width: min(78vw, 1180px);
  height: min(118vh, 1180px);
  transform: translateY(-50%) scale(1.08);
  background-position: center;
  background-size: cover;
  background-repeat: no-repeat;
  opacity: 0.22;
  filter: blur(78px) saturate(1.2) brightness(0.82);
  will-change: transform, opacity, filter;
  mask-image: radial-gradient(circle at 44% 50%, black 0%, black 42%, transparent 76%);
}

.desktop-page-cover-wash {
  background:
    radial-gradient(circle at 30% 48%, rgba(0, 0, 0, 0.04), transparent 34%),
    linear-gradient(90deg, transparent 0%, var(--surface-scrim) 74%, var(--surface-scrim) 100%);
}

.light .desktop-page-cover-blur {
  opacity: 0.14;
  filter: blur(82px) saturate(1.05) brightness(1.08);
}

.light .desktop-page-cover-wash {
  background:
    radial-gradient(circle at 30% 48%, rgba(255, 255, 255, 0.08), transparent 34%),
    linear-gradient(90deg, transparent 0%, rgba(255, 255, 255, 0.58) 72%, rgba(255, 255, 255, 0.76) 100%);
}

.desktop-cover-fade-enter-active,
.desktop-cover-fade-leave-active {
  transition: opacity 360ms ease, transform 360ms ease, filter 360ms ease;
}

.desktop-cover-fade-enter-from,
.desktop-cover-fade-leave-to {
  opacity: 0;
  transform: scale(1.02);
}
</style>

