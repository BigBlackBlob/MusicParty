import { describe, expect, it } from 'vitest';
import { isUserGestureRequiredError } from './audioPlayback';

describe('audio playback helpers', () => {
  it('detects iOS autoplay gesture failures', () => {
    expect(isUserGestureRequiredError(new DOMException('blocked', 'NotAllowedError'))).toBe(true);
    expect(isUserGestureRequiredError({ name: 'NotAllowedError' })).toBe(true);
    expect(isUserGestureRequiredError({ name: 'AbortError' })).toBe(false);
    expect(isUserGestureRequiredError(null)).toBe(false);
  });
});
