import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApp, h, ref } from 'vue'
import { useAudio } from './useAudio'

let app

afterEach(() => {
  app?.unmount()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function engine() {
  const reportPlayback = vi.fn()
  const audio = { pause: vi.fn(), load: vi.fn(), play: vi.fn().mockResolvedValue(undefined), error: { code: 4 }, readyState: 0, networkState: 3 }
  const player = { nowPlaying: { playEpoch: 7, queueId: 'queue-7', music: { id: 'local-7', url: '/api/local/media/local-7', artists: [], platform: 'local' } }, isPaused: false, reportPlayback }
  let controls
  app = createApp({ setup() { controls = useAudio(ref(audio), player); return () => h('div') } })
  app.mount(document.createElement('div'))
  return { controls, reportPlayback }
}

describe('terminal playback errors', () => {
  it('reports retry exhaustion once for the current epoch', () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 404 }))
    const { controls, reportPlayback } = engine()
    for (let i = 0; i < 5; i++) controls.handleError()
    expect(reportPlayback).toHaveBeenCalledExactlyOnceWith('playback.error', { playEpoch: 7, queueId: 'queue-7', code: 'retries-exhausted' })
  })

  it.each(['stream-trial-only', 'stream-unavailable'])('reports a terminal 422 %s without waiting for retries', async code => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 422, json: async () => ({ code }) }))
    const { controls, reportPlayback } = engine()
    controls.handleError()
    await vi.waitFor(() => expect(reportPlayback).toHaveBeenCalledExactlyOnceWith('playback.error', { playEpoch: 7, queueId: 'queue-7', code }))
  })
})
