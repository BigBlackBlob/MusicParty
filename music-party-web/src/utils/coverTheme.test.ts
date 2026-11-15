import { describe, expect, it } from 'vitest'
import { brandTheme, deriveCoverTheme } from './coverTheme'

// Independent WCAG calculation against the emitted, rounded CSS colors.
function luminance(value: string) {
  const channels = [1, 3, 5].map(offset => {
    const channel = parseInt(value.slice(offset, offset + 2), 16) / 255
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4
  })
  return channels[0]! * 0.2126 + channels[1]! * 0.7152 + channels[2]! * 0.0722
}

function contrast(first: string, second: string) {
  const a = luminance(first), b = luminance(second)
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
}

describe('cover theme', () => {
  it.each(['#2f8f8a', '#e07a5f'])('derives a distinct dark stage and readable accent for %s', cover => {
    const theme = deriveCoverTheme(cover, true)
    expect(theme.stageGradient).not.toBe(brandTheme.dark.stageGradient)
    expect(theme.stageGradient).toContain('#0a0b0b 100%')
    expect(contrast(theme.accent, theme.stageTint)).toBeGreaterThanOrEqual(4.5)
    expect(contrast('#ffffff', theme.stageTint)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(theme.accent, '#07140e')).toBeGreaterThanOrEqual(4.5)
    expect(theme.accentSubtle).toContain('0.1)')
  })

  it('keeps teal and red-orange hues separate', () => {
    const teal = deriveCoverTheme('#2f8f8a', true)
    const ember = deriveCoverTheme('#e07a5f', true)
    expect(teal.stageTint).not.toBe(ember.stageTint)
    expect(teal.accent).not.toBe(ember.accent)
  })

  it.each(['#ffffff', '#fdfcfb', '#010101', '#050308', '#777777', '#101010', '#808081'])('uses brand for achromatic or extreme cover %s', cover => {
    expect(deriveCoverTheme(cover, true)).toEqual(brandTheme.dark)
    expect(deriveCoverTheme(cover, false)).toEqual(brandTheme.light)
  })

  it('bounds a pale yellow stage instead of washing it gray', () => {
    const theme = deriveCoverTheme('#fff3bc', true)
    const channels = [1, 3, 5].map(offset => parseInt(theme.stageTint.slice(offset, offset + 2), 16))
    expect(Math.max(...channels)).toBeLessThan(85)
    expect(Math.max(...channels) - Math.min(...channels)).toBeGreaterThan(30)
    expect(contrast(theme.accent, theme.stageTint)).toBeGreaterThanOrEqual(4.5)
  })

  it.each([undefined, null, '', '#oops!!', '#123', 'rgb(1, 2, 3)'])('uses brand for missing or invalid extraction %s', cover => {
    expect(deriveCoverTheme(cover, true)).toEqual(brandTheme.dark)
  })

  it.each(['#2f8f8a', '#e07a5f', '#fff3bc', '#00ff00', '#0000ff', '#ff00ff'])('keeps light controls readable for %s', cover => {
    const theme = deriveCoverTheme(cover, false)
    expect(theme.stageGradient).toContain('#f7f6f3 100%')
    expect(contrast(theme.accent, theme.stageTint)).toBeGreaterThanOrEqual(4.5)
    expect(contrast(theme.accent, '#f7f6f3')).toBeGreaterThanOrEqual(4.5)
    expect(contrast(theme.accent, '#ffffff')).toBeGreaterThanOrEqual(4.5)
    expect(contrast('#626262', theme.stageTint)).toBeGreaterThanOrEqual(4.5)
  })

  it('preserves exact brand tokens', () => {
    expect(deriveCoverTheme(null, true).accent).toBe('#71bd99')
    expect(deriveCoverTheme(null, true).accentHover).toBe('#85c6a7')
    expect(deriveCoverTheme(null, false).accent).toBe('#387455')
    expect(deriveCoverTheme(null, false).accentHover).toBe('#32684d')
    expect(contrast(brandTheme.light.accent, '#e1ece5')).toBeGreaterThanOrEqual(4.5)
  })
})
