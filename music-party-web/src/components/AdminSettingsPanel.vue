<template>
  <div class="border-t border-border-subtle pt-3">
    <button
      type="button"
      class="flex w-full items-center justify-between rounded-md px-2 py-2 text-left hover:bg-[var(--surface-control-hover)]"
      @click="expanded = !expanded"
    >
      <span class="text-text-secondary">{{ t('settings.admin.title') }}</span>
      <span class="material-symbols-outlined text-[18px] text-text-muted">{{ expanded ? 'expand_less' : 'expand_more' }}</span>
    </button>

    <div v-if="expanded" class="space-y-3 px-2 pb-1 pt-2">
      <div class="space-y-2">
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          class="w-full rounded-md border border-border-default bg-[var(--surface-control)] px-3 py-2 text-sm text-text-primary outline-none transition-colors placeholder:text-text-tertiary focus:border-primary"
          :placeholder="t('settings.admin.passwordPlaceholder')"
        />
        <input
          v-model="targetUser"
          type="text"
          class="w-full rounded-md border border-border-default bg-[var(--surface-control)] px-3 py-2 text-sm text-text-primary outline-none transition-colors placeholder:text-text-tertiary focus:border-primary"
          :placeholder="t('settings.admin.userPlaceholder')"
        />
      </div>

      <div class="grid grid-cols-2 gap-2">
        <button class="admin-action" type="button" :disabled="busy" @click="runGrantNavidrome">
          {{ t('settings.admin.grantNavidrome') }}
        </button>
        <button class="admin-action" type="button" :disabled="busy" @click="runRevokeNavidrome">
          {{ t('settings.admin.revokeNavidrome') }}
        </button>
        <button class="admin-action" type="button" :disabled="busy" @click="runStream(true)">
          {{ t('settings.admin.streamOn') }}
        </button>
        <button class="admin-action" type="button" :disabled="busy" @click="runStream(false)">
          {{ t('settings.admin.streamOff') }}
        </button>
        <button class="admin-action" type="button" :disabled="busy" @click="runClearQueue">
          {{ t('settings.admin.clearQueue') }}
        </button>
        <button class="admin-action" type="button" :disabled="busy" @click="runClearChat">
          {{ t('settings.admin.clearChat') }}
        </button>
      </div>

      <div class="space-y-2 border-t border-border-subtle pt-3">
        <button
          type="button"
          class="flex w-full items-center justify-between rounded-md px-0 py-1 text-left"
          @click="toggleCustomNavidrome"
        >
          <span class="text-sm font-bold text-text-secondary">{{ t('settings.admin.customNavidrome') }}</span>
          <span class="material-symbols-outlined text-[18px] text-text-muted">{{ customNavidromeOpen ? 'expand_less' : 'expand_more' }}</span>
        </button>

        <div v-if="customNavidromeOpen" class="space-y-2">
          <div class="space-y-1">
            <div class="flex items-center justify-between gap-2">
              <span class="text-xs font-bold text-text-tertiary">{{ t('settings.admin.sourceList') }}</span>
              <button class="admin-link-action" type="button" :disabled="busy || loadingSources" @click="loadSubsonicSources">
                {{ loadingSources ? t('settings.admin.loadingSources') : t('common.refresh') }}
              </button>
            </div>
            <div v-if="subsonicSources.length" class="space-y-1">
              <button
                v-for="source in subsonicSources"
                :key="source.id"
                type="button"
                class="source-row"
                :class="{ 'source-row-active': source.id === navidromeForm.id }"
                @click="editSource(source)"
              >
                <span class="min-w-0">
                  <span class="block truncate text-xs font-bold text-text-secondary">{{ source.label }}</span>
                  <span class="block truncate text-[11px] text-text-tertiary">{{ source.id }} · {{ source.username || '-' }}</span>
                </span>
                <span class="source-state" :class="{ 'source-state-off': !source.active }">
                  {{ source.active ? t('settings.admin.sourceActive') : t('settings.admin.sourceInactive') }}
                </span>
              </button>
            </div>
            <div v-else class="rounded-md border border-border-subtle px-3 py-2 text-xs text-text-tertiary">
              {{ t('settings.admin.noSources') }}
            </div>
          </div>

          <div class="grid grid-cols-2 gap-2">
            <input
              v-model="navidromeForm.id"
              type="text"
              class="admin-input"
              :placeholder="t('settings.admin.sourceIdPlaceholder')"
            />
            <input
              v-model="navidromeForm.label"
              type="text"
              class="admin-input"
              :placeholder="t('settings.admin.sourceLabelPlaceholder')"
            />
          </div>
          <div class="grid grid-cols-[1fr_74px] gap-2">
            <input
              v-model="navidromeForm.host"
              type="text"
              class="admin-input"
              :placeholder="t('settings.admin.navidromeHostPlaceholder')"
            />
            <input
              v-model="navidromeForm.port"
              type="number"
              min="1"
              max="65535"
              class="admin-input"
              :placeholder="t('settings.admin.navidromePortPlaceholder')"
            />
          </div>
          <select v-model="navidromeForm.scheme" class="admin-input">
            <option value="https">HTTPS</option>
            <option value="http">HTTP</option>
          </select>
          <div class="grid grid-cols-2 gap-2">
            <input
              v-model="navidromeForm.username"
              type="text"
              class="admin-input"
              :placeholder="t('settings.admin.navidromeUsernamePlaceholder')"
            />
            <input
              v-model="navidromeForm.password"
              type="password"
              class="admin-input"
              :placeholder="t('settings.admin.navidromePasswordPlaceholder')"
            />
          </div>
          <input
            v-model="navidromeForm.allowedUsers"
            type="text"
            class="admin-input"
            :placeholder="t('settings.admin.allowedUsersPlaceholder')"
          />
          <div class="grid grid-cols-2 gap-2">
            <button class="admin-action" type="button" :disabled="busy" @click="runSaveCustomNavidrome">
              {{ t('settings.admin.saveSource') }}
            </button>
            <button class="admin-action" type="button" :disabled="busy || !navidromeForm.id.trim()" @click="runTestCustomNavidrome">
              {{ t('settings.admin.testSource') }}
            </button>
            <button class="admin-action" type="button" :disabled="busy" @click="resetNavidromeForm">
              {{ t('settings.admin.newSource') }}
            </button>
            <button class="admin-action" type="button" :disabled="busy || !navidromeForm.id.trim()" @click="runRemoveCustomNavidrome">
              {{ t('settings.admin.removeSource') }}
            </button>
          </div>
        </div>
      </div>

      <div class="space-y-2 border-t border-border-subtle pt-3">
        <input
          v-model="customCommand"
          type="text"
          class="w-full rounded-md border border-border-default bg-[var(--surface-control)] px-3 py-2 text-sm text-text-primary outline-none transition-colors placeholder:text-text-tertiary focus:border-primary"
          :placeholder="t('settings.admin.commandPlaceholder')"
          @keyup.enter="runCustomCommand"
        />
        <button
          type="button"
          class="w-full rounded-md bg-primary px-3 py-2 text-xs font-bold text-on-primary transition-colors hover:bg-[var(--accent-hover)] disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="busy || !customCommand.trim()"
          @click="runCustomCommand"
        >
          {{ busy ? t('settings.admin.running') : t('settings.admin.runCommand') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '../api/auth';
import { useToast } from '../composables/useToast';
import { useMusicStore } from '../stores/music';
import { useRoomStore } from '../stores/room';
import { useUserStore } from '../stores/user';
import { extractErrorMessage } from '../utils/errors';

const { t } = useI18n();
const { success, error } = useToast();
const musicStore = useMusicStore();
const roomStore = useRoomStore();
const userStore = useUserStore();

const expanded = ref(false);
const password = ref('');
const targetUser = ref(userStore.currentUser?.name || '');
const customCommand = ref('');
const customNavidromeOpen = ref(false);
const defaultNavidromeForm = () => ({
  id: 'navidrome-custom',
  label: 'Navidrome',
  baseUrl: '',
  scheme: 'https',
  host: '',
  port: '443',
  username: '',
  password: '',
  allowedUsers: '*',
  enabled: true
});
const navidromeForm = ref(defaultNavidromeForm());
const subsonicSources = ref([]);
const loadingSources = ref(false);
const busy = ref(false);

const currentTargetUser = () => (targetUser.value || userStore.currentUser?.name || '').trim();

const runAdminAction = async (action, successMessage = t('settings.admin.commandExecuted')) => {
  if (!password.value) {
    error(t('settings.admin.passwordRequired'));
    return;
  }
  busy.value = true;
  try {
    await action();
    success(successMessage);
    await musicStore.refreshPlatforms();
  } catch (e) {
    error(extractErrorMessage(e, t('settings.admin.commandFailed')));
  } finally {
    busy.value = false;
  }
};

const loadSubsonicSources = async () => {
  if (!password.value) {
    error(t('settings.admin.passwordRequired'));
    return;
  }
  loadingSources.value = true;
  try {
    const { data } = await authApi.listSubsonicSources(password.value, roomStore.currentRoomId);
    subsonicSources.value = Array.isArray(data) ? data : [];
  } catch (e) {
    error(extractErrorMessage(e, t('settings.admin.sourcesLoadFailed')));
  } finally {
    loadingSources.value = false;
  }
};

const toggleCustomNavidrome = async () => {
  customNavidromeOpen.value = !customNavidromeOpen.value;
  if (customNavidromeOpen.value && password.value) {
    await loadSubsonicSources();
  }
};

const editSource = (source) => {
  const parsed = parseSourceUrl(source.baseUrl || '');
  navidromeForm.value = {
    id: source.id,
    label: source.label || source.id,
    baseUrl: source.baseUrl || '',
    scheme: parsed.scheme,
    host: parsed.host,
    port: parsed.port,
    username: source.username || '',
    password: '',
    allowedUsers: source.allowedUsers || '*',
    enabled: source.enabled !== false
  };
};

const resetNavidromeForm = () => {
  navidromeForm.value = defaultNavidromeForm();
};

const parseSourceUrl = (value) => {
  const fallback = { scheme: 'https', host: '', port: '443' };
  if (!value) return fallback;
  const normalized = /^[a-z][a-z0-9+.-]*:\/\//i.test(value) ? value : `https://${value}`;
  try {
    const url = new URL(normalized);
    return {
      scheme: url.protocol.replace(':', '') || fallback.scheme,
      host: url.hostname || fallback.host,
      port: url.port || (url.protocol === 'http:' ? '80' : '443')
    };
  } catch {
    return fallback;
  }
};

const buildSourceBaseUrl = () => {
  const host = navidromeForm.value.host.trim();
  const port = String(navidromeForm.value.port || '').trim();
  const scheme = navidromeForm.value.scheme === 'http' ? 'http' : 'https';
  if (!host) return navidromeForm.value.baseUrl.trim();
  return `${scheme}://${host}${port ? `:${port}` : ''}`;
};

const runGrantNavidrome = () => runAdminAction(
  () => authApi.grantNavidrome(password.value, currentTargetUser(), roomStore.currentRoomId),
  t('settings.admin.navidromeGranted')
);

const runRevokeNavidrome = () => runAdminAction(
  () => authApi.revokeNavidrome(password.value, currentTargetUser(), roomStore.currentRoomId),
  t('settings.admin.navidromeRevoked')
);

const runStream = (enabled) => runAdminAction(
  () => authApi.setStreamEnabled(password.value, enabled, roomStore.currentRoomId),
  enabled ? t('settings.admin.streamEnabled') : t('settings.admin.streamDisabled')
);

const runClearQueue = () => runAdminAction(
  () => authApi.clearQueue(password.value, roomStore.currentRoomId),
  t('settings.admin.queueCleared')
);

const runClearChat = () => runAdminAction(
  () => authApi.clearChat(password.value, roomStore.currentRoomId),
  t('settings.admin.chatCleared')
);

const runSaveCustomNavidrome = () => runAdminAction(
  async () => {
    await authApi.saveSubsonicSource(password.value, roomStore.currentRoomId, {
      id: navidromeForm.value.id.trim(),
      label: navidromeForm.value.label.trim() || navidromeForm.value.id.trim(),
      baseUrl: buildSourceBaseUrl(),
      username: navidromeForm.value.username.trim(),
      password: navidromeForm.value.password,
      allowedUsers: navidromeForm.value.allowedUsers.trim() || '*',
      enabled: navidromeForm.value.enabled
    });
    navidromeForm.value.password = '';
    await loadSubsonicSources();
  },
  t('settings.admin.sourceSaved')
);

const runRemoveCustomNavidrome = () => runAdminAction(
  async () => {
    await authApi.removeSubsonicSource(password.value, roomStore.currentRoomId, navidromeForm.value.id.trim());
    resetNavidromeForm();
    await loadSubsonicSources();
  },
  t('settings.admin.sourceRemoved')
);

const runTestCustomNavidrome = () => runAdminAction(
  () => authApi.testSubsonicSource(password.value, roomStore.currentRoomId, navidromeForm.value.id.trim()),
  t('settings.admin.sourceTested')
);

const runCustomCommand = () => runAdminAction(async () => {
  const command = customCommand.value.trim();
  if (!command) return;
  await authApi.adminCommand(password.value, command, roomStore.currentRoomId);
  customCommand.value = '';
});
</script>

<style scoped>
.admin-action {
  min-height: 34px;
  border-radius: 6px;
  border: 1px solid var(--border-default);
  background: var(--surface-control);
  padding: 7px 8px;
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  line-height: 1.2;
  transition: border-color 140ms ease, color 140ms ease, background 140ms ease;
}

.admin-action:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--text-primary);
  background: var(--surface-control-hover);
}

.admin-action:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.admin-input {
  width: 100%;
  border-radius: 6px;
  border: 1px solid var(--border-default);
  background: var(--surface-control);
  padding: 8px 10px;
  color: var(--text-primary);
  font-size: 13px;
  outline: none;
  transition: border-color 140ms ease, background 140ms ease;
}

.admin-input::placeholder {
  color: var(--text-tertiary);
}

.admin-input:focus {
  border-color: var(--accent);
}

.admin-link-action {
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 700;
  transition: color 140ms ease;
}

.admin-link-action:hover:not(:disabled) {
  color: var(--text-primary);
}

.admin-link-action:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.source-row {
  display: flex;
  min-height: 42px;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
  background: var(--surface-control);
  padding: 7px 8px;
  text-align: left;
  transition: border-color 140ms ease, background 140ms ease;
}

.source-row:hover,
.source-row-active {
  border-color: var(--border-default);
  background: var(--surface-control-hover);
}

.source-state {
  flex: 0 0 auto;
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
}

.source-state-off {
  color: var(--text-tertiary);
}
</style>
