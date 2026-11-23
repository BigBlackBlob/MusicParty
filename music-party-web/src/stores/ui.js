// src/stores/ui.js
import { defineStore } from 'pinia';
import { computed, ref, watch } from 'vue';
import { STORAGE_KEYS } from '../constants/keys';
import { httpClient as client } from '../transport/httpClient';
import { musicApi } from '../api/music';
import { withNavidromeResourceToken } from '../utils/audioUrl';
import { deriveCoverTheme } from '../utils/coverTheme';
import { getEffectiveVolume, normalizePlatformVolumeOffset, parsePlatformVolumeOffsets } from '../utils/playbackVolume';

export const useUiStore = defineStore('ui', () => {
    const mobileNowDensityOptions = ['compact', 'standard', 'relaxed'];
    const initialTheme = localStorage.getItem('theme') || 'dark';
    const isDarkMode = ref(initialTheme !== 'light');
    const locale = ref(localStorage.getItem('mp_locale') || 'en');
    localStorage.removeItem('mp_auto_lite_mode');
    const forceMobileLayout = ref(localStorage.getItem('mp_force_mobile_layout') === 'true');
    const mobilePreviewWidth = ref(parseInt(localStorage.getItem('mp_mobile_preview_width') || '390', 10));
    const mobileActiveTab = ref(localStorage.getItem('mp_mobile_active_tab') || 'now');
    const mobileNowDensity = ref(mobileNowDensityOptions.includes(localStorage.getItem('mp_mobile_now_density')) ? localStorage.getItem('mp_mobile_now_density') : 'standard');
    const mainStageScale = ref(Number(localStorage.getItem('mp_main_stage_scale') || 1));
    const globalZoomLevel = ref(Number(localStorage.getItem('mp_global_zoom_level') || 1));
    const volume = ref(getEffectiveVolume(parseFloat(localStorage.getItem(STORAGE_KEYS.VOLUME) || '0.5')));
    const platformVolumeOffsets = ref(parsePlatformVolumeOffsets(localStorage.getItem(STORAGE_KEYS.PLATFORM_VOLUME_OFFSETS)));
    const showLyricTranslation = ref(localStorage.getItem(STORAGE_KEYS.LYRIC_TRANSLATION) !== 'false');
    const showLyricRomanization = ref(localStorage.getItem(STORAGE_KEYS.LYRIC_ROMANIZATION) === 'true');
    const lyricAlignment = ref(localStorage.getItem('mp_lyric_alignment') || 'center');

    const authorName = ref('ThorNex');
    const backWords = ref('THORNEX');
    const dynamicAccent = ref(null);
    const lastAccentCoverUrl = ref('');

    const activeTheme = computed(() => deriveCoverTheme(dynamicAccent.value?.accent, isDarkMode.value));
    const stageGradient = computed(() => activeTheme.value.stageGradient);

    const syncThemeClass = (val) => {
        const root = document.documentElement;
        root.classList.toggle('dark', val);
        root.classList.toggle('light', !val);
        syncAccentVariables();
    };

    const syncAccentVariables = () => {
        const root = document.documentElement;
        const active = activeTheme.value;
        root.style.setProperty('--accent', active.accent);
        root.style.setProperty('--accent-hover', active.accentHover);
        root.style.setProperty('--accent-muted', active.accentMuted);
        root.style.setProperty('--accent-subtle', active.accentSubtle);
        root.style.setProperty('--border-accent', active.borderAccent);
        root.style.setProperty('--stage-tint', active.stageTint);
        root.style.setProperty('--stage-gradient', active.stageGradient);
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

    const setMobileNowDensity = (value) => {
        if (mobileNowDensityOptions.includes(value)) {
            mobileNowDensity.value = value;
        }
    };

    const toggleDarkMode = () => {
        isDarkMode.value = !isDarkMode.value;
    };

    const setLocale = (newLocale) => {
        locale.value = newLocale;
    };

    const setVolume = (val) => {
        volume.value = getEffectiveVolume(val);
    };

    /** @param {string | undefined} platform */
    const getPlatformVolumeOffset = (platform) => {
        if (!platform) return 0;
        const offsets = platformVolumeOffsets.value;
        return Object.hasOwn(offsets, platform) ? (offsets[platform] ?? 0) : 0;
    };

    /** @param {string} platform @param {number} offsetDb */
    const setPlatformVolumeOffset = (platform, offsetDb) => {
        const next = normalizePlatformVolumeOffset(offsetDb);
        if (next === getPlatformVolumeOffset(platform)) return;
        const offsets = { ...platformVolumeOffsets.value, [platform]: next };
        if (next === 0) delete offsets[platform];
        platformVolumeOffsets.value = offsets;
    };

    const toggleLyricTranslation = () => {
        showLyricTranslation.value = !showLyricTranslation.value;
    };

    const setLyricTranslation = (val) => {
        showLyricTranslation.value = !!val;
    };

    const toggleLyricRomanization = () => {
        showLyricRomanization.value = !showLyricRomanization.value;
    };

    const setLyricRomanization = (val) => {
        showLyricRomanization.value = !!val;
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

    let accentRequestRevision = 0;
    let pendingAccentUrl = '';

    const clearDynamicAccent = () => {
        accentRequestRevision++;
        pendingAccentUrl = '';
        lastAccentCoverUrl.value = '';
        dynamicAccent.value = null;
        syncAccentVariables();
    };

    const setDynamicAccent = (accentSet) => {
        // Keep the extracted color, not a theme-normalized result, for subsequent theme switches.
        dynamicAccent.value = accentSet;
        syncAccentVariables();
    };

    const updateAccentFromCover = async (coverUrl) => {
        if (!coverUrl) {
            clearDynamicAccent();
            return;
        }

        const requestUrl = withNavidromeResourceToken(coverUrl);

        if (requestUrl === pendingAccentUrl) return;
        const revision = ++accentRequestRevision;
        pendingAccentUrl = '';
        // Remember completed failures too, so remounted views keep the fallback.
        if (requestUrl === lastAccentCoverUrl.value) return;
        pendingAccentUrl = requestUrl;

        // Deduplicate broadcasts for one cover without dropping a different track's cover.
        try {
            const accentSet = await musicApi.extractCoverColor(requestUrl);
            if (revision !== accentRequestRevision) return;
            if (accentSet?.accent) {
                lastAccentCoverUrl.value = requestUrl;
                setDynamicAccent(accentSet);
            } else {
                clearDynamicAccent();
                lastAccentCoverUrl.value = requestUrl;
            }
        } catch (e) {
            if (revision !== accentRequestRevision) return;
            console.warn('Failed to update accent from cover', e);
            clearDynamicAccent();
            lastAccentCoverUrl.value = requestUrl;
        } finally {
            if (revision === accentRequestRevision) pendingAccentUrl = '';
        }
    };

    // 监听音量变化并持久化
    watch(platformVolumeOffsets, (offsets) => {
        try {
            localStorage.setItem(STORAGE_KEYS.PLATFORM_VOLUME_OFFSETS, JSON.stringify(offsets));
        } catch {
            // Keep the setting active for this session if storage is unavailable.
        }
    });

    watch(volume, (newVal) => {
        localStorage.setItem(STORAGE_KEYS.VOLUME, newVal.toString());
    });

    watch(showLyricTranslation, (newVal) => {
        localStorage.setItem(STORAGE_KEYS.LYRIC_TRANSLATION, newVal.toString());
    });

    watch(showLyricRomanization, (newVal) => {
        localStorage.setItem(STORAGE_KEYS.LYRIC_ROMANIZATION, newVal.toString());
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

    watch(mobileNowDensity, (newVal) => {
        localStorage.setItem('mp_mobile_now_density', newVal);
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
        forceMobileLayout,
        setForceMobileLayout,
        toggleForceMobileLayout,
        mobilePreviewWidth,
        setMobilePreviewWidth,
        mobileActiveTab,
        setMobileActiveTab,
        mobileNowDensity,
        setMobileNowDensity,
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
        platformVolumeOffsets,
        getPlatformVolumeOffset,
        setPlatformVolumeOffset,
        showLyricTranslation,
        toggleLyricTranslation,
        showLyricRomanization,
        toggleLyricRomanization,
        setLyricRomanization,
        setLyricTranslation,
        lyricAlignment,
        setLyricAlignment,
        authorName,
        backWords,
        fetchConfig,
        dynamicAccent,
        activeTheme,
        stageGradient,
        clearDynamicAccent,
        setDynamicAccent,
        updateAccentFromCover
    };
});

