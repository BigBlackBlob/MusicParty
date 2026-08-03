<template>
  <section class="invite-manager">
    <div class="invite-manager__header">
      <div>
        <h3>{{ t('settings.invites.title') }}</h3>
        <p>{{ t('settings.invites.description') }}</p>
      </div>
    </div>

    <div class="invite-manager__create">
      <label class="invite-manager__field">
        <span>{{ t('settings.invites.room') }}</span>
        <select v-model="selectedRoomId" :aria-label="t('settings.invites.room')" class="invite-manager__input">
          <option v-for="room in roomStore.rooms" :key="room.roomId" :value="room.roomId">{{ room.name }}</option>
        </select>
      </label>
      <label class="invite-manager__field">
        <span>{{ t('settings.invites.label') }}</span>
        <input v-model="label" maxlength="64" :placeholder="t('settings.invites.labelPlaceholder')" class="invite-manager__input" />
      </label>
      <button type="button" class="invite-manager__primary" :disabled="creating || !selectedRoomId" @click="createInvite">
        {{ creating ? t('settings.invites.creating') : t('settings.invites.create') }}
      </button>
    </div>

    <div v-if="createdUrl" class="invite-manager__created" aria-live="polite">
      <strong>{{ t('settings.invites.createdNotice') }}</strong>
      <div class="invite-manager__url-row">
        <input :value="createdUrl" readonly class="invite-manager__input invite-manager__url" :aria-label="t('settings.invites.link')" />
        <button type="button" class="invite-manager__action" :disabled="copying" @click="copyUrl">
          {{ copying ? t('settings.invites.copying') : t('settings.invites.copy') }}
        </button>
      </div>
    </div>

    <div class="invite-manager__history">
      <div class="invite-manager__history-header">
        <h4>{{ t('settings.invites.history') }}</h4>
        <button type="button" class="invite-manager__link" :disabled="loading" @click="loadInvites">
          {{ loading ? t('settings.invites.loading') : t('settings.invites.refresh') }}
        </button>
      </div>
      <p v-if="loading" class="invite-manager__muted">{{ t('settings.invites.loading') }}</p>
      <p v-else-if="loadError" class="invite-manager__error">{{ loadError }}</p>
      <p v-else-if="!invites.length" class="invite-manager__muted">{{ t('settings.invites.empty') }}</p>
      <div v-else class="invite-manager__list">
        <div v-for="invite in invites" :key="invite.id" class="invite-manager__row">
          <div class="invite-manager__details">
            <strong>{{ invite.label || t('settings.invites.noLabel') }}</strong>
            <span>{{ statusLabel(invite) }} · {{ expiryLabel(invite) }}</span>
          </div>
          <button v-if="getInviteStatus(invite) === 'available' || getInviteStatus(invite) === 'permanent'" type="button" class="invite-manager__link invite-manager__danger" :disabled="revokingId === invite.id" @click="revokeInvite(invite)">
            {{ revokingId === invite.id ? t('settings.invites.revoking') : t('settings.invites.revoke') }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '../api/auth';
import { useToast } from '../composables/useToast';
import { useRoomStore } from '../stores/room';
import { buildInviteUrl, getInviteStatus } from '../utils/invites';
import { extractErrorMessage } from '../utils/errors';

const { t } = useI18n();
const { error } = useToast();
const roomStore = useRoomStore();
const selectedRoomId = ref(roomStore.currentRoomId);
const label = ref('');
const invites = ref([]);
const createdUrl = ref('');
const creating = ref(false);
const copying = ref(false);
const loading = ref(false);
const loadError = ref('');
const revokingId = ref('');
const selectedRoom = computed(() => roomStore.rooms.find(room => room.roomId === selectedRoomId.value));

const statusLabel = invite => t(`settings.invites.status.${getInviteStatus(invite)}`);
const expiryLabel = invite => invite.permanent ? t('settings.invites.permanent') : new Date(invite.expiresAt).toLocaleString();

const loadInvites = async () => {
    if (!selectedRoomId.value) return;
    loading.value = true;
    loadError.value = '';
    try {
        const data = await authApi.listInvites(selectedRoomId.value);
        invites.value = Array.isArray(data) ? data : [];
    } catch (e) {
        loadError.value = extractErrorMessage(e, t('settings.invites.loadFailed'));
    } finally {
        loading.value = false;
    }
};

const createInvite = async () => {
    creating.value = true;
    createdUrl.value = '';
    try {
        const invite = await authApi.createInvite(selectedRoomId.value, label.value.trim());
        createdUrl.value = buildInviteUrl(invite.secret);
        label.value = '';
        await loadInvites();
    } catch (e) {
        error(extractErrorMessage(e, t('settings.invites.createFailed')));
    } finally {
        creating.value = false;
    }
};

const copyUrl = async () => {
    copying.value = true;
    try {
        await navigator.clipboard.writeText(createdUrl.value);
    } catch {
        error(t('settings.invites.copyFailed'));
    } finally {
        copying.value = false;
    }
};

const revokeInvite = async invite => {
    if (!window.confirm(t('settings.invites.confirmRevoke'))) return;
    revokingId.value = invite.id;
    try {
        await authApi.revokeInvite(selectedRoomId.value, invite.id);
        await loadInvites();
    } catch (e) {
        error(extractErrorMessage(e, t('settings.invites.revokeFailed')));
    } finally {
        revokingId.value = '';
    }
};

watch(selectedRoomId, () => {
    createdUrl.value = '';
    invites.value = [];
    void loadInvites();
});
watch(() => roomStore.currentRoomId, value => {
    if (!selectedRoom.value) selectedRoomId.value = value;
});
void loadInvites();
</script>

<style scoped>
.invite-manager { display: flex; min-width: 0; flex-direction: column; gap: 16px; }
.invite-manager__header h3, .invite-manager__history-header h4 { color: var(--text-primary); font-size: 18px; font-weight: 900; }
.invite-manager__header p, .invite-manager__muted { color: var(--text-tertiary); font-size: 12px; line-height: 1.5; }
.invite-manager__create { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; gap: 10px; align-items: end; }
.invite-manager__field { display: flex; min-width: 0; flex-direction: column; gap: 6px; color: var(--text-secondary); font-size: 12px; font-weight: 700; }
.invite-manager__input { width: 100%; min-height: 36px; border: 1px solid var(--border-default); border-radius: 6px; background: var(--surface-control); padding: 8px 10px; color: var(--text-primary); outline: none; }
.invite-manager__input:focus { border-color: var(--accent); }
.invite-manager__primary, .invite-manager__action { min-height: 36px; border-radius: 6px; padding: 8px 12px; background: var(--primary); color: var(--on-primary); font-size: 12px; font-weight: 800; }
.invite-manager__primary:disabled, .invite-manager__action:disabled { cursor: not-allowed; opacity: .55; }
.invite-manager__created { display: flex; flex-direction: column; gap: 8px; border: 1px solid var(--border-subtle); border-radius: 8px; background: var(--surface-control); padding: 12px; color: var(--text-primary); font-size: 12px; }
.invite-manager__url-row { display: flex; gap: 8px; min-width: 0; }
.invite-manager__url { min-width: 0; font-family: var(--font-mono, ui-monospace); }
.invite-manager__history { display: flex; flex-direction: column; gap: 10px; }
.invite-manager__history-header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.invite-manager__link { color: var(--text-tertiary); font-size: 11px; font-weight: 800; }
.invite-manager__link:hover:not(:disabled) { color: var(--text-primary); }
.invite-manager__danger { color: var(--danger, #ef4444); }
.invite-manager__list { display: flex; flex-direction: column; gap: 6px; }
.invite-manager__row { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--border-subtle); padding: 10px 0; }
.invite-manager__details { display: flex; min-width: 0; flex-direction: column; gap: 4px; }
.invite-manager__details strong { overflow: hidden; color: var(--text-primary); font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.invite-manager__details span { color: var(--text-tertiary); font-size: 11px; }
.invite-manager__error { color: var(--danger, #ef4444); font-size: 12px; }
@media (max-width: 720px) { .invite-manager__create { grid-template-columns: 1fr; } .invite-manager__url-row { flex-direction: column; } .invite-manager__row { align-items: flex-start; } }
</style>
