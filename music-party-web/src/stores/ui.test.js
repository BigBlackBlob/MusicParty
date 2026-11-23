import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { nextTick } from 'vue';
import { STORAGE_KEYS } from '../constants/keys';
import { brandTheme, deriveCoverTheme } from '../utils/coverTheme';

// Mock the API client and music API
vi.mock('../transport/httpClient', () => ({ httpClient: { get: vi.fn() } }));
vi.mock('../api/music', () => ({
    musicApi: {
        extractCoverColor: vi.fn().mockResolvedValue({ accent: '#ff0000', borderAccent: '#cc0000' }),
        getLyric: vi.fn(),
        getLyricDetail: vi.fn(),
    }
}));

import { useUiStore } from './ui';
import { musicApi } from '../api/music';

it('restores per-platform volume offsets without changing the viewer volume', async () => {
    const pinia = createPinia();
    const ui = useUiStore(pinia);
    ui.setVolume(0.8);
    ui.setPlatformVolumeOffset('netease', -6);
    ui.setPlatformVolumeOffset('soundcloud', -30);
    await nextTick();
    ui.$dispose();

    const restored = useUiStore(createPinia());
    expect(restored.volume).toBe(0.8);
    expect(restored.getPlatformVolumeOffset('netease')).toBe(-6);
    expect(restored.getPlatformVolumeOffset('soundcloud')).toBe(-12);
    expect(restored.getPlatformVolumeOffset('youtube')).toBe(0);
    restored.setPlatformVolumeOffset('netease', 0);
    await nextTick();
    expect(JSON.parse(localStorage.getItem(STORAGE_KEYS.PLATFORM_VOLUME_OFFSETS))).toEqual({ soundcloud: -12 });
    restored.$dispose();
    localStorage.removeItem(STORAGE_KEYS.VOLUME);
    localStorage.removeItem(STORAGE_KEYS.PLATFORM_VOLUME_OFFSETS);
});

describe('ui store - cover accent requests', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.clearAllMocks();
        localStorage.setItem('theme', 'dark');
        musicApi.extractCoverColor.mockReset().mockResolvedValue({ accent: '#ff0000', borderAccent: '#cc0000' });
        vi.useFakeTimers();
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('extracts a new cover immediately even within 2s of the previous cover', async () => {
        const ui = useUiStore();
        const url = '/media/cover1.png';

        await ui.updateAccentFromCover(url);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);

        // The cover watcher fires once per URL, so dropping this call leaves stale colors.
        await ui.updateAccentFromCover('/media/cover2.png');
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(2);
    });

    it('deduplicates concurrent requests for the same cover', async () => {
        let resolve;
        musicApi.extractCoverColor.mockImplementationOnce(() => new Promise(done => { resolve = done; }));
        const ui = useUiStore();
        const first = ui.updateAccentFromCover('/media/cover.png');
        const duplicate = ui.updateAccentFromCover('/media/cover.png');
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);
        resolve({ accent: '#71bd99' });
        await Promise.all([first, duplicate]);
        expect(ui.dynamicAccent.accent).toBe('#71bd99');
    });

    it('ignores a stale failure after the next cover succeeds', async () => {
        let reject;
        musicApi.extractCoverColor.mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; }));
        const ui = useUiStore();
        const old = ui.updateAccentFromCover('/media/old.png');
        musicApi.extractCoverColor.mockResolvedValueOnce({ accent: '#71bd99' });
        await ui.updateAccentFromCover('/media/new.png');
        reject(new Error('old request failed'));
        await old;
        expect(ui.dynamicAccent.accent).toBe('#71bd99');
    });

    it('does not restore an accent after the cover is cleared', async () => {
        let resolve;
        musicApi.extractCoverColor.mockImplementationOnce(() => new Promise(done => { resolve = done; }));
        const ui = useUiStore();
        const pending = ui.updateAccentFromCover('/media/cover.png');
        await ui.updateAccentFromCover('');
        resolve({ accent: '#71bd99' });
        await pending;
        expect(ui.dynamicAccent).toBeNull();
    });

    it('skips request when URL unchanged and accent already set', async () => {
        const ui = useUiStore();
        const url = '/media/cover.png';

        await ui.updateAccentFromCover(url);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);

        // Same URL — should skip even after throttle window
        vi.advanceTimersByTime(3000);
        await ui.updateAccentFromCover(url);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);
    });

    it('clears accent when coverUrl is empty', async () => {
        const ui = useUiStore();
        await ui.updateAccentFromCover('');
        expect(ui.dynamicAccent).toBeNull();
        expect(musicApi.extractCoverColor).not.toHaveBeenCalled();
    });

    it.each([
        ['HTTP 400', () => Promise.reject(Object.assign(new Error('Only proxied cover URLs are supported'), { status: 400 }))],
        ['network failure', () => Promise.reject(new Error('Network unavailable'))],
        ['missing accent', () => Promise.resolve({})],
    ])('remembers a %s and keeps the default accent on remount', async (_, response) => {
        localStorage.setItem('theme', 'dark');
        const ui = useUiStore();
        await ui.updateAccentFromCover('/media/previous.png');
        musicApi.extractCoverColor.mockImplementationOnce(response);
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
        try {
            const unsupported = 'https://covers.musicparty.test/one.png';
            await ui.updateAccentFromCover(unsupported);
            await ui.updateAccentFromCover(unsupported);
            expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(2);
            expect(ui.dynamicAccent).toBeNull();
            expect(document.documentElement.style.getPropertyValue('--accent')).toBe('#71bd99');
            expect(document.documentElement.style.getPropertyValue('--stage-gradient')).toBe(brandTheme.dark.stageGradient);

            await ui.updateAccentFromCover('/media/next.png');
            expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(3);
            expect(ui.dynamicAccent).not.toBeNull();
        } finally {
            warn.mockRestore();
        }
    });

    it('rederives the stage and accent from the original extraction after theme switches', async () => {
        musicApi.extractCoverColor.mockResolvedValueOnce({ accent: '#2f8f8a' });
        const ui = useUiStore();
        await ui.updateAccentFromCover('/media/teal.png');
        const darkGradient = ui.stageGradient;
        expect(document.documentElement.style.getPropertyValue('--stage-gradient')).toBe(darkGradient);
        ui.toggleDarkMode();
        await nextTick();
        expect(document.documentElement.style.getPropertyValue('--accent')).toBe(deriveCoverTheme('#2f8f8a', false).accent);
        expect(ui.stageGradient).toBe(deriveCoverTheme('#2f8f8a', false).stageGradient);
        ui.toggleDarkMode();
        await nextTick();
        expect(ui.stageGradient).toBe(darkGradient);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);
    });
});
