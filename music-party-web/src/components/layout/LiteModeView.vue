<template>
  <div class="fixed inset-0 z-[var(--z-overlay)] flex flex-col items-center justify-center bg-[radial-gradient(circle_at_top,rgba(211,194,243,0.08),transparent_40%),var(--surface-0)] p-6">
    <div class="relative z-10 w-full max-w-lg flex flex-col items-center gap-10">
      <!-- Status -->
      <div class="flex items-center gap-3 text-xs text-[var(--text-tertiary)] tracking-[0.2em] uppercase font-bold">
        <Activity class="w-3 h-3 text-[var(--accent)]" />
        <span>精简模式 (LITE)</span>
      </div>

      <!-- Main Card -->
      <div class="w-full bg-[var(--surface-1)] border border-[var(--border-default)] shadow-2xl p-8 rounded-[var(--radius-lg)]">
        <div class="flex flex-col md:flex-row items-center gap-8">
          <div class="w-28 h-28 flex-shrink-0 overflow-hidden rounded-[var(--radius-md)] border border-[var(--border-default)] shadow-inner">
            <CoverImage :src="nowPlaying?.music.coverUrl" :alt="nowPlaying?.music.name" class="w-full h-full object-cover" />
          </div>

          <div class="flex-1 min-w-0 flex flex-col items-center md:items-start text-center md:text-left">
            <span class="mb-1 text-[10px] font-bold uppercase tracking-widest text-[var(--accent)]">Now Playing</span>
            <h2 class="text-2xl font-bold text-[var(--text-primary)] leading-tight mb-1 line-clamp-2">
              {{ nowPlaying?.music.name || 'Room Idle' }}
            </h2>
            <div class="text-sm font-medium text-[var(--text-secondary)]">
              {{ nowPlaying?.music.artists.join(' / ') || 'Music Party' }}
            </div>
          </div>
        </div>
      </div>

      <!-- Volume -->
      <div class="w-full max-w-[320px] bg-[var(--surface-2)] border border-[var(--border-default)] p-4 flex flex-col gap-3 rounded-[var(--radius-lg)]">
        <div class="flex justify-between items-center text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider">
          <span>Volume</span>
          <span class="text-[var(--text-primary)]">{{ Math.round(volume * 100) }}%</span>
        </div>
        <div class="flex items-center gap-3">
          <Volume2 class="w-4 h-4 text-[var(--text-tertiary)]" />
          <input
            type="range" min="0" max="1" step="0.01"
            :value="volume"
            @input="$emit('update:volume', parseFloat($event.target.value))"
            class="w-full h-1 appearance-none cursor-pointer accent-[var(--accent)] bg-[var(--progress-track)] rounded-full"
          />
        </div>
      </div>
      
      <!-- Auto-Lite Toggle -->
      <label class="flex items-center gap-3 cursor-pointer group select-none">
        <div class="relative w-9 h-5 rounded-full transition-colors duration-300"
          :class="autoLite ? 'bg-[var(--accent)]' : 'bg-[var(--surface-3)]'">
          <input type="checkbox" :checked="autoLite" @change="$emit('update:autoLite', $event.target.checked)" class="hidden" />
          <div class="absolute left-1 top-1 w-3 h-3 bg-white rounded-full transition-transform duration-300"
            :style="{ transform: autoLite ? 'translateX(16px)' : 'translateX(0)' }"></div>
        </div>
        <span class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider">
          Auto-Lite when inactive
        </span>
      </label>

      <!-- Exit -->
      <button
        @click="$emit('exit')"
        class="w-full min-h-[56px] bg-transparent text-[var(--text-tertiary)] font-bold flex items-center justify-center gap-4 transition-all hover:text-[var(--text-primary)] hover:bg-[var(--surface-2)] active:scale-[0.98] rounded-[var(--radius-lg)] border border-[var(--border-default)]"
      >
        <Maximize2 class="w-5 h-5 opacity-60" />
        <span class="text-xs tracking-[0.2em] uppercase">Return to Main View</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { Activity, Volume2, Maximize2 } from 'lucide-vue-next';
import CoverImage from '../CoverImage.vue';

defineProps({
  nowPlaying: Object,
  volume: Number,
  autoLite: Boolean
});

defineEmits(['update:volume', 'update:autoLite', 'exit']);
</script>
