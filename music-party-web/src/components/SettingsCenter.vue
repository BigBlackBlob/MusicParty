<template>
  <div class="settings-center">
    <div class="settings-center__shell" :class="{ 'settings-center__shell--mobile': mobile }">
      <header class="settings-center__header">
        <div>
          <p class="settings-center__eyebrow">{{ t('settings.workbench') }}</p>
          <h2 class="settings-center__title">{{ t('settings.title') }}</h2>
        </div>
        <button type="button" class="settings-center__close" :aria-label="t('common.close')" @click="$emit('close')">
          <span class="material-symbols-outlined">close</span>
        </button>
      </header>

      <div class="settings-center__body">
        <nav class="settings-center__nav" aria-label="Settings sections">
          <button
            v-for="section in sections"
            :key="section.id"
            type="button"
            class="settings-center__nav-item"
            :class="{ 'settings-center__nav-item--active': activeSection === section.id }"
            @click="activeSection = section.id"
          >
            <span class="material-symbols-outlined text-[18px]">{{ section.icon }}</span>
            <span>{{ section.label }}</span>
          </button>
        </nav>

        <main class="settings-center__content">
          <section v-if="activeSection === 'general'" class="settings-section">
            <h3 class="settings-section__title">{{ t('settings.general') }}</h3>
            <div class="settings-grid">
              <button class="settings-row" type="button" @click="ui.toggleDarkMode">
                <span>{{ t('settings.theme') }}</span>
                <strong>{{ ui.isDarkMode ? t('settings.dark') : t('settings.light') }}</strong>
              </button>
              <button class="settings-row" type="button" @click="ui.setLocale(ui.locale === 'en' ? 'zh' : 'en')">
                <span>{{ t('settings.language') }}</span>
                <strong>{{ ui.locale === 'en' ? t('settings.english') : t('settings.chinese') }}</strong>
              </button>
              <button class="settings-row" type="button" @click="ui.toggleLiteMode">
                <span>{{ t('settings.liteMode') }}</span>
                <strong>{{ ui.isLiteMode ? t('settings.on') : t('settings.off') }}</strong>
              </button>
              <label class="settings-row">
                <span>{{ t('settings.autoLiteMode') }}</span>
                <input v-model="ui.autoLiteMode" type="checkbox" class="settings-checkbox" />
              </label>
            </div>

            <div class="settings-control-block">
              <div class="settings-control-block__header">
                <span>{{ t('settings.stageDensity') }}</span>
                <strong>{{ Math.round(ui.mainStageScale * 100) }}%</strong>
              </div>
              <input :value="ui.mainStageScale" type="range" min="0.90" max="1.20" step="0.02" class="settings-range" @input="e => ui.setMainStageScale(e.target.value)" />
            </div>
            <div class="settings-control-block">
              <div class="settings-control-block__header">
                <span>{{ t('settings.globalZoom') }}</span>
                <strong>{{ Math.round(ui.globalZoomLevel * 100) }}%</strong>
              </div>
              <input :value="ui.globalZoomLevel" type="range" min="1.0" max="1.5" step="0.05" class="settings-range" @input="e => ui.setGlobalZoomLevel(e.target.value)" />
            </div>
            <div class="settings-control-block">
              <div class="settings-control-block__header">
                <span>{{ t('settings.mobileDensity') }}</span>
                <strong>{{ ui.mobileNowDensity }}</strong>
              </div>
              <div class="settings-segment">
                <button v-for="density in ['compact', 'standard', 'relaxed']" :key="density" type="button" :class="{ active: ui.mobileNowDensity === density }" @click="ui.setMobileNowDensity(density)">
                  {{ t(`settings.${density}`) }}
                </button>
              </div>
            </div>
          </section>

          <section v-else-if="activeSection === 'members'" class="settings-section">
            <h3 class="settings-section__title">{{ t('settings.onlineMembers') }}</h3>
            <div class="settings-members">
              <div v-for="member in displayMembers" :key="member.publicId || member.name" class="settings-member">
                <span class="settings-member__avatar">{{ getInitials(member.name) }}</span>
                <span class="min-w-0">
                  <strong class="block truncate">{{ member.name }}</strong>
                  <small>{{ member.isGuest ? t('settings.guest') : t('settings.member') }}</small>
                </span>
              </div>
            </div>
          </section>

          <section v-else class="settings-section">
            <AdminSettingsPanel :section="activeSection" />
          </section>
        </main>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import AdminSettingsPanel from './AdminSettingsPanel.vue';
import { useUiStore } from '../stores/ui';
import { useUserStore } from '../stores/user';

defineProps({
  mobile: {
    type: Boolean,
    default: false
  }
});

defineEmits(['close']);

const { t } = useI18n();
const ui = useUiStore();
const user = useUserStore();
const activeSection = ref('general');

const sections = computed(() => [
  { id: 'general', icon: 'tune', label: t('settings.general') },
  { id: 'library', icon: 'library_music', label: t('settings.admin.localLibrary') },
  { id: 'sources', icon: 'dns', label: t('settings.admin.sourceManager') },
  { id: 'admin', icon: 'admin_panel_settings', label: t('settings.admin.title') },
  { id: 'members', icon: 'group', label: t('settings.onlineMembers') }
]);

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
</script>

<style scoped>
.settings-center {
  position: fixed;
  inset: 0;
  z-index: var(--z-modal);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
  background: var(--overlay-backdrop);
  backdrop-filter: blur(14px);
}

.settings-center__shell {
  display: flex;
  width: min(1040px, 100%);
  height: min(760px, calc(var(--app-height) - 64px));
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--border-default);
  border-radius: 12px;
  background: var(--surface-panel);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.42);
}

.settings-center__shell--mobile {
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: 0;
}

.settings-center__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border-default);
}

.settings-center__eyebrow {
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--text-tertiary);
}

.settings-center__title {
  font-size: 22px;
  font-weight: 900;
  color: var(--text-primary);
}

.settings-center__close {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 8px;
  color: var(--text-secondary);
}

.settings-center__close:hover {
  background: var(--surface-control-hover);
  color: var(--text-primary);
}

.settings-center__body {
  display: grid;
  min-height: 0;
  flex: 1;
  grid-template-columns: 220px 1fr;
}

.settings-center__nav {
  min-height: 0;
  overflow-y: auto;
  border-right: 1px solid var(--border-default);
  padding: 14px;
}

.settings-center__nav-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 10px;
  border-radius: 8px;
  padding: 10px 12px;
  text-align: left;
  font-size: 13px;
  font-weight: 800;
  color: var(--text-secondary);
}

.settings-center__nav-item:hover,
.settings-center__nav-item--active {
  background: var(--surface-control-hover);
  color: var(--primary);
}

.settings-center__content {
  min-height: 0;
  overflow-y: auto;
  padding: 22px;
}

.settings-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.settings-section__title {
  font-size: 16px;
  font-weight: 900;
  color: var(--text-primary);
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.settings-row,
.settings-control-block,
.settings-member {
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--surface-control);
}

.settings-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  color: var(--text-secondary);
}

.settings-row strong,
.settings-control-block strong {
  color: var(--text-primary);
}

.settings-checkbox,
.settings-range {
  accent-color: var(--accent);
}

.settings-control-block {
  padding: 14px;
}

.settings-control-block__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  color: var(--text-secondary);
}

.settings-range {
  width: 100%;
}

.settings-segment {
  display: flex;
  gap: 6px;
}

.settings-segment button {
  flex: 1;
  border-radius: 6px;
  padding: 8px;
  font-size: 12px;
  font-weight: 800;
  color: var(--text-secondary);
  background: var(--surface-raised);
}

.settings-segment button.active {
  background: var(--primary);
  color: var(--on-primary);
}

.settings-members {
  display: grid;
  gap: 10px;
}

.settings-member {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
}

.settings-member__avatar {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 8px;
  background: var(--surface-raised);
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 900;
}

.settings-member small {
  color: var(--text-tertiary);
}

@media (max-width: 720px) {
  .settings-center {
    padding: 0;
  }

  .settings-center__body {
    grid-template-columns: 1fr;
  }

  .settings-center__nav {
    display: flex;
    gap: 8px;
    overflow-x: auto;
    border-right: 0;
    border-bottom: 1px solid var(--border-default);
  }

  .settings-center__nav-item {
    width: auto;
    white-space: nowrap;
  }

  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>
