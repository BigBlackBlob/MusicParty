<template>
  <div class="source-manager">
    <button type="button" class="source-manager-toggle" @click="open = !open">
      <span>{{ t('settings.admin.sourceManager') }}</span>
      <span class="material-symbols-outlined text-[18px]">{{ open ? 'expand_less' : 'expand_more' }}</span>
    </button>

    <div v-if="open" class="source-manager-body">
      <input
        v-model="password"
        type="password"
        autocomplete="current-password"
        class="source-input"
        :placeholder="t('settings.admin.passwordPlaceholder')"
        @keyup.enter="unlockAdmin"
      />

      <div v-if="!adminUnlocked" class="flex items-center justify-between gap-2">
        <button type="button" class="source-action primary" :disabled="busy || loadingSources" @click="unlockAdmin">
          {{ loadingSources ? t('settings.admin.loadingSources') : t('settings.admin.unlockSourceManager') }}
        </button>
      </div>

      <div v-if="adminUnlocked" class="flex items-center justify-between gap-2">
        <button type="button" class="source-action" :disabled="busy || loadingSources" @click="loadSources">
          {{ loadingSources ? t('settings.admin.loadingSources') : t('common.refresh') }}
        </button>
        <button type="button" class="source-action" :disabled="busy" @click="resetForm">
          {{ t('settings.admin.newSource') }}
        </button>
      </div>

      <div v-if="adminUnlocked" class="space-y-1">
        <div v-if="sources.length === 0" class="source-empty">{{ t('settings.admin.noSources') }}</div>
        <div v-for="(source, index) in sources" :key="source.id" class="source-item" :class="{ selected: source.id === form.id }">
          <button type="button" class="source-main" @click="editSource(source)">
            <span class="min-w-0">
              <span class="block truncate text-[12px] font-bold text-text-primary">{{ source.label }}</span>
              <span class="block truncate text-[11px] text-text-tertiary">{{ source.id }} · {{ source.username || '-' }}</span>
            </span>
            <span class="source-status" :class="{ off: !source.active }">{{ source.active ? t('settings.admin.sourceActive') : t('settings.admin.sourceInactive') }}</span>
          </button>
          <div class="source-row-actions">
            <button type="button" class="source-remove" :disabled="busy" @click.stop="removeSourceById(source.id)" :title="t('settings.admin.removeSource')">
              <span class="material-symbols-outlined">close</span>
            </button>
            <button type="button" :disabled="busy || index === 0" @click="moveSource(index, -1)" :title="t('settings.admin.moveUp')">
              <span class="material-symbols-outlined">keyboard_arrow_up</span>
            </button>
            <button type="button" :disabled="busy || index === sources.length - 1" @click="moveSource(index, 1)" :title="t('settings.admin.moveDown')">
              <span class="material-symbols-outlined">keyboard_arrow_down</span>
            </button>
          </div>
        </div>
      </div>

      <div v-if="adminUnlocked" class="space-y-2 border-t border-border-subtle pt-2">
        <div class="grid grid-cols-2 gap-2">
          <input v-model="form.id" type="text" class="source-input" :placeholder="t('settings.admin.sourceIdPlaceholder')" />
          <input v-model="form.label" type="text" class="source-input" :placeholder="t('settings.admin.sourceLabelPlaceholder')" />
        </div>
        <div class="grid grid-cols-[1fr_74px] gap-2">
          <input v-model="form.host" type="text" class="source-input" :placeholder="t('settings.admin.navidromeHostPlaceholder')" />
          <input v-model="form.port" type="number" min="1" max="65535" class="source-input" :placeholder="t('settings.admin.navidromePortPlaceholder')" />
        </div>
        <select v-model="form.scheme" class="source-input">
          <option value="https">HTTPS</option>
          <option value="http">HTTP</option>
        </select>
        <div class="grid grid-cols-2 gap-2">
          <input v-model="form.username" type="text" class="source-input" :placeholder="t('settings.admin.navidromeUsernamePlaceholder')" />
          <input v-model="form.password" type="password" class="source-input" :placeholder="t('settings.admin.navidromePasswordPlaceholder')" />
        </div>
        <input v-model="form.allowedUsers" type="text" class="source-input" :placeholder="t('settings.admin.allowedUsersPlaceholder')" />
        <div class="grid grid-cols-2 gap-2">
          <button type="button" class="source-action primary" :disabled="busy" @click="saveSource">{{ t('settings.admin.saveSource') }}</button>
          <button type="button" class="source-action" :disabled="busy || !form.id.trim()" @click="testSource">{{ t('settings.admin.testSource') }}</button>
          <button type="button" class="source-action" :disabled="busy || !form.id.trim()" @click="removeSource">{{ t('settings.admin.removeSource') }}</button>
          <button type="button" class="source-action" :disabled="busy" @click="refreshEverything">{{ t('search.refreshSources') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '../api/auth';
import { useMusicStore } from '../stores/music';
import { useRoomStore } from '../stores/room';
import { useToast } from '../composables/useToast';
import { extractErrorMessage } from '../utils/errors';

const { t } = useI18n();
const { success, error } = useToast();
const roomStore = useRoomStore();
const musicStore = useMusicStore();

const open = ref(false);
const busy = ref(false);
const loadingSources = ref(false);
const adminUnlocked = ref(false);
const password = ref('');
const sources = ref([]);

const defaultForm = () => ({
  id: 'navidrome-custom',
  label: 'Navidrome',
  scheme: 'https',
  host: '',
  port: '443',
  username: '',
  password: '',
  allowedUsers: '*',
  enabled: true
});
const form = ref(defaultForm());

const requirePassword = () => {
  if (password.value) return true;
  error(t('settings.admin.passwordRequired'));
  return false;
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

const buildBaseUrl = () => {
  const host = form.value.host.trim();
  const port = String(form.value.port || '').trim();
  const scheme = form.value.scheme === 'http' ? 'http' : 'https';
  return host ? `${scheme}://${host}${port ? `:${port}` : ''}` : '';
};

const runAdmin = async (action, message) => {
  if (!requirePassword()) return;
  busy.value = true;
  try {
    await action();
    success(message);
    await refreshEverything();
  } catch (e) {
    error(extractErrorMessage(e, t('settings.admin.commandFailed')));
  } finally {
    busy.value = false;
  }
};

const runCleanupAction = async (action, message) => {
  if (!requirePassword()) return;
  busy.value = true;
  try {
    await action();
    success(message);
  } catch (e) {
    const messageText = extractErrorMessage(e, t('settings.admin.commandFailed'));
    if (!String(messageText).includes('Source not found')) {
      error(messageText);
    }
  } finally {
    resetForm();
    await refreshEverything();
    busy.value = false;
  }
};

const loadSources = async () => {
  if (!requirePassword()) return;
  loadingSources.value = true;
  try {
    const { data } = await authApi.listSubsonicSources(password.value, roomStore.currentRoomId);
    sources.value = Array.isArray(data) ? data : [];
    adminUnlocked.value = true;
  } catch (e) {
    adminUnlocked.value = false;
    error(extractErrorMessage(e, t('settings.admin.sourcesLoadFailed')));
  } finally {
    loadingSources.value = false;
  }
};

const unlockAdmin = async () => {
  await loadSources();
};

const refreshEverything = async () => {
  await loadSources();
  await musicStore.refreshPlatforms();
};

const editSource = (source) => {
  const parsed = parseSourceUrl(source.baseUrl || '');
  form.value = {
    id: source.id,
    label: source.label || source.id,
    scheme: parsed.scheme,
    host: parsed.host,
    port: parsed.port,
    username: source.username || '',
    password: '',
    allowedUsers: source.allowedUsers || '*',
    enabled: source.enabled !== false
  };
};

const resetForm = () => {
  form.value = defaultForm();
};

const saveSource = () => runAdmin(async () => {
  await authApi.saveSubsonicSource(password.value, roomStore.currentRoomId, {
    id: form.value.id.trim(),
    label: form.value.label.trim() || form.value.id.trim(),
    baseUrl: buildBaseUrl(),
    username: form.value.username.trim(),
    password: form.value.password,
    allowedUsers: form.value.allowedUsers.trim() || '*',
    enabled: form.value.enabled
  });
  form.value.password = '';
}, t('settings.admin.sourceSaved'));

const testSource = () => runAdmin(
  () => authApi.testSubsonicSource(password.value, roomStore.currentRoomId, form.value.id.trim()),
  t('settings.admin.sourceTested')
);

const removeSource = () => runCleanupAction(async () => {
  await authApi.removeSubsonicSource(password.value, roomStore.currentRoomId, form.value.id.trim());
}, t('settings.admin.sourceRemoved'));

const removeSourceById = (id) => runCleanupAction(async () => {
  await authApi.removeSubsonicSource(password.value, roomStore.currentRoomId, id);
  if (form.value.id === id) resetForm();
}, t('settings.admin.sourceRemoved'));

const moveSource = (index, direction) => runAdmin(async () => {
  const nextIndex = index + direction;
  const current = sources.value[index];
  const target = sources.value[nextIndex];
  if (!current || !target) return;
  await authApi.reorderSubsonicSource(password.value, roomStore.currentRoomId, current.id, target.sortOrder);
  await authApi.reorderSubsonicSource(password.value, roomStore.currentRoomId, target.id, current.sortOrder);
}, t('settings.admin.sourceOrderSaved'));
</script>

<style scoped>
.source-manager {
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

.source-manager-toggle {
  display: flex;
  min-height: 34px;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.source-manager-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 8px;
}

.source-input,
.source-action {
  min-height: 32px;
  border-radius: 6px;
  border: 1px solid var(--border-default);
  background: var(--surface-control);
  padding: 7px 9px;
  color: var(--text-primary);
  font-size: 12px;
  outline: none;
}

.source-input::placeholder {
  color: var(--text-tertiary);
}

.source-action {
  color: var(--text-secondary);
  font-weight: 700;
  transition: border-color 140ms ease, background 140ms ease, color 140ms ease;
}

.source-action.primary {
  border-color: var(--accent);
  color: var(--text-primary);
}

.source-action:hover:not(:disabled) {
  border-color: var(--accent);
  background: var(--surface-control-hover);
  color: var(--text-primary);
}

.source-action:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

.source-empty {
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  padding: 8px;
  color: var(--text-tertiary);
  font-size: 12px;
}

.source-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 32px;
  gap: 6px;
  align-items: stretch;
}

.source-main {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
  background: var(--surface-control);
  padding: 7px 8px;
  text-align: left;
}

.source-item.selected .source-main,
.source-main:hover {
  border-color: var(--border-default);
  background: var(--surface-control-hover);
}

.source-status {
  flex: 0 0 auto;
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
}

.source-status.off {
  color: var(--text-tertiary);
}

.source-row-actions {
  display: grid;
  grid-template-rows: 1fr 1fr 1fr;
  gap: 4px;
}

.source-row-actions button {
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
  color: var(--text-tertiary);
}

.source-row-actions button:hover:not(:disabled) {
  border-color: var(--border-default);
  color: var(--text-primary);
}

.source-row-actions button:disabled {
  opacity: 0.4;
}

.source-row-actions .material-symbols-outlined {
  font-size: 18px;
}

.source-remove:hover:not(:disabled) {
  color: var(--error, #ef4444);
}
</style>
