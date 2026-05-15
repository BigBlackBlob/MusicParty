import { describe, expect, it } from 'vitest';
import { mergeTranslatedLyrics, parseLyrics } from './parser';

describe('parseLyrics', () => {
  it('parses and sorts timestamped lyric lines', () => {
    expect(parseLyrics('[00:02.50]B\n[00:01.000]A')).toEqual([
      { time: 1000, text: 'A' },
      { time: 2500, text: 'B' }
    ]);
  });

  it('merges translated lyrics by timestamp', () => {
    expect(mergeTranslatedLyrics('[00:01.00]Hello', '[00:01.00]你好')).toEqual([
      { time: 1000, text: 'Hello', translation: '你好' }
    ]);
  });
});
