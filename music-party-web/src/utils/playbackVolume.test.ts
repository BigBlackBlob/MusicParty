import { describe, expect, it } from 'vitest'
import { getEffectiveVolume, parsePlatformVolumeOffsets } from './playbackVolume'

describe('effective playback volume', () => {
  it('combines the viewer volume, a dB offset, and the current fade', () => {
    expect(getEffectiveVolume(0.5)).toBe(0.5)
    expect(getEffectiveVolume(0.8, -6, 0.5)).toBeCloseTo(0.200475, 6)
    expect(getEffectiveVolume(0, -6, 0.5)).toBe(0)
  })

  it('keeps volume within the media element range', () => {
    expect(getEffectiveVolume(-1)).toBe(0)
    expect(getEffectiveVolume(2)).toBe(1)
    expect(getEffectiveVolume(NaN)).toBe(0.5)
  })
})

describe('stored platform volume offsets', () => {
  it.each([null, '{broken', 'null', '[]', '3'])('uses defaults for invalid storage: %s', raw => {
    expect(parsePlatformVolumeOffsets(raw)).toEqual({})
  })

  it('clamps and rounds numeric offsets, ignoring invalid values', () => {
    expect(parsePlatformVolumeOffsets('{"netease":-25,"soundcloud":-6.4,"qq":7,"local":"-4","youtube":null,"navidrome":1e999,"subsonic-test":-3}')).toEqual({
      netease: -12, soundcloud: -6, 'subsonic-test': -3,
    })
  })
})
