<template>
  <div class="admin-settings-section">
    <div class="admin-auth-strip">
      <div class="admin-session-pill">{{ userStore.accountType }}</div>
      <input
        v-model="targetUser"
        type="text"
        class="admin-input"
        :placeholder="t('settings.admin.userPlaceholder')"
      />
    </div>

    <section v-if="section === 'library'" class="admin-page">
      <div class="admin-page-header">
        <div>
          <h3>{{ t('settings.admin.localLibrary') }}</h3>
          <p>{{ t('settings.admin.localLibraryDesc') }}</p>
        </div>
        <button class="admin-action" type="button" :disabled="busy || loadingLocalTracks" @click="loadLocalTracks">
          {{ loadingLocalTracks ? t('settings.admin.loadingSources') : t('common.refresh') }}
        </button>
      </div>

      <div class="admin-two-column">
        <div class="admin-panel">
          <h4>{{ t('settings.admin.uploadLocalTrack') }}</h4>
          <div class="grid grid-cols-2 gap-2">
            <input v-model="localForm.title" type="text" class="admin-input" :placeholder="t('settings.admin.localTitlePlaceholder')" />
            <input v-model="localForm.artists" type="text" class="admin-input" :placeholder="t('settings.admin.localArtistsPlaceholder')" />
          </div>
          <input v-model="localForm.album" type="text" class="admin-input" :placeholder="t('settings.admin.localAlbumPlaceholder')" />
          <input ref="localFileInput" type="file" accept="audio/*" multiple class="admin-input" @change="onLocalFileChange" />
          <div class="grid grid-cols-2 gap-2">
            <button class="admin-action" type="button" :disabled="busy || readingLocalTags || localFiles.length === 0" @click="readSelectedLocalTags">
              {{ readingLocalTags ? t('settings.admin.readingLocalTags') : t('settings.admin.readLocalTags') }}
            </button>
            <button class="admin-action" type="button" :disabled="busy || localFiles.length === 0" @click="runUploadLocalTrack">
              {{ t('settings.admin.uploadLocalTrack') }}
            </button>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <button class="admin-action" type="button" :disabled="busy" @click="loadLocalUploadAccess">
              {{ t('settings.admin.localUploadAccess') }}
            </button>
            <button class="admin-action" type="button" :disabled="busy || loadingLocalTracks" @click="loadLocalTracks">
              {{ loadingLocalTracks ? t('settings.admin.loadingSources') : t('common.refresh') }}
            </button>
          </div>
          <p v-if="localTagHint" class="admin-hint">{{ localTagHint }}</p>
        </div>

        <div class="admin-panel">
          <div class="admin-subheader">
            <h4>{{ t('settings.admin.localUploadAccess') }}</h4>
            <button class="admin-link-action" type="button" :disabled="busy" @click="loadLocalUploadAccess">
              {{ t('common.refresh') }}
            </button>
          </div>
          <div class="grid grid-cols-[1fr_110px] gap-2">
            <input v-model="localAccessUser" type="text" class="admin-input" :placeholder="t('settings.admin.userPlaceholder')" />
            <button class="admin-action" type="button" :disabled="busy || !localAccessUser.trim()" @click="runGrantLocalUpload">
              {{ t('settings.admin.grantLocalUpload') }}
            </button>
          </div>
          <div v-if="localUploadUsers.length" class="flex flex-wrap gap-1">
            <button
              v-for="name in localUploadUsers"
              :key="name"
              type="button"
              class="local-user-pill"
              :disabled="busy || name === '*'"
              @click="runRevokeLocalUpload(name)"
            >
              {{ name }}
            </button>
          </div>
        </div>
      </div>

      <div class="admin-panel">
        <div class="admin-subheader">
          <h4>{{ t('settings.admin.localTracks') }}</h4>
          <span class="admin-count">{{ localTracks.length }}</span>
        </div>
          <div class="space-y-1">
            <div v-if="localTracks.length" class="space-y-1">
              <div v-for="track in localTracks" :key="track.id" class="source-row">
                <span class="min-w-0">
                  <input
                    v-model="track.title"
                    type="text"
                    class="local-inline-input font-bold"
                    :placeholder="t('settings.admin.localTitlePlaceholder')"
                  />
                  <input
                    v-model="track.artistsText"
                    type="text"
                    class="local-inline-input text-[11px]"
                    :placeholder="t('settings.admin.localArtistsPlaceholder')"
                  />
                  <span class="block truncate text-[11px] text-text-tertiary">{{ track.status }}{{ track.errorMessage ? ` · ${track.errorMessage}` : '' }}</span>
                </span>
                <span class="flex flex-col gap-1">
                  <button class="admin-link-action" type="button" :disabled="busy" @click="runUpdateLocalTrack(track)">
                    {{ t('common.done') }}
                  </button>
                  <button class="admin-link-action danger-link" type="button" :disabled="busy" @click="runDeleteLocalTrack(track)">
                    {{ t('settings.admin.deleteLocalTrack') }}
                  </button>
                </span>
              </div>
            </div>
            <div v-else class="rounded-md border border-border-subtle px-3 py-2 text-xs text-text-tertiary">
              {{ t('settings.admin.noLocalTracks') }}
            </div>
          </div>
      </div>
    </section>

    <section v-else-if="section === 'sources'" class="admin-page">
      <div class="admin-page-header">
        <div>
          <h3>{{ t('settings.admin.sourceManager') }}</h3>
          <p>{{ t('settings.admin.sourceManagerDesc') }}</p>
        </div>
        <button class="admin-action" type="button" :disabled="busy || loadingSources" @click="loadSubsonicSources">
          {{ loadingSources ? t('settings.admin.loadingSources') : t('common.refresh') }}
        </button>
      </div>

      <div class="admin-two-column admin-two-column--sources">
        <div class="admin-panel">
          <div class="space-y-1">
            <h4>{{ t('settings.admin.sourceList') }}</h4>
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
        </div>

        <div class="admin-panel">
          <h4>{{ t('settings.admin.customNavidrome') }}</h4>
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
    </section>

    <section v-else class="admin-page">
      <div class="admin-page-header">
        <div>
          <h3>{{ t('settings.admin.title') }}</h3>
          <p>{{ t('settings.admin.adminToolsDesc') }}</p>
        </div>
      </div>

      <div class="admin-panel">
        <h4>{{ t('settings.admin.userAccess') }}</h4>
        <div class="grid grid-cols-2 gap-2">
          <button class="admin-action" type="button" :disabled="busy" @click="runGrantNavidrome">
            {{ t('settings.admin.grantNavidrome') }}
          </button>
          <button class="admin-action" type="button" :disabled="busy" @click="runRevokeNavidrome">
            {{ t('settings.admin.revokeNavidrome') }}
          </button>
        </div>
      </div>

      <div class="admin-panel">
        <h4>{{ t('settings.admin.platformCredentials') }}</h4>
        <p class="admin-hint">{{ t('settings.admin.platformCredentialsHint') }}</p>
        <div class="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
          <input v-model="neteaseCredential" type="password" autocomplete="off" class="admin-input" :placeholder="t('settings.admin.neteaseCookiePlaceholder')" :aria-label="t('settings.admin.neteaseCookie')" />
          <button class="admin-action" type="button" :disabled="busy || !neteaseCredential.trim()" @click="updatePlatformCredential('netease')">
            {{ t('settings.admin.updateCredential') }}
          </button>
        </div>
        <div class="grid grid-cols-[minmax(0,1fr)_auto] gap-2">
          <input v-model="bilibiliCredential" type="password" autocomplete="off" class="admin-input" :placeholder="t('settings.admin.bilibiliSessdataPlaceholder')" :aria-label="t('settings.admin.bilibiliSessdata')" />
          <button class="admin-action" type="button" :disabled="busy || !bilibiliCredential.trim()" @click="updatePlatformCredential('bilibili')">
            {{ t('settings.admin.updateCredential') }}
          </button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '../api/auth';
import { useToast } from '../composables/useToast';
import { useQuery, useQueryClient } from '@tanstack/vue-query';
import { queryKeys } from '../domains/queryKeys';
import { useRoomStore } from '../stores/room';
import { useUserStore } from '../stores/user';
import { readAudioTagsFromFile } from '../utils/id3Tags';
import { extractErrorMessage } from '../utils/errors';

const { t } = useI18n();
const { success, error } = useToast();
const queryClient = useQueryClient();
const roomStore = useRoomStore();
const userStore = useUserStore();
const props = defineProps({
  section: {
    type: String,
    default: 'admin'
  }
});

const targetUser = ref(userStore.currentUser?.name || '');
const neteaseCredential = ref('');
const bilibiliCredential = ref('');
const localFiles = ref([]);
const localFileInput = ref(null);
const readingLocalTags = ref(false);
const localTagHint = ref('');
const localAccessUser = ref('');
const localForm = ref({
  title: '',
  artists: '',
  album: ''
});
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
const busy = ref(false);
const canLoadAdminResources = computed(() => userStore.capabilities.canManageSite);
const localTracksQuery = useQuery({
  queryKey: queryKeys.admin.localTracks(),
  queryFn: async () => (await authApi.listLocalTracks()).map(track => ({
    ...track,
    artistsText: Array.isArray(track.artists) ? track.artists.join(', ') : ''
  })),
  enabled: computed(() => canLoadAdminResources.value && props.section === 'library')
});
const localUploadAccessQuery = useQuery({
  queryKey: queryKeys.admin.localUploadAccess(),
  queryFn: () => authApi.listLocalUploadAccess(),
  enabled: computed(() => canLoadAdminResources.value && props.section === 'library')
});
const subsonicSourcesQuery = useQuery({
  queryKey: computed(() => queryKeys.admin.subsonic(roomStore.currentRoomId)),
  queryFn: () => authApi.listSubsonicSources(roomStore.currentRoomId),
  enabled: computed(() => canLoadAdminResources.value && props.section === 'sources')
});
const localTracks = computed(() => localTracksQuery.data.value ?? []);
const loadingLocalTracks = computed(() => localTracksQuery.isFetching.value);
const localUploadUsers = computed(() => localUploadAccessQuery.data.value ?? []);
const subsonicSources = computed(() => subsonicSourcesQuery.data.value ?? []);
const loadingSources = computed(() => subsonicSourcesQuery.isFetching.value);

const currentTargetUser = () => (targetUser.value || userStore.currentUser?.name || '').trim();
const runAdminAction = async (action, successMessage = t('settings.admin.commandExecuted')) => {
  if (!userStore.capabilities.canManageSite) {
    error(t('settings.admin.passwordRequired'));
    return;
  }
  busy.value = true;
  try {
    await action();
    success(successMessage);
    await queryClient.invalidateQueries({ queryKey: ['platforms'] });
  } catch (e) {
    error(extractErrorMessage(e, t('settings.admin.commandFailed')));
  } finally {
    busy.value = false;
  }
};

const loadSubsonicSources = async () => {
  if (!userStore.capabilities.canManageSite) {
    error(t('settings.admin.passwordRequired'));
    return;
  }
  try {
    const result = await subsonicSourcesQuery.refetch();
    if (result.error) throw result.error;
  } catch (e) {
    error(extractErrorMessage(e, t('settings.admin.sourcesLoadFailed')));
  }
};

const loadLocalTracks = async () => {
  if (!userStore.capabilities.canManageSite) {
    error(t('settings.admin.passwordRequired'));
    return;
  }
  try {
    const result = await localTracksQuery.refetch();
    if (result.error) throw result.error;
  } catch (e) {
    error(extractErrorMessage(e, t('settings.admin.localTracksLoadFailed')));
  }
};

const loadLocalUploadAccess = async () => {
  if (!userStore.capabilities.canManageSite) return;
  try {
    const result = await localUploadAccessQuery.refetch();
    if (result.error) throw result.error;
  } catch (e) {
    error(extractErrorMessage(e, t('settings.admin.localAccessLoadFailed')));
  }
};

const onLocalFileChange = (event) => {
  localFiles.value = Array.from(event.target.files || []);
  localTagHint.value = localFiles.value.length > 1
    ? t('settings.admin.localTagReadFirstFile')
    : '';
};

const readSelectedLocalTags = async () => {
  const file = localFiles.value[0];
  if (!file) return;
  readingLocalTags.value = true;
  localTagHint.value = '';
  try {
    const tags = await readAudioTagsFromFile(file);
    if (!tags.title && !tags.artists && !tags.album) {
      localTagHint.value = t('settings.admin.localTagsNotFound');
      return;
    }
    localForm.value = {
      title: tags.title || localForm.value.title,
      artists: tags.artists || localForm.value.artists,
      album: tags.album || localForm.value.album
    };
    localTagHint.value = t('settings.admin.localTagsRead');
  } catch (e) {
    localTagHint.value = extractErrorMessage(e, t('settings.admin.localTagsReadFailed'));
  } finally {
    readingLocalTags.value = false;
  }
};

const runUploadLocalTrack = () => runAdminAction(
  async () => {
    if (localFiles.value.length === 0) return;
    for (const file of localFiles.value) {
      await authApi.uploadLocalTrack(file, localForm.value);
    }
    localFiles.value = [];
    localForm.value = { title: '', artists: '', album: '' };
    if (localFileInput.value) localFileInput.value.value = '';
    await loadLocalTracks();
  },
  t('settings.admin.localUploadQueued')
);

const runUpdateLocalTrack = (track) => runAdminAction(
  async () => {
    await authApi.updateLocalTrack(track.id, {
      title: track.title,
      artists: String(track.artistsText || '').split(',').map(item => item.trim()).filter(Boolean),
      album: track.album || ''
    });
    await loadLocalTracks();
  },
  t('settings.admin.localTrackSaved')
);

const runDeleteLocalTrack = (track) => runAdminAction(
  async () => {
    await authApi.deleteLocalTrack(track.id);
    await loadLocalTracks();
  },
  t('settings.admin.localTrackDeleted')
);

const runGrantLocalUpload = () => runAdminAction(
  async () => {
    await authApi.grantLocalUploadAccess(localAccessUser.value.trim());
    localAccessUser.value = '';
    await loadLocalUploadAccess();
  },
  t('settings.admin.localUploadGranted')
);

const runRevokeLocalUpload = (name) => runAdminAction(
  async () => {
    await authApi.revokeLocalUploadAccess(name);
    await loadLocalUploadAccess();
  },
  t('settings.admin.localUploadRevoked')
);

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
  () => authApi.grantNavidrome(currentTargetUser(), roomStore.currentRoomId),
  t('settings.admin.navidromeGranted')
);

const runRevokeNavidrome = () => runAdminAction(
  () => authApi.revokeNavidrome(currentTargetUser(), roomStore.currentRoomId),
  t('settings.admin.navidromeRevoked')
);

const updatePlatformCredential = (platform) => runAdminAction(
  async () => {
    const credential = platform === 'netease' ? neteaseCredential.value.trim() : bilibiliCredential.value.trim();
    await authApi.updatePlatformCredential(platform, credential);
    if (platform === 'netease') neteaseCredential.value = '';
    else bilibiliCredential.value = '';
  },
  t('settings.admin.credentialUpdated')
);

const runSaveCustomNavidrome = () => runAdminAction(
  async () => {
    await authApi.saveSubsonicSource(roomStore.currentRoomId, {
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
    await authApi.removeSubsonicSource(roomStore.currentRoomId, navidromeForm.value.id.trim());
    resetNavidromeForm();
    await loadSubsonicSources();
  },
  t('settings.admin.sourceRemoved')
);

const runTestCustomNavidrome = () => runAdminAction(
  () => authApi.testSubsonicSource(roomStore.currentRoomId, navidromeForm.value.id.trim()),
  t('settings.admin.sourceTested')
);

watch(
  () => [props.section, userStore.capabilities.canManageSite],
  async ([section]) => {
    if (!userStore.capabilities.canManageSite) return;
    if (section === 'library') {
      await Promise.all([loadLocalTracks(), loadLocalUploadAccess()]);
    }
    if (section === 'sources') {
      await loadSubsonicSources();
    }
  },
  { immediate: true }
);
</script>

<style scoped>
.admin-settings-section,
.admin-page {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 14px;
}

.admin-auth-strip {
  display: grid;
  grid-template-columns: minmax(180px, 260px) minmax(180px, 1fr);
  gap: 10px;
}

.admin-session-pill {
  min-height: 34px;
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  background: var(--surface-2);
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  padding: 0 12px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0;
}

.admin-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.admin-page-header h3 {
  color: var(--text-primary);
  font-size: 18px;
  font-weight: 900;
}

.admin-page-header p {
  margin-top: 3px;
  max-width: 58ch;
  color: var(--text-tertiary);
  font-size: 12px;
  line-height: 1.5;
}

.admin-two-column {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(260px, 0.8fr);
  gap: 12px;
}

.admin-two-column--sources {
  grid-template-columns: minmax(260px, 0.8fr) minmax(0, 1.2fr);
}

.admin-panel {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 10px;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: color-mix(in srgb, var(--surface-control) 78%, transparent);
  padding: 12px;
}

.admin-panel h4 {
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 900;
}

.admin-subheader {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.admin-count {
  border-radius: 999px;
  background: var(--surface-raised);
  padding: 2px 8px;
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 900;
}

.admin-hint {
  color: var(--text-tertiary);
  font-size: 11px;
  line-height: 1.45;
}

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

.local-inline-input {
  display: block;
  width: 100%;
  min-width: 0;
  border: 0;
  background: transparent;
  color: var(--text-secondary);
  outline: none;
}

.danger-link {
  color: var(--danger, #ef4444);
}

.local-user-pill {
  border-radius: 999px;
  border: 1px solid var(--border-subtle);
  background: var(--surface-control);
  padding: 3px 8px;
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 700;
}

.local-user-pill:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--text-primary);
}

@media (max-width: 820px) {
  .admin-auth-strip,
  .admin-two-column,
  .admin-two-column--sources {
    grid-template-columns: 1fr;
  }

  .admin-page-header {
    flex-direction: column;
  }
}
</style>
