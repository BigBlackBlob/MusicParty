<template>
  <div v-if="isActive" class="fixed inset-0 z-[var(--z-tutorial)] pointer-events-auto">
    <div class="absolute inset-0 bg-black/55 transition-opacity duration-300"></div>

    <div
        v-if="targetRect"
        class="pointer-events-none absolute border border-[var(--accent)]/80 shadow-[0_0_0_1px_rgba(211,194,243,0.16),0_0_24px_rgba(211,194,243,0.18)] transition-all duration-300 ease-out"
        :style="{
          top: targetRect.top - 4 + 'px',
          left: targetRect.left - 4 + 'px',
          width: targetRect.width + 8 + 'px',
          height: targetRect.height + 8 + 'px',
          borderRadius: '14px'
        }"
    />

    <div
        v-if="currentStep"
        ref="tooltipRef"
        class="absolute flex flex-col gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-4 shadow-2xl"
        :style="tooltipStyle"
    >
      <div class="flex items-center justify-between border-b border-[var(--border-default)] pb-2">
        <span class="text-xs font-semibold tracking-[0.16em] text-[var(--text-tertiary)]">{{ t('tutorial.header', { current: currentStepIndex + 1, total: steps.length }) }}</span>
        <button @click="skipTutorial" class="text-xs font-semibold text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)]">{{ t('tutorial.skip') }}</button>
      </div>

      <div class="text-sm font-medium leading-relaxed text-[var(--text-primary)]">
        {{ currentDisplayContent }}
      </div>

      <div class="flex justify-end pt-1">
        <button
            @click="nextStep"
            class="rounded-full bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)]"
        >
          {{ currentStepIndex === steps.length - 1 ? t('tutorial.finish') : t('tutorial.next') }}
        </button>
      </div>

      <div
        class="absolute h-3 w-3 rotate-45 border border-[var(--border-default)] bg-[var(--surface-4)]"
        :class="arrowClass"
        :style="{ left: 'var(--arrow-left)' }"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, watch } from 'vue';
import { useWindowSize } from '@vueuse/core';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const isActive = ref(false);
const currentStepIndex = ref(0);
const targetRect = ref(null);
const isUsingMobileTarget = ref(false);

const { width, height } = useWindowSize();

const STORAGE_KEY = 'mp_tutorial_done_v1';

const steps = [
  {
    targetId: 'tutorial-rename',
    mobileTargetId: 'tutorial-rename-mobile',
    contentKey: 'tutorial.steps.rename.desktop',
    mobileContentKey: 'tutorial.steps.rename.mobile'
  },
  {
    targetId: 'tutorial-search',
    contentKey: 'tutorial.steps.search'
  },
  {
    targetId: 'tutorial-like',
    contentKey: 'tutorial.steps.like'
  },
  {
    targetId: 'tutorial-queue',
    mobileTargetId: 'tutorial-queue-mobile',
    contentKey: 'tutorial.steps.queue.desktop',
    mobileContentKey: 'tutorial.steps.queue.mobile'
  },
  {
    targetId: 'tutorial-pause',
    mobileTargetId: 'tutorial-pause-mobile',
    contentKey: 'tutorial.steps.pause'
  },
  {
    targetId: 'tutorial-download',
    mobileTargetId: 'tutorial-download-mobile',
    contentKey: 'tutorial.steps.download'
  },
  {
    targetId: 'tutorial-random',
    mobileTargetId: 'tutorial-random-mobile',
    contentKey: 'tutorial.steps.random'
  },
  {
    targetId: 'tutorial-chat',
    contentKey: 'tutorial.steps.chat'
  },
  {
    targetId: 'tutorial-source',
    contentKey: 'tutorial.steps.source'
  }
];

const currentStep = computed(() => steps[currentStepIndex.value]);
const currentDisplayContent = computed(() => {
  if (isUsingMobileTarget.value && currentStep.value.mobileContentKey) {
    return t(currentStep.value.mobileContentKey);
  }
  return t(currentStep.value.contentKey);
});

const tooltipRef = ref(null);
const tooltipStyle = ref({});
const arrowClass = ref('');

const isElementVisible = (el) => {
  if (!el) return false;
  const rect = el.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
};

const updatePosition = async () => {
  if (!isActive.value) return;
  await nextTick();

  const step = currentStep.value;
  let el = document.getElementById(step.targetId);
  isUsingMobileTarget.value = false;

  if (!isElementVisible(el) && step.mobileTargetId) {
    const mobileEl = document.getElementById(step.mobileTargetId);
    if (isElementVisible(mobileEl)) {
      el = mobileEl;
      isUsingMobileTarget.value = true;
    }
  }

  if (!isElementVisible(el)) {
    if (currentStepIndex.value < steps.length - 1) {
      currentStepIndex.value++;
      return;
    }
    finishTutorial();
    return;
  }

  const rect = el.getBoundingClientRect();
  targetRect.value = rect;

  const screenWidth = window.innerWidth;
  const screenHeight = window.innerHeight;
  const maxWidth = Math.min(320, screenWidth - 32);

  await nextTick();
  const actualHeight = tooltipRef.value ? tooltipRef.value.offsetHeight : 150;
  const margin = 12;

  let top;
  let left;
  let arrowPos = '';

  const spaceBelow = screenHeight - rect.bottom;
  const spaceAbove = rect.top;

  if (spaceBelow > actualHeight + margin + 20) {
    top = rect.bottom + margin;
    arrowPos = 'top';
  } else if (spaceAbove > actualHeight + margin + 20) {
    top = rect.top - actualHeight - margin;
    arrowPos = 'bottom';
  } else {
    top = spaceBelow > spaceAbove ? rect.bottom + margin : rect.top - actualHeight - margin;
    arrowPos = spaceBelow > spaceAbove ? 'top' : 'bottom';
  }

  if (top + actualHeight > screenHeight - 16) {
    top = screenHeight - actualHeight - 16;
  }
  if (top < 16) top = 16;

  left = rect.left + (rect.width / 2) - (maxWidth / 2);
  if (left < 16) left = 16;
  if (left + maxWidth > screenWidth - 16) left = screenWidth - maxWidth - 16;

  tooltipStyle.value = {
    top: `${top}px`,
    left: `${left}px`,
    width: `${maxWidth}px`
  };

  arrowClass.value = arrowPos === 'top' ? '-top-1.5' : '-bottom-1.5';

  const targetCenter = rect.left + (rect.width / 2);
  const arrowLeft = targetCenter - left;
  const clampedArrowLeft = Math.max(12, Math.min(maxWidth - 12, arrowLeft));
  tooltipStyle.value['--arrow-left'] = `${clampedArrowLeft}px`;
};

const nextStep = () => {
  if (currentStepIndex.value < steps.length - 1) {
    currentStepIndex.value++;
  } else {
    finishTutorial();
  }
};

const skipTutorial = () => {
  finishTutorial();
};

const finishTutorial = () => {
  isActive.value = false;
  localStorage.setItem(STORAGE_KEY, 'true');
};

const startTutorial = () => {
  if (localStorage.getItem(STORAGE_KEY)) return;
  setTimeout(() => {
    isActive.value = true;
    updatePosition();
  }, 1000);
};

watch([width, height, currentStepIndex], updatePosition);

const restart = () => {
  currentStepIndex.value = 0;
  isActive.value = true;
  updatePosition();
};

onMounted(() => {
  startTutorial();
});

defineExpose({ restart });
</script>
