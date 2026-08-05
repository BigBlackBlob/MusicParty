<template>
  <div class="h-[var(--app-height)] w-full relative flex flex-col overflow-hidden bg-bg-base text-text-primary font-body text-body selection:bg-primary selection:text-on-primary" style="--top-bar-height: 64px;">
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
      :now-playing="playerRuntime.nowPlaying"
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
              :aria-label="t('rooms.currentRoom', { name: currentRoomName })"
              :title="t('rooms.currentRoom', { name: currentRoomName })"
            >
              <span>{{ currentRoomName }}</span>
              <span class="material-symbols-outlined text-[18px]">expand_more</span>
            </button>
            <span
              class="h-2 w-2 rounded-full"
              :class="realtimeConnection.connected ? 'bg-[#22C55E] shadow-[0_0_14px_rgba(34,197,94,0.45)]' : 'bg-error'"
              :title="realtimeConnection.connected ? t('settings.connected') : t('settings.disconnected')"
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
                @click="attemptSwitchRoom(room)"
              >
                <div class="flex flex-col min-w-0 flex-1">
                  <span class="flex items-center gap-1 truncate font-compact text-sm font-bold">
                    <span v-if="room.privateRoom" class="material-symbols-outlined text-[14px] text-primary">lock</span>
                    <span class="truncate">{{ room.name }}</span>
                  </span>
                  <span class="text-[10px] opacity-60 font-mono tracking-tight">{{ room.onlineCount || 0 }} {{ t('settings.active') }}</span>
                </div>
                
                <div class="ml-3 flex items-center gap-2">
                  <span v-if="roomStore.currentRoomId === room.roomId" class="h-1.5 w-1.5 rounded-full bg-primary shadow-[0_0_8px_var(--primary)]"></span>
                  <button
                    v-if="canDeleteRoom(room)"
                    class="material-symbols-outlined text-[18px] opacity-0 group-hover:opacity-60 hover:!opacity-100 hover:text-error transition-all"
                    :aria-label="t('rooms.delete')"
                    :title="t('rooms.delete')"
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
            <label class="mt-2 flex items-center gap-2 text-xs text-text-secondary">
              <input v-model="newRoomPrivate" type="checkbox" class="h-4 w-4 accent-[var(--accent)]" />
              {{ t('rooms.createPrivate') }}
            </label>
            <input
              v-if="newRoomPrivate"
              v-model="newRoomPassword"
              type="password"
              class="mt-2 w-full rounded-md border border-border-default bg-surface-raised px-3 py-2 text-sm text-text-primary outline-none focus:border-primary"
              :placeholder="t('rooms.newRoomPassword')"
              autocomplete="new-password"
              @keyup.enter="createRoom"
            />
          </div>
          <button
            type="button"
            class="hidden min-w-[220px] cursor-pointer items-center gap-3 rounded-md border border-border-subtle bg-[var(--surface-control)] px-4 py-2 text-left transition-colors hover:bg-[var(--surface-control-hover)] focus-visible:ring-2 focus-visible:ring-[var(--accent-muted)] md:flex"
            :aria-label="t('search.searchAndAdd')"
            :title="t('search.searchAndAdd')"
            @click="handleSearchClick"
          >
            <span class="material-symbols-outlined text-text-muted text-[18px]">search</span>
            <span class="text-text-muted font-compact text-compact flex items-center tracking-tight">{{ t('search.placeholder') }}</span>
          </button>
        </div>
        <div class="flex flex-shrink-0 items-center gap-3">
          <div class="relative">
            <button
              type="button"
              class="mr-4 flex -space-x-2 items-center cursor-pointer rounded-md transition-transform hover:scale-105 focus-visible:ring-2 focus-visible:ring-[var(--accent-muted)]"
              :aria-label="t('settings.activeUsers')"
              :title="t('settings.activeUsers')"
              @click="toggleUserList"
            >
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
            </button>

            <div
              v-if="isUserListOpen"
              class="absolute right-4 top-12 z-50 w-[320px] overflow-y-auto rounded-lg border border-border-default bg-surface-overlay/95 shadow-lg backdrop-blur-xl"
              style="max-height: min(520px, calc(var(--app-height) - 96px));"
            >
              <UserList />
            </div>
          </div>

          <button @click="uiStore.toggleDarkMode" class="flex h-10 w-10 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-text-primary" :aria-label="t('settings.toggleTheme')" :title="t('settings.toggleTheme')">
            <span class="material-symbols-outlined">{{ uiStore.isDarkMode ? 'light_mode' : 'dark_mode' }}</span>
          </button>

          <button
            @click="layoutStore.enterEditMode"
            class="flex h-10 w-10 items-center justify-center rounded-md transition-all hover:bg-[var(--surface-control-hover)] active:scale-95"
            :class="layoutStore.isEditMode ? 'text-primary bg-primary/10' : 'text-text-secondary hover:text-text-primary'"
            :aria-label="t('layout.editLayout')"
            :title="t('layout.editLayout')"
          >
            <span class="material-symbols-outlined">grid_view</span>
          </button>

          <div class="relative">
            <button @click="toggleSettings" class="flex h-10 w-10 items-center justify-center rounded-md text-text-secondary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-text-primary" :aria-label="t('settings.title')" :title="t('settings.title')">
              <span class="material-symbols-outlined">settings</span>
            </button>

          </div>
        </div>
      </header>
      <SettingsCenter v-if="isSettingsOpen" @close="isSettingsOpen = false" />
      <PrivateRoomAccessDialog
        :open="privateRoomDialogOpen"
        :room="privateRoomDialogRoom"
        :loading="privateRoomDialogLoading"
        :error="privateRoomDialogError"
        @submit="submitPrivateRoomPassword"
        @cancel="closePrivateRoomDialog"
      />

      <!-- Main Immersive Canvas -->
      <main class="relative z-20 flex w-full items-center justify-center px-5 pt-[var(--top-bar-height)]" style="height: var(--app-height);">
        <div
          class="flex w-full min-h-0 items-stretch overflow-visible transition-all duration-300"
          :style="{
            '--stage-scale': uiStore.mainStageScale,
            '--stage-height': 'calc(var(--app-height) / var(--global-zoom) - var(--top-bar-height) / var(--global-zoom) - 40px)',
            '--global-zoom': uiStore.globalZoomLevel,
            maxWidth: `min(calc(1520px * ${uiStore.mainStageScale}), calc((100% - 40px) / ${uiStore.globalZoomLevel}))`,
            height: `min(var(--stage-height), calc(708px * ${uiStore.mainStageScale}))`,
            transform: `scale(${uiStore.globalZoomLevel})`,
            transformOrigin: 'center center'
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
import SettingsCenter from '../SettingsCenter.vue';
import PrivateRoomAccessDialog from '../PrivateRoomAccessDialog.vue';
import LiteModeView from './LiteModeView.vue';
import UserList from '../UserList.vue';
import { useUserStore } from '../../stores/user';
import { useUiStore } from '../../stores/ui';
import { ROOM_SWITCH_PASSWORD_REQUIRED, useRoomRealtimeCoordinator } from '../../domains/realtime/roomRealtimeCoordinator';
import { useRoomRuntimeStore } from '../../domains/realtime/roomRuntimeStore';
import { useRoomPresenceStore } from '../../domains/realtime/roomPresenceStore';
import { useRealtimeConnectionStore } from '../../domains/realtime/realtimeConnectionStore';
import { useRoomStore } from '../../stores/room';
import { useLayoutStore } from '../../stores/layout';
import { extractErrorMessage } from '../../utils/errors';
import { useToast } from '../../composables/useToast';

const emit = defineEmits(['search', 'toggle-mobile-chat']);
const { t } = useI18n();
const userStore = useUserStore();
const uiStore = useUiStore();
const playerRuntime = useRoomRuntimeStore();
const presenceStore = useRoomPresenceStore();
const realtimeConnection = useRealtimeConnectionStore();
const realtimeCoordinator = useRoomRealtimeCoordinator();
const roomStore = useRoomStore();
const layoutStore = useLayoutStore();
const toast = useToast();


const isSettingsOpen = ref(false);
const isUserListOpen = ref(false);
const isRoomMenuOpen = ref(false);
const newRoomName = ref('');
const newRoomPrivate = ref(false);
const newRoomPassword = ref('');
const privateRoomDialogOpen = ref(false);
const privateRoomDialogRoom = ref(null);
const privateRoomDialogLoading = ref(false);
const privateRoomDialogError = ref('');
const currentMusic = computed(() => playerRuntime.nowPlaying?.music || null);
const currentCover = computed(() => currentMusic.value?.coverUrl || '');
const currentRoomName = computed(() => roomStore.currentRoom?.name || t('app.lounge'));
const visibleUsers = computed(() => presenceStore.users.slice(0, 3));
const extraUserCount = computed(() => Math.max(0, presenceStore.count - visibleUsers.value.length));

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

const openPrivateRoomDialog = (room) => {
  privateRoomDialogRoom.value = room;
  privateRoomDialogError.value = '';
  privateRoomDialogOpen.value = true;
};

const closePrivateRoomDialog = () => {
  privateRoomDialogOpen.value = false;
  privateRoomDialogRoom.value = null;
  privateRoomDialogError.value = '';
  privateRoomDialogLoading.value = false;
};

const attemptSwitchRoom = async (room) => {
  if (!room?.roomId) return;
  try {
    await realtimeCoordinator.switchRoom(room.roomId);
    isRoomMenuOpen.value = false;
  } catch (error) {
    if (error?.code === ROOM_SWITCH_PASSWORD_REQUIRED) {
      openPrivateRoomDialog(room);
      return;
    }
    toast.error(extractErrorMessage(error, 'Could not switch Lounge'));
  }
};

const submitPrivateRoomPassword = async (password) => {
  const room = privateRoomDialogRoom.value;
  if (!room?.roomId) return;
  privateRoomDialogLoading.value = true;
  privateRoomDialogError.value = '';
  try {
    await realtimeCoordinator.switchRoom(room.roomId, password);
    closePrivateRoomDialog();
    isRoomMenuOpen.value = false;
  } catch (error) {
    privateRoomDialogError.value = extractErrorMessage(error, 'Room password could not be verified');
  } finally {
    privateRoomDialogLoading.value = false;
  }
};

const createRoom = () => {
  const name = newRoomName.value.trim();
  const password = newRoomPassword.value;
  if (!name || (newRoomPrivate.value && !password.trim())) return;
  if (userStore.isGuest) {
    userStore.setPostNameAction(() => roomStore.createRoom(name, { isPrivate: newRoomPrivate.value, password }));
    userStore.showNameModal = true;
    return;
  }
  roomStore.createRoom(name, { isPrivate: newRoomPrivate.value, password });
  newRoomName.value = '';
  newRoomPrivate.value = false;
  newRoomPassword.value = '';
};

const canDeleteRoom = (room) => !room.system && userStore.capabilitiesForRoom(room.creatorPublicId).canManageCurrentRoom;

const deleteRoom = (room) => {
  roomStore.deleteRoom(room.roomId);
};

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

