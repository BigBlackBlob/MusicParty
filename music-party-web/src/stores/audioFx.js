import { defineStore } from 'pinia';
import { ref, watch } from 'vue';

export const AUDIO_FX_STORAGE_KEY = 'mp-audio-fx-v1';

export const DEFAULT_AUDIO_FX = {
  experimentalGraphEnabled: false,
  enabled: false,
  eqEnabled: true,
  compressorEnabled: false,
  preGain: 1,
  outputGain: 1,
  bands: [
    { id: 'low', label: '60', frequency: 60, type: 'lowshelf', gain: 0, q: 0.7 },
    { id: 'lowMid', label: '250', frequency: 250, type: 'peaking', gain: 0, q: 1 },
    { id: 'mid', label: '1k', frequency: 1000, type: 'peaking', gain: 0, q: 1 },
    { id: 'presence', label: '4k', frequency: 4000, type: 'peaking', gain: 0, q: 1 },
    { id: 'air', label: '12k', frequency: 12000, type: 'highshelf', gain: 0, q: 0.7 }
  ],
  compressor: {
    threshold: -18,
    knee: 24,
    ratio: 3,
    attack: 0.003,
    release: 0.25
  }
};

const cloneDefaults = () => JSON.parse(JSON.stringify(DEFAULT_AUDIO_FX));

const PRESETS = {
  flat: cloneDefaults(),
  bass: {
    ...cloneDefaults(),
    enabled: true,
    bands: cloneDefaults().bands.map(band => ({ ...band, gain: band.id === 'low' ? 6 : band.id === 'lowMid' ? 2 : 0 }))
  },
  vocal: {
    ...cloneDefaults(),
    enabled: true,
    bands: cloneDefaults().bands.map(band => ({ ...band, gain: band.id === 'lowMid' ? -2 : band.id === 'presence' ? 4 : band.id === 'air' ? 2 : 0 }))
  },
  night: {
    ...cloneDefaults(),
    enabled: true,
    compressorEnabled: true,
    outputGain: 0.85,
    compressor: { threshold: -24, knee: 30, ratio: 5, attack: 0.006, release: 0.35 }
  }
};

const loadInitialState = () => {
  try {
    const saved = JSON.parse(localStorage.getItem(AUDIO_FX_STORAGE_KEY) || 'null');
    if (!saved || typeof saved !== 'object') return cloneDefaults();
    const defaults = cloneDefaults();
    return {
      ...defaults,
      ...saved,
      bands: defaults.bands.map(defaultBand => ({ ...defaultBand, ...(saved.bands || []).find(band => band.id === defaultBand.id) })),
      compressor: { ...defaults.compressor, ...(saved.compressor || {}) }
    };
  } catch {
    localStorage.removeItem(AUDIO_FX_STORAGE_KEY);
    return cloneDefaults();
  }
};

export const useAudioFxStore = defineStore('audioFx', () => {
  const state = ref(loadInitialState());

  const setBandGain = (id, gain) => {
    const band = state.value.bands.find(item => item.id === id);
    if (band) band.gain = Math.max(-12, Math.min(12, Number(gain) || 0));
  };

  const setCompressor = (key, value) => {
    if (!(key in state.value.compressor)) return;
    state.value.compressor[key] = Number(value);
  };

  const applyPreset = (preset) => {
    state.value = JSON.parse(JSON.stringify(PRESETS[preset] || PRESETS.flat));
  };

  const reset = () => {
    state.value = cloneDefaults();
  };

  watch(state, value => {
    localStorage.setItem(AUDIO_FX_STORAGE_KEY, JSON.stringify(value));
  }, { deep: true });

  return { state, presets: PRESETS, setBandGain, setCompressor, applyPreset, reset };
});
