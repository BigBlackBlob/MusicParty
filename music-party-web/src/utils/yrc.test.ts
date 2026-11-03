import { describe, expect, it } from 'vitest'
import { pairByTime, pairLyricExtras, parseYrc } from './yrc'

const CLING_CLING_SAMPLE = [
  '{"t":0,"c":[{"tx":"作词: "},{"tx":"中田ヤスタカ"}]}',
  '[3580,3600](3580,490,0)Cling (4070,2750,0)Cling cling',
  '[7180,3610](7180,0,0)[0](7180,900,0)Kimi no '].join('\n')

describe('parseYrc', () => {
  it('parses line headers and word segments with absolute timings', () => {
    const lines = parseYrc(CLING_CLING_SAMPLE)
    expect(lines).toHaveLength(2)
    expect(lines[0].start).toBe(3580)
    expect(lines[0].end).toBe(7180)
    expect(lines[0].text).toBe('Cling Cling cling')
    expect(lines[0].segments).toEqual([
      { start: 3580, dur: 490, end: 4070, text: 'Cling ' },
      { start: 4070, dur: 2750, end: 6820, text: 'Cling cling' }
    ])
  })

  it('keeps zero-duration segments so word indexes stay verbatim', () => {
    const lines = parseYrc(CLING_CLING_SAMPLE)
    const second = lines[1]
    expect(second.segments[0]).toEqual({ start: 7180, dur: 0, end: 7180, text: '[0]' })
    expect(second.segments[1].text).toBe('Kimi no') // row trim drops line-final whitespace only
  })

  it('skips credit JSON lines, blank rows and non-YRC text', () => {
    expect(parseYrc('{"t":1,"c":[{"tx":"作曲: "}]}')).toEqual([])
    expect(parseYrc(''))
      .toEqual([])
    expect(parseYrc(null)).toEqual([])
  })

  it('treats header-only lines as one whole-line segment', () => {
    const lines = parseYrc('[1000,4000]plain line')
    expect(lines[0].segments).toEqual([{ start: 1000, dur: 4000, end: 5000, text: 'plain line' }])
  })

  it('accepts colon-timestamp LRC rows served inside word fields', () => {
    // Real shape of wordTranslatedLyric for 28816031 (Cling Cling):
    // ytlrc arrives as plain LRC, not segmented YRC.
    const lines = parseYrc('[by:DarkFantasyMaid]\n[00:03.580]Cling Cling 牵起手\n[00:07.21]你和我的开怀心情')
    expect(lines).toHaveLength(2)
    expect(lines[0].start).toBe(3580)
    expect(lines[0].end).toBe(3580)
    expect(lines[1].start).toBe(7210) // 2-digit fraction = hundredths
    expect(lines[0].text).toBe('Cling Cling 牵起手')
  })

  it('pairs colon-LRC extras into segmented mains by time window', () => {
    const main = parseYrc('[3580,3600](3580,490,0)Cling cling\n[7330,3370](7330,260,0)キミと')
    const extra = parseYrc('[00:03.580]牵起手\n[00:07.330]你和我的开怀心情')
    expect(pairByTime(main, extra)).toEqual(['牵起手', '你和我的开怀心情'])
  })
})

describe('pairByTime', () => {
  it('pairs extra lines by time window, not line number', () => {
    const main = parseYrc('[0,4000](0,2000,0)A\n[4000,4000](4000,2000,0)B\n[8000,4000](8000,2000,0)C')
    const extra = parseYrc('[5000,3000](5000,2000,0)译文二')
    expect(pairByTime(main, extra)).toEqual(['', '译文二', ''])
  })

  it('joins multiple extra lines that fall into one main window', () => {
    const main = parseYrc('[0,10000](0,2000,0)Long line')
    const extra = parseYrc('[1000,2000](1000,1000,0)one\n[5000,2000](5000,1000,0)two')
    expect(pairByTime(main, extra)).toEqual(['one two'])
  })

  it('leaves a distant leading extra unpaired', () => {
    const main = parseYrc('[2000,3000](2000,1000,0)X')
    const extra = parseYrc('[0,500](0,500,0)intro')
    expect(pairByTime(main, extra)).toEqual([''])
  })

  it('returns empty strings when no extra lines exist', () => {
    const main = parseYrc('[0,3000](0,1000,0)A')
    expect(pairByTime(main, [])).toEqual([''])
  })

  it('handles slightly early onsets and ignores distant trailing extras', () => {
    const main = parseYrc('[1000,3000]first\n[4000,2000]second')
    expect(pairByTime(main, parseYrc('[00:03.990]second translation\n[00:12.000]unrelated'))).toEqual(['', 'second translation'])
  })

  it('prefers an exact onset over an overlapping earlier window', () => {
    const main = parseYrc('[1000,10000]first\n[4000,2000]second')
    expect(pairByTime(main, parseYrc('[00:04.000]second translation'))).toEqual(['', 'second translation'])
  })
})

describe('pairLyricExtras', () => {
  // NetEase 1850706852, Secret (Your Fire): legacy and word timestamps drift independently.
  const main = '[134220,3060]You\'re like just what I like\n[138660,2040]Make another friend\n[142440,1350]You think you know me'
  const legacy = '[02:14.363]You\'re like just what I like\n[02:18.648]Make another friend\n[02:22.046]You think you know me'
  const extra = '[02:14.363]你正合我心意\n[02:18.648]结识新友吧\n[02:22.046]你自以为了解我'

  it('prefers word translations and uses legacy text anchors for missing rows', () => {
    expect(pairLyricExtras(parseYrc(main), '[02:14.220]逐字译文\n[02:22.440]你自以为了解我', legacy, extra)).toEqual(['逐字译文', '结识新友吧', '你自以为了解我'])
    expect(pairLyricExtras(parseYrc(main), '', legacy, extra)).toEqual(['你正合我心意', '结识新友吧', '你自以为了解我'])
  })

  it('does not fill an untranslated La line with the following Sickness translation', () => {
    const lines = parseYrc('[174120,2820]La la la la la\n[177510,2100]Sickness')
    expect(pairLyricExtras(lines, '[02:57.510]病态', '[02:54.186]La la la la la\n[02:57.497]Sickness', '[02:57.497]病态')).toEqual(['', '病态'])
  })

  it('matches repeated original texts in order even when timing differs by seconds', () => {
    const lines = parseYrc('[1000,1000]HELLO!\n[2000,1000]unmatched\n[20000,1000]Hello\n[30000,1000]Final line')
    const plain = '[00:01.100]Hello\n[00:20.200]Hello\n[00:36.000]Final line'
    const translated = '[00:01.100]first\n[00:20.200]second\n[00:36.000]final'
    expect(pairLyricExtras(lines, '', plain, translated)).toEqual(['first', '', 'second', 'final'])
  })
})
