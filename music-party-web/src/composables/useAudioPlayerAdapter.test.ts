import { createPinia, setActivePinia } from 'pinia'
import { expect, it, vi } from 'vitest'
import { useAudioPlayerAdapter } from './useAudioPlayerAdapter'
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore'
import { realtimeClient } from '../transport/realtimeClient'

it('rounds fractional clock offsets before sending a Go integer timestamp', () => {
  setActivePinia(createPinia())
  const runtime = useRoomRuntimeStore()
  runtime.serverClockOffset = 0.5
  const adapter = useAudioPlayerAdapter()
  const send = vi.spyOn(realtimeClient, 'send').mockReturnValue(true)
  const clientTime = adapter.serverNow()
  adapter.reportPlayback('playback.started', { playEpoch: 1, queueId: 'queue', clientTime })
  expect(Number.isInteger(clientTime)).toBe(true)
  expect(send).toHaveBeenCalledWith('playback.started', expect.objectContaining({ clientTime }))
  send.mockRestore()
})
