import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { beforeEach, describe, expect, it } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useAudioPlaybackStore } from '../domains/playback/audioPlaybackStore';
import { useRoomRealtimeCoordinator } from '../domains/realtime/roomRealtimeCoordinator';

const readSrc = (rel) => readFileSync(resolve(process.cwd(), rel), 'utf8');

describe('buffering progress + reload position restore', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
  });

  it('player store exposes reset bufferedMs state', () => {
    const player = useAudioPlaybackStore();
    const coordinator = useRoomRealtimeCoordinator();
    expect(player.bufferedMs).toBe(0);
    player.bufferedMs = 12345;
    expect(player.bufferedMs).toBe(12345);
    coordinator.resetRoomState();
    expect(player.bufferedMs).toBe(0);
  });

  it('useAudio tracks buffered ranges and restores position after reload (source-level)', () => {
    const src = readSrc('src/composables/useAudio.js');
    // 缓冲进度采集
    expect(src).toContain('updateBufferedMs');
    expect(src).toContain('audio.buffered');
    expect(src).toContain('bufferedMs');
    // 重载后位置恢复，避免从开头播放
    expect(src).toContain('pendingResumePositionMs');
    expect(src).toContain('audio.currentTime = restoreMs / 1000');
    // 重试路径不再直接 safePlay（由 canplay -> checkAutoPlay 恢复进度后再播放）
    expect(src).not.toContain('currentAudio.load();\n            safePlay();');
  });

  it('ProgressScrubber exposes a loaded (buffered) range layer', () => {
    const src = readSrc('src/components/ui/ProgressScrubber.vue');
    expect(src).toContain('loadedMs');
    expect(src).toContain('loadedPercent');
    expect(src).toContain('progress-loaded');
  });

  it('NowPlayingModule renders the buffered range behind the played range', () => {
    const src = readSrc('src/components/modules/NowPlayingModule.vue');
    expect(src).toContain('bufferedPercent');
    expect(src).toContain('progress-loaded');
  });

  it('PlayerControl renders the buffered range behind the played range', () => {
    const src = readSrc('src/components/PlayerControl.vue');
    expect(src).toContain('bufferedPercent');
    expect(src).toContain('audio.bufferedMs');
  });

  it('useNowPlayingViewModel exposes bufferedPercent', () => {
    const src = readSrc('src/composables/useNowPlayingViewModel.ts');
    expect(src).toContain('bufferedPercent');
    expect(src).toContain('audio.bufferedMs');
  });

  it('useAudio implements three-level soft retry (L1 wait / L2 soft retry / L3 hard reload)', () => {
    const src = readSrc('src/composables/useAudio.js');
    // 三级超时常量
    expect(src).toContain('STALLED_L1_SOFT_WAIT_MS');
    expect(src).toContain('STALLED_L2_SOFT_RETRY_MS');
    expect(src).toContain('STALLED_L3_HARD_RELOAD_MS');
    // 旧的单一 7s watchdog 常量已移除
    expect(src).not.toContain('STALLED_WATCHDOG_MS');
    // 三级实现函数
    expect(src).toContain('softRetrySource');
    expect(src).toContain('hardReloadSource');
    // L2 软重试用 src=src 保留缓冲，不是 load()
    expect(src).toContain('audio.src = currentSrc');
    // L3 硬重载仍用 load() + pendingResumePositionMs 恢复
    expect(src).toContain('currentAudio.load()');
    // isActuallyStalled 过滤误判
    expect(src).toContain('isActuallyStalled');
    expect(src).toContain("reason === 'emptied'");
    expect(src).toContain("reason === 'suspend'");
  });

  it('AudioEngine filters stalled events via isActuallyStalled and drops emptied', () => {
    const src = readSrc('src/components/AudioEngine.vue');
    expect(src).toContain('isActuallyStalled');
    // emptied 事件不再触发 onPlaybackStalled
    expect(src).not.toContain("@emptied=\"onPlaybackStalled('emptied')\"");
  });
});
