<template>
  <div class="personal-info">
    <header class="personal-info__header">
      <div>
        <p class="personal-info__eyebrow">{{ t('settings.account.kicker') }}</p>
        <h3 class="personal-info__title">{{ t('settings.account.title') }}</h3>
      </div>
      <span class="personal-info__role" :class="{ 'personal-info__role--admin': userStore.accountType === 'admin' }">
        {{ roleLabel }}
      </span>
    </header>

    <div v-if="!userStore.isSignedIn" class="personal-info__empty">
      <span class="material-symbols-outlined">lock</span>
      <strong>{{ t('settings.account.signedOut') }}</strong>
      <p>{{ t('settings.account.signedOutDesc') }}</p>
    </div>

    <template v-else>
      <section class="personal-info__panel">
        <div class="personal-info__avatar">{{ initials }}</div>
        <div class="personal-info__identity">
          <strong>{{ userStore.currentUser.name }}</strong>
          <span>{{ accountUsername }}</span>
        </div>
        <dl class="personal-info__facts">
          <div>
            <dt>{{ t('settings.account.publicId') }}</dt>
            <dd>{{ userStore.publicId }}</dd>
          </div>
          <div>
            <dt>{{ t('settings.account.lastLogin') }}</dt>
            <dd>{{ formattedLastLogin }}</dd>
          </div>
        </dl>
      </section>

      <section class="personal-info__section">
        <h4>{{ t('settings.account.profile') }}</h4>
        <form class="personal-info__form" @submit.prevent="saveProfile">
          <label for="account-display-name">
            <span>{{ t('settings.account.displayName') }}</span>
            <input
              id="account-display-name"
              v-model.trim="displayName"
              type="text"
              maxlength="32"
              required
              autocomplete="nickname"
              :aria-invalid="Boolean(profileError)"
              aria-describedby="account-profile-status"
            />
          </label>
          <button type="submit" :disabled="savingProfile || !displayName">
            {{ savingProfile ? t('settings.account.saving') : t('settings.account.saveProfile') }}
          </button>
        </form>
        <p
          id="account-profile-status"
          class="personal-info__status"
          :class="{ 'personal-info__status--error': profileError }"
          aria-live="polite"
        >
          {{ profileError || t('settings.account.displayNameHint') }}
        </p>
      </section>

      <section class="personal-info__section">
        <h4>{{ t('settings.account.security') }}</h4>
        <p class="personal-info__access-copy">{{ t(securityAccessCopyKey) }}</p>
        <p class="personal-info__status">{{ t('settings.account.browserSession') }}</p>
        <div class="personal-info__actions">
          <button
            type="button"
            class="personal-info__secondary"
            :class="{ 'personal-info__secondary--danger': confirmingLogout }"
            @click="requestLogout"
          >
            {{ confirmingLogout ? t('settings.account.confirmLogout') : t('settings.account.logout') }}
          </button>
        </div>
      </section>

      <section class="personal-info__stats" :aria-label="t('settings.account.personalDataSummary')">
        <div class="personal-info__stat">
          <span class="material-symbols-outlined">favorite</span>
          <div>
            <strong>{{ playerStore.likedSongs.length }}</strong>
            <small>{{ t('settings.account.likedSongs') }}</small>
          </div>
        </div>
        <div class="personal-info__stat">
          <span class="material-symbols-outlined">queue_music</span>
          <div>
            <strong>{{ userPlaylistsStore.playlists.length }}</strong>
            <small>{{ t('settings.account.personalPlaylists') }}</small>
          </div>
        </div>
        <div class="personal-info__stat">
          <span class="material-symbols-outlined">link</span>
          <div>
            <strong>{{ bindingEntries.length }}</strong>
            <small>{{ t('settings.account.platformBindings') }}</small>
          </div>
        </div>
      </section>

      <section class="personal-info__section">
        <h4>{{ t('settings.account.bindings') }}</h4>
        <div v-if="bindingEntries.length" class="personal-info__bindings">
          <span v-for="[platform, accountId] in bindingEntries" :key="platform" class="personal-info__binding">
            <strong>{{ platform }}</strong>
            <small>{{ accountId }}</small>
          </span>
        </div>
        <p v-else class="personal-info__muted">{{ t('settings.account.noBindings') }}</p>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useToast } from '../composables/useToast';
import { useLikedSongsStore } from '../domains/playback/likedSongs';
import { useUserStore } from '../stores/user';
import { useUserPlaylistsStore } from '../stores/userPlaylists';
import { useRoomRealtimeCoordinator } from '../domains/realtime/roomRealtimeCoordinator';
import { extractErrorMessage } from '../utils/errors';

const emit = defineEmits(['logged-out']);

const { t } = useI18n();
const { success, error } = useToast();
const userStore = useUserStore();
const playerStore = useLikedSongsStore();
const userPlaylistsStore = useUserPlaylistsStore();
const realtimeCoordinator = useRoomRealtimeCoordinator();

const displayName = ref(userStore.currentUser.name || '');
const savingProfile = ref(false);
const confirmingLogout = ref(false);
const profileError = ref('');

const roleLabel = computed(() => userStore.accountType === 'admin' ? t('settings.account.adminRole') : t('settings.account.userRole'));
const securityAccessCopyKey = computed(() => {
  if (userStore.accountType === 'admin') return 'settings.account.adminPasswordAccess';
  if (userStore.accountType === 'guest') return 'settings.account.guestAccess';
  return 'settings.account.inviteMemberAccess';
});
const accountUsername = computed(() => userStore.currentUser.name);
const bindingEntries = computed(() => Object.entries(userStore.bindings || {}).filter(([, value]) => value));
const initials = computed(() => {
  const name = userStore.currentUser.name || accountUsername.value || '';
  return name.trim().slice(0, 2).toUpperCase() || '?';
});
const formattedLastLogin = computed(() => {
  const value = userStore.accountLastLoginAt;
  return value ? new Date(value).toLocaleString() : t('settings.account.notAvailable');
});
onMounted(() => {
  if (!userStore.isSignedIn) return;
  userStore.refreshAccount()
    .then(session => {
      if (session?.displayName) displayName.value = session.displayName;
    })
    .catch(() => {});
  userPlaylistsStore.loadPlaylists().catch(() => {});
});

const saveProfile = async () => {
  profileError.value = '';
  if (!displayName.value) {
    profileError.value = t('settings.account.displayNameRequired');
    return;
  }
  savingProfile.value = true;
  try {
    const session = await userStore.updateProfile(displayName.value);
    displayName.value = session.displayName || userStore.currentUser.name;
    success(t('settings.account.profileSaved'));
  } catch (e) {
    profileError.value = extractErrorMessage(e, t('settings.account.profileSaveFailed'));
    error(profileError.value);
  } finally {
    savingProfile.value = false;
  }
};

const requestLogout = async () => {
  if (!confirmingLogout.value) {
    confirmingLogout.value = true;
    window.setTimeout(() => {
      confirmingLogout.value = false;
    }, 3500);
    return;
  }
  await logout();
};

const logout = async () => {
  realtimeCoordinator.disconnectForLogout();
  await userStore.logout();
  success(t('settings.account.loggedOut'));
  emit('logged-out');
};
</script>

<style scoped>
.personal-info {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.personal-info__header,
.personal-info__panel,
.personal-info__section,
.personal-info__empty,
.personal-info__stats {
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--surface-control);
}

.personal-info__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px;
}

.personal-info__eyebrow {
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--text-tertiary);
}

.personal-info__title {
  font-size: 18px;
  font-weight: 900;
  color: var(--text-primary);
}

.personal-info__role {
  flex: 0 0 auto;
  border-radius: 999px;
  padding: 6px 10px;
  background: var(--surface-raised);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 900;
  text-transform: uppercase;
  transition: background 160ms var(--ease-out, ease), color 160ms var(--ease-out, ease);
}

.personal-info__role--admin {
  background: var(--primary);
  color: var(--on-primary);
}

.personal-info__empty {
  display: grid;
  place-items: center;
  gap: 6px;
  padding: 42px 18px;
  text-align: center;
  color: var(--text-secondary);
}

.personal-info__empty strong {
  color: var(--text-primary);
}

.personal-info__panel {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 12px;
  padding: 14px;
}

.personal-info__avatar {
  display: grid;
  width: 52px;
  height: 52px;
  place-items: center;
  border-radius: 8px;
  background: var(--surface-raised);
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 900;
}

.personal-info__identity {
  min-width: 0;
}

.personal-info__identity strong,
.personal-info__identity span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.personal-info__identity strong {
  color: var(--text-primary);
  font-size: 17px;
  font-weight: 900;
}

.personal-info__identity span {
  color: var(--text-tertiary);
  font-size: 12px;
}

.personal-info__facts {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin: 4px 0 0;
}

.personal-info__facts div {
  min-width: 0;
  border-radius: 8px;
  background: var(--surface-raised);
  padding: 10px;
}

.personal-info__facts dt,
.personal-info__summary small,
.personal-info__muted {
  color: var(--text-tertiary);
  font-size: 11px;
}

.personal-info__facts dd {
  overflow: hidden;
  margin: 2px 0 0;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 800;
}

.personal-info__section {
  padding: 14px;
}

.personal-info__section h4 {
  margin-bottom: 10px;
  color: var(--text-primary);
  font-size: 14px;
  font-weight: 900;
}

.personal-info__form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
}

.personal-info__form--password {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.personal-info__form label {
  min-width: 0;
}

.personal-info__form span {
  display: block;
  margin-bottom: 6px;
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 800;
}

.personal-info__form input {
  width: 100%;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--surface-panel);
  padding: 10px 12px;
  color: var(--text-primary);
}

.personal-info__form input:focus-visible,
.personal-info__form button:focus-visible,
.personal-info__secondary:focus-visible {
  outline: none;
  border-color: var(--border-accent, var(--accent));
  box-shadow: 0 0 0 3px var(--focus-ring);
}

.personal-info__form input[aria-invalid="true"] {
  border-color: var(--error);
}

.personal-info__form button,
.personal-info__secondary {
  align-self: end;
  min-height: 40px;
  border: 1px solid transparent;
  border-radius: 8px;
  padding: 10px 14px;
  background: var(--primary);
  color: var(--on-primary);
  font-size: 12px;
  font-weight: 900;
  line-height: 1;
  transition: background 160ms var(--ease-out, ease), color 160ms var(--ease-out, ease), opacity 160ms var(--ease-out, ease), transform 120ms var(--ease-out, ease);
}

.personal-info__form button:hover:not(:disabled) {
  background: var(--accent-hover);
}

.personal-info__form button:active:not(:disabled),
.personal-info__secondary:active:not(:disabled) {
  transform: translateY(1px);
}

.personal-info__status {
  margin-top: 8px;
  color: var(--text-tertiary);
  font-size: 11px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.personal-info__status--error {
  color: var(--error-soft-text, var(--error));
}

.personal-info__form button:disabled {
  opacity: 0.55;
}

.personal-info__actions {
  display: flex;
  align-items: end;
  gap: 8px;
}

.personal-info__access-copy {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

/* Keep the security actions explicit: --primary is not a theme token. */
.personal-info__actions > .personal-info__secondary {
  border-color: var(--border-default);
  background: var(--surface-2);
  color: var(--text-primary);
}

.personal-info__secondary {
  border-color: var(--border-default);
  background: var(--surface-2);
  color: var(--text-primary);
  box-shadow: inset 0 1px 0 color-mix(in srgb, var(--text-primary) 8%, transparent);
  /* 增加字体粗细和字号提升可读性 */
  font-weight: 800;
  font-size: 13px;
}

.personal-info__secondary:hover {
  background: var(--surface-1);
  color: var(--text-primary);
  border-color: var(--border-hover);
  font-weight: 900;
}

.personal-info__secondary--danger {
  /* 使用更高对比度的错误色 */
  border-color: var(--error);
  background: var(--error-soft-bg);
  color: var(--error-text);
  font-weight: 900;
}

.personal-info__secondary--danger:hover {
  background: var(--error-bg);
  color: var(--error-text);
  border-color: var(--error);
}

.personal-info__stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 10px 12px;
}

.personal-info__stat {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
  padding: 4px 10px;
}

.personal-info__stat + .personal-info__stat {
  border-inline-start: 1px solid var(--border-subtle);
}

.personal-info__stat span {
  flex: 0 0 auto;
  color: var(--primary);
  font-size: 20px;
}

.personal-info__stat div {
  min-width: 0;
}

.personal-info__stat strong,
.personal-info__stat small {
  display: block;
}

.personal-info__stat strong {
  color: var(--text-primary);
  font-size: 18px;
  font-weight: 900;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}

.personal-info__stat small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.personal-info__bindings {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.personal-info__binding {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 8px;
  border-radius: 8px;
  background: var(--surface-raised);
  padding: 8px 10px;
}

.personal-info__binding strong {
  color: var(--text-primary);
  font-size: 12px;
}

.personal-info__binding small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-tertiary);
}

@media (max-width: 720px) {
  .personal-info__facts,
  .personal-info__form,
  .personal-info__form--password,
  .personal-info__stats {
    grid-template-columns: 1fr;
  }

  .personal-info__stat {
    padding: 8px 2px;
  }

  .personal-info__stat + .personal-info__stat {
    border-inline-start: 0;
    border-top: 1px solid var(--border-subtle);
  }

  .personal-info__actions {
    align-items: stretch;
    flex-direction: column;
  }

  .personal-info__form input,
  .personal-info__form button,
  .personal-info__secondary {
    min-height: 44px;
  }
}

@media (min-width: 721px) and (max-width: 980px) {
  .personal-info__form--password {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .personal-info__actions {
    grid-column: 1 / -1;
  }
}
</style>
