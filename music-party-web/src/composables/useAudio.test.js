import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApp, h, nextTick, ref } from 'vue'
import { useAudio } from './useAudio'

let app

afterEach(() => {
  app?.unmount()
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function engine(userVolume, offsetDb) {
  const reportPlayback = vi.fn()
  const audio = { pause: vi.fn(), load: vi.fn(), play: vi.fn().mockResolvedValue(undefined), error: { code: 4 }, readyState: 0, networkState: 3 }
  const player = { nowPlaying: { playEpoch: 7, queueId: 'queue-7', music: { id: 'local-7', url: '/api/local/media/local-7', artists: [], platform: 'local' } }, isPaused: false, reportPlayback, getCurrentProgress: () => 0 }
  const audioRef = ref(audio)
  let controls
  app = createApp({ setup() { controls = useAudio(audioRef, player, userVolume, offsetDb); return () => h('div') } })
  app.mount(document.createElement('div'))
  return { audio, audioRef, controls, player, reportPlayback }
}

it('applies the current offset after an element switch and before replaying after a reload', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 404 }))
  const offsetDb = ref(-6)
  const { audio, audioRef, controls, player } = engine(ref(0.8), offsetDb)
  player.setPlaybackPosition = vi.fn()
  expect(audio.volume).toBeCloseTo(0.40095, 5)

  const standby = { ...audio, volume: 1, load: vi.fn(), play: vi.fn().mockResolvedValue(undefined) }
  audioRef.value = standby
  await nextTick()
  expect(standby.volume).toBeCloseTo(0.40095, 5)
  offsetDb.value = -12
  await nextTick()
  expect(standby.volume).toBeCloseTo(0.20095, 5)

  controls.handleError()
  standby.load.mockImplementation(() => { standby.volume = 1 })
  await vi.advanceTimersByTimeAsync(1500)
  expect(standby.load).toHaveBeenCalledOnce()
  let volumeAtPlay
  standby.play.mockImplementation(() => {
    volumeAtPlay = standby.volume
    return Promise.resolve()
  })
  controls.checkAutoPlay()
  expect(standby.play).toHaveBeenCalledOnce()
  expect(volumeAtPlay).toBeCloseTo(0.20095, 5)
})

describe('terminal playback errors', () => {
  it('reports retry exhaustion once for the current epoch', () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 404 }))
    const { controls, reportPlayback } = engine()
    for (let i = 0; i < 5; i++) controls.handleError()
    expect(reportPlayback).toHaveBeenCalledExactlyOnceWith('playback.error', { playEpoch: 7, queueId: 'queue-7', code: 'retries-exhausted' })
  })

  it('reports retry exhaustion after a stream claim and deduplicates each code per epoch', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 422, json: async () => ({ code: 'stream-trial-only' }) }))
    const { controls, reportPlayback } = engine()
    controls.handleError()
    await vi.waitFor(() => expect(reportPlayback).toHaveBeenCalledTimes(1))
    controls.handleError()
    controls.handleError()
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(3))
    controls.handleError()
    controls.handleError()
    expect(reportPlayback.mock.calls).toEqual([
      ['playback.error', { playEpoch: 7, queueId: 'queue-7', code: 'stream-trial-only' }],
      ['playback.error', { playEpoch: 7, queueId: 'queue-7', code: 'retries-exhausted' }],
    ])
  })

  it.each(['stream-trial-only', 'stream-unavailable'])('reports a terminal 422 %s without waiting for retries', async code => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ status: 422, json: async () => ({ code }) }))
    const { controls, reportPlayback } = engine()
    controls.handleError()
    await vi.waitFor(() => expect(reportPlayback).toHaveBeenCalledExactlyOnceWith('playback.error', { playEpoch: 7, queueId: 'queue-7', code }))
  })
})
