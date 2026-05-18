<template>
  <div class="audio-fx-module flex flex-col h-full overflow-hidden bg-surface-panel/30">
    <!-- Header -->
    <header class="flex items-center justify-between px-4 py-3 border-b border-border-subtle bg-surface-panel/50">
      <div class="flex flex-col">
        <span class="text-[10px] font-black uppercase tracking-[0.2em] text-text-muted leading-none mb-1">{{ t('audioFx.kicker') }}</span>
        <h2 class="text-sm font-bold uppercase tracking-widest text-text-primary">{{ t('audioFx.title') }}</h2>
      </div>
      
      <button 
        @click="fx.state.enabled = !fx.state.enabled"
        class="group relative flex items-center gap-2 px-3 py-1.5 rounded-md border transition-all duration-200"
        :class="fx.state.enabled 
          ? 'bg-primary/10 border-primary/50 text-primary' 
          : 'bg-surface-raised border-border-default text-text-muted hover:border-border-strong'"
      >
        <span class="flex h-1.5 w-1.5 rounded-full" :class="fx.state.enabled ? 'bg-primary animate-pulse' : 'bg-text-disabled'"></span>
        <span class="text-[10px] font-bold uppercase tracking-wider">
          {{ fx.state.enabled ? t('audioFx.enabled') : t('audioFx.bypass') }}
        </span>
      </button>
    </header>

    <div class="flex-1 overflow-y-auto p-4 space-y-6 scrollbar-none">
      <!-- Diagnostics / Experimental (Compact) -->
      <section v-if="!audioGraph.isAvailable || audioGraph.diagnostics" class="p-3 rounded-lg bg-error/5 border border-error/20">
        <p v-if="!audioGraph.isAvailable" class="text-xs text-error font-medium flex items-center gap-2">
          <span class="material-symbols-outlined text-[16px]">warning</span>
          {{ t('audioFx.unavailable') }}
        </p>
        <div v-if="audioGraph.diagnostics" class="mt-2 font-mono text-[9px] text-text-muted opacity-80 uppercase tracking-tighter">
          {{ t('audioFx.diagnostics') }}: {{ audioGraph.diagnostics.readyState }} · {{ audioGraph.diagnostics.networkState }}
        </div>
      </section>

      <!-- Experimental Toggle -->
      <section class="flex items-center justify-between p-3 rounded-xl bg-surface-raised/40 border border-border-subtle">
        <div class="flex flex-col gap-0.5">
          <span class="text-[10px] font-bold text-text-primary uppercase tracking-wide">{{ t('audioFx.experimentalGraph') }}</span>
          <span class="text-[9px] text-text-muted leading-tight max-w-[160px]">Enable advanced WebAudio routing</span>
        </div>
        <button 
          @click="fx.state.experimentalGraphEnabled ? disableGraph() : (fx.state.experimentalGraphEnabled = true)"
          class="w-10 h-5 rounded-full relative transition-colors duration-200"
          :class="fx.state.experimentalGraphEnabled ? 'bg-primary' : 'bg-surface-raised border border-border-default'"
        >
          <div 
            class="absolute top-1 left-1 w-3 h-3 rounded-full bg-white transition-transform duration-200"
            :style="{ transform: fx.state.experimentalGraphEnabled ? 'translateX(20px)' : 'translateX(0)' }"
          ></div>
        </button>
      </section>

      <!-- Presets -->
      <section>
        <h3 class="text-[10px] font-black uppercase tracking-[0.15em] text-text-muted mb-3 flex items-center gap-2">
          <span class="h-px w-4 bg-border-default"></span>
          {{ t('audioFx.presets.flat') }} & MODES
        </h3>
        <div class="grid grid-cols-2 gap-2">
          <button 
            v-for="p in ['flat', 'bass', 'vocal', 'night']" 
            :key="p"
            @click="fx.applyPreset(p)"
            class="px-3 py-2 rounded-lg border border-border-default bg-surface-raised/30 text-[10px] font-bold uppercase tracking-wider text-text-secondary hover:bg-surface-raised hover:text-text-primary transition-all"
          >
            {{ t(`audioFx.presets.${p}`) }}
          </button>
        </div>
      </section>

      <!-- Equalizer (Console Style) -->
      <section class="space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-[10px] font-black uppercase tracking-[0.15em] text-text-muted flex items-center gap-2">
            <span class="h-px w-4 bg-border-default"></span>
            {{ t('audioFx.eq') }}
          </h3>
          <button 
            @click="fx.state.eqEnabled = !fx.state.eqEnabled"
            class="text-[9px] font-bold uppercase tracking-widest transition-colors"
            :class="fx.state.eqEnabled ? 'text-primary' : 'text-text-disabled'"
          >
            {{ fx.state.eqEnabled ? 'ACTIVE' : 'OFF' }}
          </button>
        </div>

        <div class="eq-console flex justify-between items-end h-[180px] bg-surface-panel/20 rounded-xl p-4 border border-border-subtle" :class="{ 'opacity-40 grayscale pointer-events-none': !fx.state.eqEnabled }">
          <div v-for="band in fx.state.bands" :key="band.id" class="flex flex-col items-center gap-3 flex-1">
            <span class="font-mono text-[9px] text-text-muted tabular-nums">
              {{ band.gain > 0 ? '+' : '' }}{{ band.gain }}
            </span>
            
            <div class="relative w-6 h-full flex justify-center group">
              <!-- Slider Track -->
              <div class="absolute inset-y-0 w-1 bg-surface-raised rounded-full"></div>
              <!-- Active Range -->
              <div 
                class="absolute bottom-1/2 w-1 bg-primary rounded-full transition-all duration-150"
                :style="{ 
                  height: `${Math.abs(band.gain) / 12 * 50}%`,
                  bottom: band.gain >= 0 ? '50%' : `calc(50% - ${Math.abs(band.gain) / 12 * 50}%)`
                }"
              ></div>
              <!-- Input Handle (Invisible range input) -->
              <input
                type="range"
                min="-12"
                max="12"
                step="1"
                :value="band.gain"
                class="absolute inset-0 w-full h-full opacity-0 cursor-ns-resize z-10"
                style="writing-mode: vertical-lr; direction: rtl;"
                @input="fx.setBandGain(band.id, parseInt($event.target.value))"
              >
              <!-- Visual Thumb -->
              <div 
                class="absolute left-1/2 -translate-x-1/2 w-3.5 h-1.5 bg-text-primary border border-white/20 rounded-sm shadow-sm transition-all duration-150 pointer-events-none z-0"
                :style="{ bottom: `calc(${(band.gain + 12) / 24 * 100}% - 3px)` }"
              ></div>
            </div>
            
            <strong class="text-[9px] font-black uppercase tracking-tighter text-text-muted">
              {{ band.label }}
            </strong>
          </div>
        </div>
      </section>

      <!-- Dynamics (Compressor) -->
      <section class="space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-[10px] font-black uppercase tracking-[0.15em] text-text-muted flex items-center gap-2">
            <span class="h-px w-4 bg-border-default"></span>
            {{ t('audioFx.compressor') }}
          </h3>
          <button 
            @click="fx.state.compressorEnabled = !fx.state.compressorEnabled"
            class="text-[9px] font-bold uppercase tracking-widest transition-colors"
            :class="fx.state.compressorEnabled ? 'text-primary' : 'text-text-disabled'"
          >
            {{ fx.state.compressorEnabled ? 'ACTIVE' : 'OFF' }}
          </button>
        </div>

        <div class="grid grid-cols-1 gap-4 p-4 rounded-xl bg-surface-panel/20 border border-border-subtle" :class="{ 'opacity-40 grayscale pointer-events-none': !fx.state.compressorEnabled }">
          <!-- Threshold -->
          <div class="space-y-1.5">
            <div class="flex justify-between text-[9px] font-bold uppercase tracking-widest text-text-secondary">
              <span>{{ t('audioFx.threshold') }}</span>
              <span class="font-mono text-primary">{{ fx.state.compressor.threshold }}dB</span>
            </div>
            <input 
              v-model.number="fx.state.compressor.threshold" 
              type="range" min="-48" max="0"
              class="custom-range"
            >
          </div>
          
          <!-- Ratio -->
          <div class="space-y-1.5">
            <div class="flex justify-between text-[9px] font-bold uppercase tracking-widest text-text-secondary">
              <span>{{ t('audioFx.ratio') }}</span>
              <span class="font-mono text-primary">{{ fx.state.compressor.ratio }}:1</span>
            </div>
            <input 
              v-model.number="fx.state.compressor.ratio" 
              type="range" min="1" max="12" step="0.5"
              class="custom-range"
            >
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- Attack -->
            <div class="space-y-1.5">
              <div class="flex justify-between text-[9px] font-bold uppercase tracking-widest text-text-secondary">
                <span>{{ t('audioFx.attack') }}</span>
                <span class="font-mono text-primary">{{ (fx.state.compressor.attack * 1000).toFixed(0) }}ms</span>
              </div>
              <input 
                v-model.number="fx.state.compressor.attack" 
                type="range" min="0.001" max="0.05" step="0.001"
                class="custom-range"
              >
            </div>
            <!-- Release -->
            <div class="space-y-1.5">
              <div class="flex justify-between text-[9px] font-bold uppercase tracking-widest text-text-secondary">
                <span>{{ t('audioFx.release') }}</span>
                <span class="font-mono text-primary">{{ (fx.state.compressor.release * 1000).toFixed(0) }}ms</span>
              </div>
              <input 
                v-model.number="fx.state.compressor.release" 
                type="range" min="0.05" max="1" step="0.01"
                class="custom-range"
              >
            </div>
          </div>
        </div>
      </section>
    </div>

    <!-- Footer -->
    <footer class="p-3 border-t border-border-subtle bg-surface-panel/50 flex justify-between items-center">
      <div v-if="fx.state.experimentalGraphEnabled" class="flex gap-2">
        <button @click="enableGraph" class="text-[9px] font-black uppercase tracking-widest text-primary hover:underline">{{ t('audioFx.enableGraph') }}</button>
      </div>
      <button 
        @click="fx.reset()"
        class="ml-auto text-[9px] font-black uppercase tracking-[0.2em] text-text-muted hover:text-text-primary transition-colors"
      >
        {{ t('common.reset') }}
      </button>
    </footer>
  </div>
</template>

<script setup>
import { watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useAudioFxStore } from '../../stores/audioFx';
import { useAudioGraphStore } from '../../stores/audioGraph';

const fx = useAudioFxStore();
const audioGraph = useAudioGraphStore();
const { t } = useI18n();

const enableGraph = async () => {
  if (!fx.state.experimentalGraphEnabled) return;
  audioGraph.init();
  await audioGraph.resume().catch(() => {});
  audioGraph.applySettings();
};

const disableGraph = async () => {
  fx.state.experimentalGraphEnabled = false;
  fx.state.enabled = false;
  await audioGraph.reset();
  window.location.reload();
};

watch(() => fx.state, () => {
  if (fx.state.experimentalGraphEnabled) {
    audioGraph.applySettings();
  }
}, { deep: true, immediate: true });
</script>

<style scoped>
.scrollbar-none::-webkit-scrollbar {
  display: none;
}
.scrollbar-none {
  scrollbar-width: none;
}

/* Custom Range Styling */
.custom-range {
  @apply w-full h-1 bg-surface-raised rounded-full appearance-none cursor-pointer accent-primary outline-none transition-all;
}
.custom-range::-webkit-slider-thumb {
  @apply appearance-none w-3 h-3 rounded-full bg-text-primary border border-white/20 shadow-sm transition-transform hover:scale-110 active:scale-95;
}
.custom-range::-moz-range-thumb {
  @apply appearance-none w-3 h-3 rounded-full bg-text-primary border border-white/20 shadow-sm transition-transform hover:scale-110 active:scale-95 border-none;
}

/* EQ Slider Vertical */
.eq-console input[type=range] {
  -webkit-appearance: none;
  background: transparent;
}
.eq-console input[type=range]:focus {
  outline: none;
}
</style>

