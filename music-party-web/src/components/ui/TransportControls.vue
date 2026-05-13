<template>
  <div class="flex items-center gap-1 sm:gap-3">
    <IconButton
      size="md"
      :variant="shuffle ? 'primary' : 'ghost'"
      :disabled="shuffleLocked"
      @click="$emit('toggle-shuffle')"
      title="随机播放"
    >
      <Shuffle class="w-4 h-4" />
    </IconButton>

    <IconButton
      size="md"
      variant="ghost"
      :disabled="skipLocked"
      @click="$emit('prev')"
      title="上一首"
    >
      <SkipBack class="w-5 h-5 fill-current" />
    </IconButton>

    <div class="relative">
      <IconButton
        size="lg"
        variant="primary"
        :disabled="pauseLocked && !isPaused"
        @click="$emit('toggle-pause')"
        class="shadow-md"
        :title="isPaused ? '播放' : '暂停'"
      >
        <Lock v-if="pauseLocked && !isPaused" class="w-5 h-5" />
        <template v-else>
          <Play v-if="isPaused" class="w-5 h-5 fill-current ml-0.5" />
          <Pause v-else class="w-5 h-5 fill-current" />
        </template>
      </IconButton>
    </div>

    <IconButton
      size="md"
      variant="ghost"
      :disabled="skipLocked"
      @click="$emit('next')"
      title="下一首"
    >
      <SkipForward class="w-5 h-5 fill-current" />
    </IconButton>

    <IconButton
      size="md"
      variant="ghost"
      @click="$emit('toggle-repeat')"
      title="循环播放"
    >
      <Repeat class="w-4 h-4" />
    </IconButton>
  </div>
</template>

<script setup>
import { Shuffle, SkipBack, SkipForward, Play, Pause, Lock, Repeat } from 'lucide-vue-next';
import IconButton from './IconButton.vue';

defineProps({
  isPaused: Boolean,
  shuffle: Boolean,
  shuffleLocked: Boolean,
  pauseLocked: Boolean,
  skipLocked: Boolean
});

defineEmits(['toggle-pause', 'toggle-shuffle', 'next', 'prev', 'toggle-repeat']);
</script>
