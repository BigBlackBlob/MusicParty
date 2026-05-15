import { describe, expect, it } from 'vitest';
import { formatDuration } from './format';

describe('formatDuration', () => {
  it('formats milliseconds as mm:ss', () => {
    expect(formatDuration(65000)).toBe('01:05');
    expect(formatDuration(-1)).toBe('00:00');
  });
});
