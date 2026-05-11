<template>
  <TransitionGroup
      tag="div"
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="translate-y-2 opacity-0 scale-95"
      enter-to-class="translate-y-0 opacity-100 scale-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="translate-y-0 opacity-100 scale-100"
      leave-to-class="translate-y-2 opacity-0 scale-95"
      class="fixed top-10 left-1/2 -translate-x-1/2 z-[100] flex flex-col items-center gap-2 pointer-events-none"
  >
    <div
        v-for="toast in toastStore.toasts"
        :key="toast.id"
        class="flex min-w-[320px] max-w-[min(92vw,720px)] cursor-pointer items-start gap-3 rounded-2xl border px-5 py-3 text-[var(--text-primary)] shadow-[var(--toast-shadow)] backdrop-blur-xl pointer-events-auto"
        :class="getTypeClass(toast.type)"
        @click="toastStore.remove(toast.id)"
    >
      <component :is="getIcon(toast.type)" class="mt-0.5 h-5 w-5 flex-shrink-0" :class="getIconClass(toast.type)" />
      <div class="flex-1 min-w-0">
        <div class="font-semibold text-sm font-sans">{{ toast.title }}</div>
        <div v-if="toast.message" class="mt-1 whitespace-pre-wrap break-all text-xs leading-relaxed text-[var(--toast-message)]">{{ toast.message }}</div>
      </div>
    </div>
  </TransitionGroup>
</template>

<script setup>
import { useToastStore } from '../stores/toast';
import { CheckCircle, AlertCircle, Info, AlertTriangle } from 'lucide-vue-next';

const toastStore = useToastStore();

const getTypeClass = (type) => {
  switch (type) {
    case 'success': return 'bg-[var(--toast-success-bg)] border-[var(--toast-success-border)]';
    case 'error': return 'bg-[var(--toast-error-bg)] border-[var(--toast-error-border)]';
    case 'warning': return 'bg-[var(--toast-warning-bg)] border-[var(--toast-warning-border)]';
    default: return 'bg-[var(--toast-info-bg)] border-[var(--toast-info-border)]';
  }
};

const getIcon = (type) => {
  switch (type) {
    case 'success': return CheckCircle;
    case 'error': return AlertCircle;
    case 'warning': return AlertTriangle;
    default: return Info;
  }
};

const getIconClass = (type) => {
  switch (type) {
    case 'success': return 'text-[var(--toast-success-icon)]';
    case 'error': return 'text-[var(--toast-error-icon)]';
    case 'warning': return 'text-[var(--toast-warning-icon)]';
    default: return 'text-[var(--toast-info-icon)]';
  }
};
</script>
