// src/stores/ui.js
import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import { STORAGE_KEYS } from '../constants/keys';
import client from '../api/client';
import { musicApi } from '../api/music';

export const useUiStore = defineStore('ui', () => {
    const initialTheme = localStorage.getItem('theme') || 'dark';
    const isDarkMode = ref(initialTheme !== 'light');
    const locale = ref(localStorage.getItem('mp_locale') || 'en');
    const isLiteMode = ref(false);
    const forceMobileLayout = ref(localStorage.getItem('mp_force_mobile_layout') === 'true');
    const mobilePreviewWidth = ref(parseInt(localStorage.getItem('mp_mobile_preview_width') || '390', 10));
    const mobileActiveTab = ref(localStorage.getItem('mp_mobile_active_tab') || 'now');
    const mainStageScale = ref(Number(localStorage.getItem('mp_main_stage_scale') || 1));
    const globalZoomLevel = ref(Number(localStorage.getItem('mp_global_zoom_level') || 1));
    const volume = ref(parseFloat(localStorage.getItem(STORAGE_KEYS.VOLUME) || '0.5'));
    const showLyricTranslation = ref(localStorage.getItem(STORAGE_KEYS.LYRIC_TRANSLATION) !== 'false');
    const lyricAlignment = ref(localStorage.getItem('mp_lyric_alignment') || 'center');
    const autoLiteMode = ref(localStorage.getItem('mp_auto_lite_mode') === 'true'); // 默认 false

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

    const parseHexColor = (accent) => {
        if (!accent || !accent.startsWith('#') || accent.length !== 7) {
            return null;
        }
        return {
            r: parseInt(accent.slice(1, 3), 16),
            g: parseInt(accent.slice(3, 5), 16),
            b: parseInt(accent.slice(5, 7), 16)
        };
    };

    const toHex = ({ r, g, b }) => `#${[r, g, b].map(v => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, '0')).join('')}`;

    const toRgba = ({ r, g, b }, alpha) => `rgba(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)}, ${alpha})`;

    const relativeLuminance = ({ r, g, b }) => {
        const channel = (value) => {
            const normalized = value / 255;
            return normalized <= 0.03928 ? normalized / 12.92 : Math.pow((normalized + 0.055) / 1.055, 2.4);
        };
        return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
    };

    const mix = (color, target, amount) => ({
        r: color.r + (target.r - color.r) * amount,
        g: color.g + (target.g - color.g) * amount,
        b: color.b + (target.b - color.b) * amount
    });

    const normalizeAccentForTheme = (accentSet) => {
        const defaults = isDarkMode.value ? defaultAccentSet.dark : defaultAccentSet.light;
        const parsed = parseHexColor(accentSet?.accent);
        if (!parsed) return accentSet || defaults;

        let color = parsed;
        if (isDarkMode.value) {
            let guard = 0;
            while (relativeLuminance(color) < 0.32 && guard < 6) {
                color = mix(color, { r: 255, g: 255, b: 255 }, 0.18);
                guard++;
            }
        } else {
            let guard = 0;
            while (relativeLuminance(color) > 0.22 && guard < 10) {
                color = mix(color, { r: 0, g: 0, b: 0 }, 0.20);
                guard++;
            }
        }

        const hoverTarget = isDarkMode.value ? { r: 255, g: 255, b: 255 } : { r: 0, g: 0, b: 0 };
        const hover = mix(color, hoverTarget, isDarkMode.value ? 0.14 : 0.10);

        return {
            accent: toHex(color),
            accentHover: toHex(hover),
            accentMuted: toRgba(color, isDarkMode.value ? 0.18 : 0.14),
            accentSubtle: toRgba(color, isDarkMode.value ? 0.10 : 0.07),
            borderAccent: toRgba(color, 0.5)
        };
    };

    const deriveBorderAccent = (accent) => {
        const color = parseHexColor(accent);
        if (!color) {
            return isDarkMode.value ? defaultAccentSet.dark.borderAccent : defaultAccentSet.light.borderAccent;
        }
        const { r, g, b } = color;
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
        const active = normalizeAccentForTheme(dynamicAccent.value || defaults);

        root.style.setProperty('--accent', active.accent);
        root.style.setProperty('--accent-hover', active.accentHover);
        root.style.setProperty('--accent-muted', active.accentMuted);
        root.style.setProperty('--accent-subtle', active.accentSubtle);
        root.style.setProperty('--border-accent', active.borderAccent || deriveBorderAccent(active.accent));
    };

    const toggleLiteMode = () => {
        isLiteMode.value = !isLiteMode.value;
    };

    const setForceMobileLayout = (val) => {
        forceMobileLayout.value = !!val;
    };

    const toggleForceMobileLayout = () => {
        forceMobileLayout.value = !forceMobileLayout.value;
    };

    const setMobilePreviewWidth = (val) => {
        const next = Number(val);
        if (!Number.isFinite(next)) return;
        mobilePreviewWidth.value = Math.max(320, Math.min(768, next));
    };

    const setMobileActiveTab = (tab) => {
        if (['now', 'queue', 'search', 'chat'].includes(tab)) {
            mobileActiveTab.value = tab;
        }
    };

    const toggleDarkMode = () => {
        isDarkMode.value = !isDarkMode.value;
    };

    const setLocale = (newLocale) => {
        locale.value = newLocale;
    };

    const setVolume = (val) => {
        volume.value = Math.max(0, Math.min(1, val));
    };

    const toggleLyricTranslation = () => {
        showLyricTranslation.value = !showLyricTranslation.value;
    };

    const setLyricTranslation = (val) => {
        showLyricTranslation.value = !!val;
    };

    const setLyricAlignment = (val) => {
        if (['left', 'center', 'right'].includes(val)) {
            lyricAlignment.value = val;
        }
    };

    const setMainStageScale = (val) => {
        const num = parseFloat(val);
        if (!isNaN(num)) {
            mainStageScale.value = Math.max(0.90, Math.min(1.20, num));
        }
    };

    const setGlobalZoomLevel = (val) => {
        const num = parseFloat(val);
        if (!isNaN(num)) {
            globalZoomLevel.value = Math.max(1.0, Math.min(1.5, num));
        }
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
        const normalized = normalizeAccentForTheme(accentSet);
        dynamicAccent.value = {
            ...normalized,
            borderAccent: accentSet.borderAccent || normalized.borderAccent || deriveBorderAccent(accentSet.accent)
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

    watch(showLyricTranslation, (newVal) => {
        localStorage.setItem(STORAGE_KEYS.LYRIC_TRANSLATION, newVal.toString());
    });

    watch(autoLiteMode, (newVal) => {
        localStorage.setItem('mp_auto_lite_mode', newVal.toString());
    });

    watch(forceMobileLayout, (newVal) => {
        localStorage.setItem('mp_force_mobile_layout', newVal.toString());
    });

    watch(mobilePreviewWidth, (newVal) => {
        localStorage.setItem('mp_mobile_preview_width', newVal.toString());
    });

    watch(mobileActiveTab, (newVal) => {
        localStorage.setItem('mp_mobile_active_tab', newVal);
    });

    watch(mainStageScale, (newVal) => {
        localStorage.setItem('mp_main_stage_scale', newVal.toString());
    });

    watch(globalZoomLevel, (newVal) => {
        localStorage.setItem('mp_global_zoom_level', newVal.toString());
    });

    watch(locale, (newVal) => {
        localStorage.setItem('mp_locale', newVal);
    });

    watch(isDarkMode, (newVal) => {
        localStorage.setItem('theme', newVal ? 'dark' : 'light');
        syncThemeClass(newVal);
    }, { immediate: true });

    return {
        isLiteMode,
        toggleLiteMode,
        forceMobileLayout,
        setForceMobileLayout,
        toggleForceMobileLayout,
        mobilePreviewWidth,
        setMobilePreviewWidth,
        mobileActiveTab,
        setMobileActiveTab,
        mainStageScale,
        setMainStageScale,
        globalZoomLevel,
        setGlobalZoomLevel,
        isDarkMode,
        toggleDarkMode,
        locale,
        setLocale,
        volume,
        setVolume,
        showLyricTranslation,
        toggleLyricTranslation,
        setLyricTranslation,
        lyricAlignment,
        setLyricAlignment,
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
