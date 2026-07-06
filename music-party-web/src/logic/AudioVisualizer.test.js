import { describe, expect, it } from 'vitest';
import { AudioVisualizer } from './AudioVisualizer';

describe('AudioVisualizer performance gating', () => {
  it('skips idle hidden frames once the visualizer is settled', () => {
    const visualizer = new AudioVisualizer();
    visualizer.isPlaying = false;
    visualizer.speedMultiplier = 1.001;
    visualizer.widthMultiplier = 1.001;
    visualizer.roughnessMultiplier = 1.001;
    visualizer.smoothAlpha = 0.051;
    visualizer.smoothWidthScale = 0.301;

    expect(visualizer.shouldDrawFrame({ visibilityState: 'hidden' })).toBe(false);
  });

  it('keeps drawing when playing or when an impulse is still decaying', () => {
    const visualizer = new AudioVisualizer();

    visualizer.isPlaying = true;
    expect(visualizer.shouldDrawFrame({ visibilityState: 'hidden' })).toBe(true);

    visualizer.isPlaying = false;
    visualizer.speedMultiplier = 1.5;
    expect(visualizer.shouldDrawFrame({ visibilityState: 'hidden' })).toBe(true);
  });

  it('keeps visible idle frames alive at a reduced cadence', () => {
    const visualizer = new AudioVisualizer();
    visualizer.isPlaying = false;
    visualizer.speedMultiplier = 1.001;
    visualizer.widthMultiplier = 1.001;
    visualizer.roughnessMultiplier = 1.001;
    visualizer.smoothAlpha = 0.051;
    visualizer.smoothWidthScale = 0.301;
    visualizer.lastIdleFrameAt = 1000;

    expect(visualizer.shouldDrawFrame({ visibilityState: 'visible', now: 1100 })).toBe(false);
    expect(visualizer.shouldDrawFrame({ visibilityState: 'visible', now: 1300 })).toBe(true);
  });
});
