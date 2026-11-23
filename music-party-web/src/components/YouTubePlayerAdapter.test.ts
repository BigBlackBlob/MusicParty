import { createApp, nextTick, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import YouTubePlayerAdapter from './YouTubePlayerAdapter.vue'
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore'
import { useAudioPlaybackStore } from '../domains/playback/audioPlaybackStore'
import { useToastStore } from '../stores/toast'
import { useUiStore } from '../stores/ui'
import { realtimeClient } from '../transport/realtimeClient'

vi.mock('../domains/playback/lyrics', () => ({useLyricsStore: () => ({load:vi.fn()})}))

let app: App | undefined
let host: HTMLDivElement
let events: { onReady: () => void; onStateChange: (e: {data:number}) => void; onError: (e: {data:number}) => void }
const video = { loadVideoById:vi.fn(), cueVideoById:vi.fn(), playVideo:vi.fn(), pauseVideo:vi.fn(), destroy:vi.fn(), getCurrentTime:()=>0, getDuration:()=>189, seekTo:vi.fn(), setVolume:vi.fn() }

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  setActivePinia(createPinia())
  useUiStore().locale = 'zh'
  useRoomRuntimeStore().nowPlaying = {
    music:{id:'0Ojarj6nTM4',name:'Test',artists:[],coverUrl:'',url:'',needsProxy:false,platform:'youtube',duration:189000,embedId:'0Ojarj6nTM4',playerType:'youtube-embed'},
    queueId:'queue-1',playEpoch:1,currentPosition:0,positionUpdatedAt:0,likedUserIds:[],likeMarkers:[],sessionState:'ready',
  }
  vi.stubGlobal('YT', {PlayerState:{PLAYING:1,BUFFERING:3,PAUSED:2,ENDED:0}, Player:class {
    constructor(_id: string, options: {events:typeof events}) { events = options.events; return video }
  }})
  vi.spyOn(realtimeClient,'send').mockReturnValue(true)
  host = document.createElement('div')
  document.body.append(host)
})

afterEach(() => {
  app?.unmount(); app = undefined
  host.remove()
  localStorage.clear()
  vi.restoreAllMocks(); vi.unstubAllGlobals(); vi.useRealTimers()
})

async function mountPlayer() {
  app = createApp(YouTubePlayerAdapter)
  app.mount(host)
  await nextTick()
  events.onReady()
}

it('sets volume before the video loads and follows slider, mute and offset changes', async () => {
  const ui = useUiStore()
  ui.setVolume(0.8)
  ui.setPlatformVolumeOffset('youtube', -6)
  await mountPlayer()
  expect(video.setVolume).toHaveBeenCalledExactlyOnceWith(40)
  expect(video.setVolume).toHaveBeenCalledBefore(video.loadVideoById)

  ui.setVolume(0)
  await nextTick()
  expect(video.setVolume).toHaveBeenLastCalledWith(0)
  ui.setVolume(0.6)
  await nextTick()
  expect(video.setVolume).toHaveBeenLastCalledWith(30)
  ui.setPlatformVolumeOffset('youtube', -12)
  await nextTick()
  expect(video.setVolume).toHaveBeenLastCalledWith(15)
})

it('ends buffering, shows the embed error once and ignores subsequent buffering events', async () => {
  await mountPlayer()
  events.onStateChange({data:3})
  expect(useAudioPlaybackStore().isBuffering).toBe(true)
  events.onError({data:150})
  events.onStateChange({data:3})
  events.onError({data:150})
  await vi.advanceTimersByTimeAsync(4000)
  expect(useAudioPlaybackStore().isBuffering).toBe(false)
  expect(useAudioPlaybackStore().isErrorState).toBe(true)
  expect(useToastStore().toasts).toHaveLength(1)
  expect(useToastStore().toasts[0]?.message).toContain('嵌入')
  expect(useToastStore().toasts[0]?.message).toContain('150')
  expect(realtimeClient.send).toHaveBeenCalledExactlyOnceWith('playback.error',{playEpoch:1,queueId:'queue-1',code:'stream-unavailable'})
  expect(host.textContent).not.toContain('点击启用声音')
})

it('clears the failed state when the same video starts in a new playback epoch', async () => {
  await mountPlayer()
  events.onError({data:150})
  const runtime = useRoomRuntimeStore()
  runtime.nowPlaying = {...runtime.nowPlaying!, playEpoch:2,queueId:'queue-2'}
  await nextTick()
  expect(useAudioPlaybackStore().isErrorState).toBe(false)
  events.onStateChange({data:1})
  expect(realtimeClient.send).toHaveBeenLastCalledWith('playback.started',expect.objectContaining({playEpoch:2,queueId:'queue-2'}))
})

it('does not reload a video for an ordinary player state snapshot', async () => {
  await mountPlayer()
  video.loadVideoById.mockClear()
  const runtime = useRoomRuntimeStore()
  runtime.nowPlaying = {...runtime.nowPlaying!, currentPosition:1000,sessionState:'playing'}
  await nextTick()
  expect(video.loadVideoById).not.toHaveBeenCalled()
})

it.each([
  [2,'ID 无效'],[5,'点击启用声音'],[100,'私密'],[101,'嵌入'],[153,'Referer'],
])('explains YouTube error %i', async (code, text) => {
  await mountPlayer()
  events.onError({data:code})
  expect(useToastStore().toasts[0]?.message).toContain(text)
  expect(useAudioPlaybackStore().isErrorState).toBe(true)
})

it('reports API loading timeout instead of leaving buffering active', async () => {
  vi.stubGlobal('YT',undefined)
  vi.spyOn(document.head,'appendChild').mockImplementation(node => node)
  useAudioPlaybackStore().isBuffering = true
  app = createApp(YouTubePlayerAdapter)
  app.mount(host)
  await vi.advanceTimersByTimeAsync(15000)
  expect(useAudioPlaybackStore().isBuffering).toBe(false)
  expect(useAudioPlaybackStore().isErrorState).toBe(true)
  expect(useToastStore().toasts[0]?.message).toContain('(API)')
})

it('suggests a domain name when YouTube rejects an IP origin', async () => {
  vi.stubGlobal('location',new URL('http://154.37.221.9:8848'))
  await mountPlayer()
  events.onError({data:150})
  expect(useToastStore().toasts[0]?.message).toContain('请改用域名')
})
