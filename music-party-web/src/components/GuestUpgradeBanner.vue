<template>
  <div v-if="user.accountType === 'guest'" class="guest-upgrade-banner">
    <div class="guest-upgrade-banner__content">
      <span class="material-symbols-outlined guest-upgrade-banner__icon">cloud_sync</span>
      <div class="guest-upgrade-banner__text">
        <strong>{{ t('auth.guestUpgradeBanner.title') }}</strong>
        <p>{{ t('auth.guestUpgradeBanner.description') }}</p>
      </div>
    </div>
    <button
      type="button"
      class="guest-upgrade-banner__button"
      @click="showUpgradeModal = true"
    >
      {{ t('auth.guestUpgradeBanner.action') }}
    </button>
  </div>

  <!-- 访客升级弹窗 -->
  <div v-if="showUpgradeModal" class="modal-overlay" @click.self="showUpgradeModal = false">
    <div class="modal-content">
      <div class="modal-header">
        <h3 class="modal-title">{{ t('auth.upgradeModal.title') }}</h3>
        <button type="button" class="modal-close" @click="showUpgradeModal = false">
          <span class="material-symbols-outlined">close</span>
        </button>
      </div>
      <div class="modal-body">
        <p class="modal-description">{{ t('auth.upgradeModal.description') }}</p>
        <form @submit.prevent="upgradeAccount">
          <label class="modal-label" for="upgrade-invite-code">{{ t('auth.upgradeModal.inviteCodeLabel') }}</label>
          <input
            id="upgrade-invite-code"
            v-model.trim="inviteCode"
            type="text"
            class="modal-input"
            :placeholder="t('auth.upgradeModal.inviteCodePlaceholder')"
            :disabled="upgrading"
          />
          <p v-if="upgradeError" class="modal-error">{{ upgradeError }}</p>
          <div class="modal-actions">
            <button type="button" class="modal-button modal-button--secondary" @click="showUpgradeModal = false" :disabled="upgrading">
              {{ t('common.cancel') }}
            </button>
            <button type="submit" class="modal-button modal-button--primary" :disabled="!inviteCode || upgrading">
              {{ upgrading ? t('auth.upgradeModal.upgrading') : t('auth.upgradeModal.upgrade') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '../api/auth';
import { useUserStore } from '../stores/user';

const { t } = useI18n();
const user = useUserStore();

const showUpgradeModal = ref(false);
const inviteCode = ref('');
const upgrading = ref(false);
const upgradeError = ref('');

const upgradeAccount = async () => {
  if (!inviteCode.value || upgrading.value) return;

  upgrading.value = true;
  upgradeError.value = '';

  try {
    const session = await authApi.upgradeGuestToUser(inviteCode.value);
    user.initAccount(session);
    showUpgradeModal.value = false;
    inviteCode.value = '';
    // Show success message
    console.log('Account upgraded successfully');
  } catch (error) {
    upgradeError.value = error.response?.data?.message || t('auth.upgradeModal.error');
  } finally {
    upgrading.value = false;
  }
};
</script>

<style scoped>
.guest-upgrade-banner {
  background: linear-gradient(135deg, var(--accent-muted) 0%, var(--accent) 100%);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.guest-upgrade-banner__content {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
}

.guest-upgrade-banner__icon {
  font-size: 32px;
  color: var(--text-inverse);
  flex-shrink: 0;
}

.guest-upgrade-banner__text {
  color: var(--text-inverse);
}

.guest-upgrade-banner__text strong {
  display: block;
  font-size: 16px;
  margin-bottom: 4px;
}

.guest-upgrade-banner__text p {
  font-size: 14px;
  opacity: 0.9;
  margin: 0;
}

.guest-upgrade-banner__button {
  background: rgba(255, 255, 255, 0.2);
  color: var(--text-inverse);
  border: 1px solid rgba(255, 255, 255, 0.3);
  padding: 8px 16px;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.guest-upgrade-banner__button:hover {
  background: rgba(255, 255, 255, 0.3);
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: var(--z-modal);
  padding: 16px;
}

.modal-content {
  background: var(--surface-4);
  border-radius: 12px;
  width: 100%;
  max-width: 480px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.modal-close {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  transition: background 0.2s;
}

.modal-close:hover {
  background: var(--surface-2);
}

.modal-body {
  padding: 24px;
}

.modal-description {
  color: var(--text-secondary);
  margin-bottom: 20px;
  line-height: 1.5;
}

.modal-label {
  display: block;
  color: var(--text-secondary);
  font-size: 14px;
  margin-bottom: 8px;
}

.modal-input {
  width: 100%;
  background: var(--surface-2);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 10px 12px;
  color: var(--text-primary);
  font-size: 14px;
  transition: border-color 0.2s;
}

.modal-input:focus {
  outline: none;
  border-color: var(--accent);
}

.modal-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.modal-error {
  color: var(--error-soft-text);
  font-size: 14px;
  margin-top: 8px;
}

.modal-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}

.modal-button {
  flex: 1;
  padding: 10px 16px;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

.modal-button--secondary {
  background: var(--surface-2);
  color: var(--text-primary);
}

.modal-button--secondary:hover:not(:disabled) {
  background: var(--surface-1);
}

.modal-button--primary {
  background: var(--accent);
  color: var(--text-inverse);
}

.modal-button--primary:hover:not(:disabled) {
  opacity: 0.9;
}

.modal-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
