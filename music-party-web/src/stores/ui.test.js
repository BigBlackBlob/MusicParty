import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';

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

describe('ui store - updateAccentFromCover throttle', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        vi.clearAllMocks();
        vi.useFakeTimers();
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('throttles repeated cover color requests within 2s', async () => {
        const ui = useUiStore();
        const url = '/media/cover1.png';

        await ui.updateAccentFromCover(url);
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);

        // Different URL but within throttle window — should be suppressed
        await ui.updateAccentFromCover('/media/cover2.png');
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(1);

        // Advance past throttle window
        vi.advanceTimersByTime(2100);
        await ui.updateAccentFromCover('/media/cover2.png');
        expect(musicApi.extractCoverColor).toHaveBeenCalledTimes(2);
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
});
