import { describe, expect, it } from 'vitest'
import { pairByTime, parseYrc } from './yrc'

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

  it('assigns a leading extra line to the first main line', () => {
    const main = parseYrc('[2000,3000](2000,1000,0)X')
    const extra = parseYrc('[0,500](0,500,0)intro')
    expect(pairByTime(main, extra)).toEqual(['intro'])
  })

  it('returns empty strings when no extra lines exist', () => {
    const main = parseYrc('[0,3000](0,1000,0)A')
    expect(pairByTime(main, [])).toEqual([''])
  })
})
