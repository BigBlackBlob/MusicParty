import { createApp, nextTick, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import SoundCloudPlayerAdapter from './SoundCloudPlayerAdapter.vue'
import AudioEngine from './AudioEngine.vue'
import { useAudioPlaybackStore } from '../domains/playback/audioPlaybackStore'
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore'
import { realtimeClient } from '../transport/realtimeClient'

vi.mock('../domains/playback/lyrics', () => ({useLyricsStore: () => ({load:vi.fn()})}))

let app: App
let host: HTMLDivElement
let handlers: Record<string, () => void>
let completeDuration: (durationMs: number) => void
const widget = {
  bind:(event:string, handler:()=>void) => {handlers[event] = handler},
  unbind:vi.fn(),load:vi.fn(),play:vi.fn(),pause:vi.fn(),seekTo:vi.fn(),
  getPosition:(callback:(position:number)=>void) => callback(1200.6),
  getDuration:(callback:(duration:number)=>void) => {completeDuration = callback},
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  setActivePinia(createPinia())
  handlers = {}
  useRoomRuntimeStore().nowPlaying = {
    music:{id:'54202938',name:'Test',artists:[],coverUrl:'',url:'',needsProxy:false,platform:'soundcloud',duration:0,embedId:'54202938',playerType:'soundcloud-widget'},
    queueId:'queue-1',playEpoch:1,currentPosition:0,positionUpdatedAt:0,likedUserIds:[],likeMarkers:[],sessionState:'ready',
  }
  const Widget = Object.assign(() => widget,{Events:{READY:'READY',PLAY:'PLAY',PAUSE:'PAUSE',FINISH:'FINISH',ERROR:'ERROR'}})
  vi.stubGlobal('SC',{Widget})
  vi.spyOn(realtimeClient,'send').mockReturnValue(true)
  host = document.createElement('div')
})

afterEach(() => {
  app?.unmount(); host.remove()
  vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers()
})

async function startWidget() {
  app = createApp(SoundCloudPlayerAdapter)
  app.mount(host)
  await nextTick()
  handlers.READY?.()
  handlers.PLAY?.()
}

it('reports the Widget duration when public metadata has no duration', async () => {
  await startWidget()
  completeDuration(377425.4)
  expect(realtimeClient.send).toHaveBeenCalledExactlyOnceWith('playback.started',expect.objectContaining({
    playEpoch:1,queueId:'queue-1',positionMs:1201,durationMs:377425,
  }))
})

it('ignores a duration callback from an earlier playback epoch', async () => {
  await startWidget()
  const runtime = useRoomRuntimeStore()
  runtime.nowPlaying = {...runtime.nowPlaying!, playEpoch:2,queueId:'queue-2'}
  completeDuration(377425)
  expect(realtimeClient.send).not.toHaveBeenCalled()
})

it('keeps Widget progress between polls while a room start is pending', async () => {
  app = createApp(AudioEngine)
  app.mount(host)
  await nextTick()
  handlers.READY?.()
  handlers.PLAY?.()
  completeDuration(377425)
  await vi.advanceTimersByTimeAsync(250)
  expect(useAudioPlaybackStore().playbackPositionMs).toBe(1200.6)
  await vi.advanceTimersByTimeAsync(150)
  expect(useAudioPlaybackStore().playbackPositionMs).toBe(1200.6)
})

it('restarts the same SoundCloud track for a new queue entry and keeps snapshot updates playing', async () => {
  await startWidget()
  const runtime = useRoomRuntimeStore()
  runtime.nowPlaying = { ...runtime.nowPlaying!, currentPosition: 1201, sessionState: 'playing' }
  await nextTick()
  expect(widget.load).not.toHaveBeenCalled()
  runtime.nowPlaying = { ...runtime.nowPlaying!, playEpoch: 2, queueId: 'queue-2', sessionState: 'ready' }
  await nextTick()
  expect(widget.load).toHaveBeenCalledExactlyOnceWith('https://api.soundcloud.com/tracks/54202938', expect.objectContaining({ auto_play: true }))
})

it('stops the native room-clock writer when switching to SoundCloud', async () => {
  const runtime = useRoomRuntimeStore()
  const soundcloud = runtime.nowPlaying!
  runtime.nowPlaying = { ...soundcloud, music: { ...soundcloud.music, platform: 'local', playerType: 'audio' } }
  runtime.remotePosition = 3000
  app = createApp(AudioEngine)
  app.mount(host)
  await vi.advanceTimersByTimeAsync(200)
  expect(useAudioPlaybackStore().playbackPositionMs).toBe(3000)
  runtime.nowPlaying = soundcloud
  await nextTick()
  handlers.READY?.()
  handlers.PLAY?.()
  completeDuration(377425)
  await vi.advanceTimersByTimeAsync(400)
  expect(useAudioPlaybackStore().playbackPositionMs).toBe(1200.6)
})

it('updates the paused progress display when seeking to the room position', async () => {
  await startWidget()
  const runtime = useRoomRuntimeStore()
  const audio = useAudioPlaybackStore()
  runtime.isPaused = true
  await nextTick()
  runtime.remotePosition = 45000
  audio.forceNextSyncSeek = true
  await nextTick()
  expect(widget.seekTo).toHaveBeenLastCalledWith(45000)
  expect(audio.playbackPositionMs).toBe(45000)
  await vi.advanceTimersByTimeAsync(500)
  expect(audio.playbackPositionMs).toBe(45000)
})
