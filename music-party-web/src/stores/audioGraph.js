import { defineStore } from 'pinia';
import { computed, shallowRef } from 'vue';
import { useAudioFxStore } from './audioFx';

let sourceElement = null;

export const useAudioGraphStore = defineStore('audioGraph', () => {
  const audioElement = shallowRef(null);
  const context = shallowRef(null);
  const inputGain = shallowRef(null);
  const outputGain = shallowRef(null);
  const compressor = shallowRef(null);
  const analyser = shallowRef(null);
  const filters = shallowRef([]);
  const unavailableReason = shallowRef('');
  const diagnostics = shallowRef(null);

  const isAvailable = computed(() => !!context.value && !unavailableReason.value);

  const registerAudioElement = (audioEl) => {
    audioElement.value = audioEl || null;
  };

  const init = (audioEl = audioElement.value) => {
    if (!audioEl || context.value) return isAvailable.value;
    const AudioContextCtor = window.AudioContext || window.webkitAudioContext;
    if (!AudioContextCtor) {
      unavailableReason.value = 'Web Audio is not available in this browser.';
      return false;
    }
    try {
      diagnostics.value = {
        currentSrc: audioEl.currentSrc || audioEl.src || '',
        crossOrigin: audioEl.crossOrigin || '',
        readyState: audioEl.readyState,
        networkState: audioEl.networkState
      };
      const ctx = new AudioContextCtor();
      const fx = useAudioFxStore();
      const source = sourceElement === audioEl ? null : ctx.createMediaElementSource(audioEl);
      if (!source) {
        unavailableReason.value = 'Web Audio is already connected to another graph.';
        return false;
      }
      sourceElement = audioEl;
      inputGain.value = ctx.createGain();
      outputGain.value = ctx.createGain();
      compressor.value = ctx.createDynamicsCompressor();
      analyser.value = ctx.createAnalyser();
      analyser.value.fftSize = 2048;
      filters.value = fx.state.bands.map(band => {
        const filter = ctx.createBiquadFilter();
        filter.type = band.type;
        filter.frequency.value = band.frequency;
        filter.Q.value = band.q;
        filter.gain.value = 0;
        return filter;
      });
      const chain = [source, inputGain.value, ...filters.value, compressor.value, outputGain.value, analyser.value, ctx.destination].filter(Boolean);
      for (let i = 0; i < chain.length - 1; i++) chain[i].connect(chain[i + 1]);
      context.value = ctx;
      applySettings();
      return true;
    } catch (error) {
      unavailableReason.value = error?.message || 'Web Audio initialization failed.';
      return false;
    }
  };

  const reset = async () => {
    try {
      await context.value?.close();
    } catch {
      // Closing is best-effort; a page reload is the hard fallback once a media element source exists.
    }
    context.value = null;
    inputGain.value = null;
    outputGain.value = null;
    compressor.value = null;
    analyser.value = null;
    filters.value = [];
    unavailableReason.value = '';
    diagnostics.value = null;
    sourceElement = null;
  };

  const resume = async () => {
    if (context.value?.state === 'suspended') await context.value.resume();
  };

  const applySettings = () => {
    if (!context.value || !inputGain.value || !outputGain.value) return;
    const fx = useAudioFxStore().state;
    inputGain.value.gain.value = fx.enabled ? fx.preGain : 1;
    outputGain.value.gain.value = fx.enabled ? fx.outputGain : 1;
    filters.value.forEach((filter, index) => {
      filter.gain.value = fx.enabled && fx.eqEnabled ? fx.bands[index].gain : 0;
    });
    if (compressor.value) {
      compressor.value.threshold.value = fx.enabled && fx.compressorEnabled ? fx.compressor.threshold : 0;
      compressor.value.knee.value = fx.compressor.knee;
      compressor.value.ratio.value = fx.enabled && fx.compressorEnabled ? fx.compressor.ratio : 1;
      compressor.value.attack.value = fx.compressor.attack;
      compressor.value.release.value = fx.compressor.release;
    }
  };

  return { audioElement, context, analyser, unavailableReason, diagnostics, isAvailable, registerAudioElement, init, resume, reset, applySettings };
});
