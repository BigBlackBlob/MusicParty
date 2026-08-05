<template>
  <div
    v-if="open"
    role="dialog"
    aria-modal="true"
    :aria-labelledby="titleId"
    class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/70 p-4 backdrop-blur-xl"
    @click.self="emitCancel"
  >
    <form
      class="w-full max-w-sm rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-6 shadow-2xl"
      @submit.prevent="submit"
    >
      <div class="mb-5 flex items-center justify-between gap-3">
        <div class="min-w-0">
          <h2 :id="titleId" class="truncate text-lg font-bold text-[var(--text-primary)]">{{ t('rooms.privateAccessTitle') }}</h2>
          <p class="mt-1 truncate text-sm text-[var(--text-secondary)]">{{ room?.name || 'Lounge' }}</p>
        </div>
        <button
          type="button"
          class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-[var(--text-tertiary)] hover:bg-[var(--surface-3)] hover:text-[var(--text-primary)]"
          :disabled="loading"
          :aria-label="t('common.close')"
          :title="t('common.close')"
          @click="emitCancel"
        >
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <label class="block">
        <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-[var(--text-tertiary)]">{{ t('rooms.password') }}</span>
        <input
          ref="passwordInput"
          v-model="password"
          type="password"
          class="w-full rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-[var(--accent)]"
          :placeholder="t('rooms.passwordPlaceholder')"
          autocomplete="current-password"
          :disabled="loading"
        />
      </label>

      <p class="mt-3 text-xs leading-5 text-[var(--text-tertiary)]">{{ t('rooms.privateAccessDesc') }}</p>

      <div v-if="error" class="mt-3 text-sm text-[var(--error-soft-text)]" role="alert">{{ error }}</div>

      <div class="mt-6 flex justify-end gap-2">
        <button
          type="button"
          class="rounded-xl border border-[var(--border-default)] px-4 py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--surface-3)] disabled:opacity-60"
          :disabled="loading"
          @click="emitCancel"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--text-inverse)] hover:bg-[var(--accent-hover)] disabled:opacity-60"
          :disabled="loading"
        >
          {{ loading ? t('rooms.verifying') : t('rooms.verifyAndEnter') }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import type { RoomSummary } from '../contracts/generated/models';

const props = withDefaults(defineProps<{
  open?: boolean;
  room?: RoomSummary | null;
  loading?: boolean;
  error?: string;
}>(), {
  open: false,
  room: null,
  loading: false,
  error: ''
});

const emit = defineEmits<{
  submit: [password: string];
  cancel: [];
}>();
const { t } = useI18n();
const titleId = 'private-room-access-title';
const password = ref('');
const passwordInput = ref<HTMLInputElement | null>(null);

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      password.value = '';
      return;
    }
    await nextTick();
    passwordInput.value?.focus?.();
  }
);

const submit = () => {
  emit('submit', password.value);
};

const emitCancel = () => {
  if (props.loading) return;
  emit('cancel');
};
</script>
