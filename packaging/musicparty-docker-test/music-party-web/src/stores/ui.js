// src/stores/ui.js
import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import { STORAGE_KEYS } from '../constants/keys';
import client from '../api/client';
import { musicApi } from '../api/music';

export const useUiStore = defineStore('ui', () => {
    const initialTheme = localStorage.getItem('theme') || 'dark';
    const isDarkMode = ref(initialTheme !== 'light');
    const isLiteMode = ref(false);
    const volume = ref(parseFloat(localStorage.getItem(STORAGE_KEYS.VOLUME) || '0.5'));
    const autoLiteMode = ref(localStorage.getItem('mp_auto_lite_mode') !== 'false'); // 默认 true

    const authorName = ref('ThorNex');
    const backWords = ref('THORNEX');
    const dynamicAccent = ref(null);
    const lastAccentCoverUrl = ref('');

    const defaultAccentSet = {
        dark: {
            accent: '#d3c2f3',
            accentHover: '#e0d4f7',
            accentMuted: 'rgba(211, 194, 243, 0.15)',
            accentSubtle: 'rgba(211, 194, 243, 0.08)',
            borderAccent: 'rgba(211, 194, 243, 0.5)'
        },
        light: {
            accent: '#7c5cbf',
            accentHover: '#8d6ec8',
            accentMuted: 'rgba(124, 92, 191, 0.12)',
            accentSubtle: 'rgba(124, 92, 191, 0.06)',
            borderAccent: 'rgba(124, 92, 191, 0.5)'
        }
    };

    const deriveBorderAccent = (accent) => {
        if (!accent || !accent.startsWith('#') || accent.length !== 7) {
            return isDarkMode.value ? defaultAccentSet.dark.borderAccent : defaultAccentSet.light.borderAccent;
        }
        const r = parseInt(accent.slice(1, 3), 16);
        const g = parseInt(accent.slice(3, 5), 16);
        const b = parseInt(accent.slice(5, 7), 16);
        return `rgba(${r}, ${g}, ${b}, 0.5)`;
    };

    const syncThemeClass = (val) => {
        const root = document.documentElement;
        root.classList.toggle('dark', val);
        root.classList.toggle('light', !val);
        syncAccentVariables();
    };

    const syncAccentVariables = () => {
        const root = document.documentElement;
        const defaults = isDarkMode.value ? defaultAccentSet.dark : defaultAccentSet.light;
        const active = dynamicAccent.value || defaults;

        root.style.setProperty('--accent', active.accent);
        root.style.setProperty('--accent-hover', active.accentHover);
        root.style.setProperty('--accent-muted', active.accentMuted);
        root.style.setProperty('--accent-subtle', active.accentSubtle);
        root.style.setProperty('--border-accent', active.borderAccent || deriveBorderAccent(active.accent));
    };

    const toggleLiteMode = () => {
        isLiteMode.value = !isLiteMode.value;
    };

    const toggleDarkMode = () => {
        isDarkMode.value = !isDarkMode.value;
    };

    const setVolume = (val) => {
        volume.value = Math.max(0, Math.min(1, val));
    };

    const fetchConfig = async () => {
        try {
            const config = await client.get('/api/config');
            authorName.value = config.authorName;
            backWords.value = config.backWords;
        } catch (e) {
            console.error('Failed to fetch config', e);
        }
    };

    const clearDynamicAccent = () => {
        lastAccentCoverUrl.value = '';
        dynamicAccent.value = null;
        syncAccentVariables();
    };

    const setDynamicAccent = (accentSet) => {
        dynamicAccent.value = {
            ...accentSet,
            borderAccent: accentSet.borderAccent || deriveBorderAccent(accentSet.accent)
        };
        syncAccentVariables();
    };

    const updateAccentFromCover = async (coverUrl) => {
        if (!coverUrl) {
            clearDynamicAccent();
            return;
        }

        if (coverUrl === lastAccentCoverUrl.value && dynamicAccent.value) {
            return;
        }

        try {
            const accentSet = await musicApi.extractCoverColor(coverUrl);
            if (accentSet?.accent) {
                lastAccentCoverUrl.value = coverUrl;
                setDynamicAccent(accentSet);
            } else {
                clearDynamicAccent();
            }
        } catch (e) {
            console.warn('Failed to update accent from cover', e);
            clearDynamicAccent();
        }
    };

    // 监听音量变化并持久化
    watch(volume, (newVal) => {
        localStorage.setItem(STORAGE_KEYS.VOLUME, newVal.toString());
    });

    watch(autoLiteMode, (newVal) => {
        localStorage.setItem('mp_auto_lite_mode', newVal.toString());
    });

    watch(isDarkMode, (newVal) => {
        localStorage.setItem('theme', newVal ? 'dark' : 'light');
        syncThemeClass(newVal);
    }, { immediate: true });

    return {
        isLiteMode,
        toggleLiteMode,
        isDarkMode,
        toggleDarkMode,
        volume,
        setVolume,
        autoLiteMode,
        authorName,
        backWords,
        fetchConfig,
        dynamicAccent,
        clearDynamicAccent,
        setDynamicAccent,
        updateAccentFromCover
    };
});
