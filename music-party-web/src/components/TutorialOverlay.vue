<template>
  <div v-if="isActive" class="fixed inset-0 z-[9999] pointer-events-auto">
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
        <span class="text-xs font-semibold tracking-[0.16em] text-[var(--text-tertiary)]">TUTORIAL SYSTEM // {{ currentStepIndex + 1 }}/{{ steps.length }}</span>
        <button @click="skipTutorial" class="text-xs font-semibold text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)]">SKIP</button>
      </div>

      <div class="text-sm font-medium leading-relaxed text-[var(--text-primary)]">
        {{ currentDisplayContent }}
      </div>

      <div class="flex justify-end pt-1">
        <button
            @click="nextStep"
            class="rounded-full bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)]"
        >
          {{ currentStepIndex === steps.length - 1 ? 'FINISH' : 'NEXT >' }}
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
    content: '点击这里可以修改你的昵称，输入后按回车确认。',
    mobileContent: '点击这里打开用户列表，可以修改你的昵称。'
  },
  {
    targetId: 'tutorial-search',
    content: '点击搜索按钮寻找歌曲。在此处也可以通过搜索用户名来查看平台账号歌单。'
  },
  {
    targetId: 'tutorial-like',
    content: '点击中间的封面可以为当前歌曲点赞。'
  },
  {
    targetId: 'tutorial-queue',
    mobileTargetId: 'tutorial-queue-mobile',
    content: '这里是播放队列。悬停在歌曲上可以进行置顶或删除操作。',
    mobileContent: '点击这里查看播放队列。'
  },
  {
    targetId: 'tutorial-pause',
    mobileTargetId: 'tutorial-pause-mobile',
    content: '注意：暂停/播放是全局生效的，会影响所有在线听众，请谨慎操作。'
  },
  {
    targetId: 'tutorial-download',
    mobileTargetId: 'tutorial-download-mobile',
    content: '听到喜欢的歌？点击这里可以直接下载当前播放的音频文件。'
  },
  {
    targetId: 'tutorial-random',
    mobileTargetId: 'tutorial-random-mobile',
    content: '随机播放模式采用“公平随机”算法，确保每个人点的歌都有均等的机会被播放。'
  },
  {
    targetId: 'tutorial-chat',
    content: '点击浮动按钮打开聊天窗口，可以和其他人聊天或查看记录。按钮可以拖动。'
  },
  {
    targetId: 'tutorial-source',
    content: '点击底部的小封面，可以跳转到歌曲的源网页。'
  }
];

const currentStep = computed(() => steps[currentStepIndex.value]);
const currentDisplayContent = computed(() => {
  if (isUsingMobileTarget.value && currentStep.value.mobileContent) {
    return currentStep.value.mobileContent;
  }
  return currentStep.value.content;
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
