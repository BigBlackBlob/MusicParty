import { describe, expect, it } from 'vitest'
import { shouldUseYouTubeEmbed } from './audioEngine'

describe('YouTube playback engine selection', () => {
  it('uses native audio for a host-stream track until its load fails', () => {
    expect(shouldUseYouTubeEmbed({ platform: 'youtube', playerType: 'audio', playbackKey: 'q:1:id', fallbackKey: '' })).toBe(false)
  })

  it('falls back to the iframe only for the matching playback instance', () => {
    expect(shouldUseYouTubeEmbed({ platform: 'youtube', playerType: 'audio', playbackKey: 'q:1:id', fallbackKey: 'q:1:id' })).toBe(true)
    expect(shouldUseYouTubeEmbed({ platform: 'youtube', playerType: 'audio', playbackKey: 'q:2:id', fallbackKey: 'q:1:id' })).toBe(false)
  })

  it('keeps existing platform embeds and leaves other tracks native', () => {
    expect(shouldUseYouTubeEmbed({ platform: 'youtube', playerType: 'youtube-embed', playbackKey: 'q:1:id', fallbackKey: '' })).toBe(true)
    expect(shouldUseYouTubeEmbed({ platform: 'netease', playerType: 'audio', playbackKey: 'q:1:id', fallbackKey: '' })).toBe(false)
  })
})
