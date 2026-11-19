<template>
  <aside class="fixed bottom-24 left-1/2 z-[var(--z-toast)] flex max-w-[90vw] -translate-x-1/2 flex-wrap items-center gap-x-3 gap-y-1 border border-white/30 bg-black px-3 py-2 text-sm text-white" aria-label="Spotify audio">
    <a :href="trackLink" target="_blank" rel="noopener noreferrer" class="underline">{{ zh ? '在 Spotify 打开' : 'Open in Spotify' }}</a>
    <p v-if="message" role="status">{{ message }}</p>
    <button v-if="canConnect" type="button" class="underline" :disabled="!ui.spotifyClientId" @click="connectSpotify">
      {{ reconnect ? (zh ? '重新连接 Spotify' : 'Reconnect Spotify') : (zh ? '连接 Spotify' : 'Connect Spotify') }}
    </button>
    <button v-else-if="deviceId && !activated && !localError" type="button" class="underline" @click="enableAudio">
      {{ zh ? '启用 Spotify 音频' : 'Enable Spotify audio' }}
    </button>
    <button v-else-if="localError && authStatus === 'connected'" type="button" class="underline" :disabled="rateLimited" @click="restart">
      {{ zh ? '重试' : 'Retry' }}
    </button>
    <button v-if="authStatus === 'connected'" type="button" class="underline" @click="disconnect">
      {{ zh ? '退出 Spotify 音频' : 'Leave Spotify audio' }}
    </button>
  </aside>
</template>

<script lang="ts">
type SDKState = { position: number; paused: boolean; track_window: { current_track: { uri: string } } }
type SDKEvents = {
  ready: { device_id: string }
  not_ready: { device_id: string }
  player_state_changed: SDKState | null
  initialization_error: { message: string }
  authentication_error: { message: string }
  account_error: { message: string }
  playback_error: { message: string }
  autoplay_failed: undefined
}
type SDKPlayer = {
  addListener<K extends keyof SDKEvents>(_event: K, _handler: (_data: SDKEvents[K]) => void): boolean
  removeListener(_event: keyof SDKEvents): boolean
  connect(): Promise<boolean>
  disconnect(): void
  activateElement(): Promise<void>
  getCurrentState(): Promise<SDKState | null>
  pause(): Promise<void>
  resume(): Promise<void>
  seek(_position: number): Promise<void>
}
type SDK = { Player: new (_options: { name: string; getOAuthToken: (_callback: (_token: string) => void) => void; volume: number }) => SDKPlayer }
const sdkWindow = () => window as Window & { Spotify?: SDK; onSpotifyWebPlaybackSDKReady?: () => void }
let sdkPromise: Promise<SDK> | undefined
let deviceCommands: Promise<void> = Promise.resolve()

function loadSDK(): Promise<SDK> {
  const host = sdkWindow()
  if (host.Spotify?.Player) return Promise.resolve(host.Spotify)
  sdkPromise ??= new Promise<SDK>((resolve, reject) => {
    const script = document.createElement('script')
    const previousCallback = host.onSpotifyWebPlaybackSDKReady
    const timer = window.setTimeout(onFailure, 15_000)
    function cleanup() {
      clearTimeout(timer)
      if (host.onSpotifyWebPlaybackSDKReady === onReady) {
        if (previousCallback) host.onSpotifyWebPlaybackSDKReady = previousCallback
        else delete host.onSpotifyWebPlaybackSDKReady
      }
      script.remove()
    }
    function onFailure() {
      cleanup()
      reject(new Error('Spotify SDK unavailable'))
    }
    function onReady() {
      if (!host.Spotify?.Player) return onFailure()
      cleanup()
      resolve(host.Spotify)
      previousCallback?.()
    }
    host.onSpotifyWebPlaybackSDKReady = onReady
    script.src = 'https://sdk.scdn.co/spotify-player.js'
    script.async = true
    script.onerror = onFailure
    document.head.append(script)
  }).catch((error: unknown) => {
    sdkPromise = undefined
    throw error
  })
  return sdkPromise
}

function serializeDeviceCommand(command: () => Promise<void>) {
  deviceCommands = deviceCommands.then(command).catch(() => {})
}

const events: (keyof SDKEvents)[] = [
  'ready', 'not_ready', 'player_state_changed', 'initialization_error', 'authentication_error',
  'account_error', 'playback_error', 'autoplay_failed',
]
</script>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { spotifyCallbackPath, useSpotifyAuth } from '../composables/useSpotifyAuth'
import { useUiStore } from '../stores/ui'

const props = defineProps<{
  trackUri: string
  currentPosition: number
  isPaused: boolean
  playEpoch: number
  queueId: string
  duration: number
  getCurrentPosition: () => number
}>()

const ui = useUiStore()
const zh = computed(() => ui.locale === 'zh')
const { status: authStatus, error: authError, grantedClientId, connect, getToken, disconnect, fail } = useSpotifyAuth()
const trackKey = computed(() => `${props.queueId}:${props.trackUri}`)
const trackID = computed(() => /^spotify:track:([A-Za-z0-9]{22})$/.exec(props.trackUri)?.[1])
const trackLink = computed(() => trackID.value ? `https://open.spotify.com/track/${trackID.value}` : 'https://open.spotify.com/')
const deviceId = ref('')
const activated = ref(false)
const rateLimited = ref(false)
type LocalError = 'spotify-sdk-unavailable' | 'spotify-playback-unavailable' | 'spotify-access-denied'
const localError = ref<LocalError | null>(null)
const canConnect = computed(() => authStatus.value === 'disconnected' || authStatus.value === 'error')
const reconnect = computed(() => authStatus.value === 'error')
const message = computed(() => {
  if (!ui.spotifyClientId) return zh.value ? 'Spotify 未配置。' : 'Spotify is not configured.'
  if (authStatus.value === 'connecting') return zh.value ? '正在连接 Spotify。' : 'Connecting to Spotify.'
  if (rateLimited.value) return zh.value ? 'Spotify 请求过多，请稍后再试。' : 'Spotify is rate limiting playback. Try again shortly.'
  switch (authError.value ?? localError.value) {
    case 'spotify-auth-required': return zh.value ? '请重新授权 Spotify。' : 'Spotify authorization is required.'
    case 'spotify-premium-required': return zh.value ? '此账号需要 Spotify Premium。' : 'This account needs Spotify Premium.'
    case 'spotify-access-denied': return zh.value ? 'Spotify 拒绝了播放请求。' : 'Spotify denied playback. Check app access.'
    case 'spotify-sdk-unavailable': return zh.value ? 'Spotify 播放器不可用。' : 'Spotify player is unavailable.'
    case 'spotify-playback-unavailable': return zh.value ? '此设备上的 Spotify 播放已中断。' : 'Spotify playback was interrupted on this device.'
    default: return ''
  }
})

let player: SDKPlayer | null = null
let generation = 0
let activationVersion = 0
let loadedTrack = ''
let confirmedTrack = false
let seekEpoch = 0
let lastCorrection = -Infinity
let pending = false
let requested = false
let retryTimer: number | undefined

function targetPosition() {
  const current = props.getCurrentPosition()
  return Math.round(Math.min(props.duration, Math.max(0, Number.isFinite(current) ? current : props.currentPosition)))
}

function connectSpotify() {
  if (ui.spotifyClientId) void connect(ui.spotifyClientId, new URL(spotifyCallbackPath, location.origin).href)
}

function teardown() {
  generation++
  deviceId.value = ''
  activated.value = false
  loadedTrack = ''
  confirmedTrack = false
  lastCorrection = -Infinity
  clearTimeout(retryTimer)
  rateLimited.value = false
  const previous = player
  player = null
  if (!previous) return
  for (const event of events) previous.removeListener(event)
  serializeDeviceCommand(async () => {
    try { await previous.pause() } finally { previous.disconnect() }
  })
}

async function setup() {
  const instance = ++generation
  let sdkToken: string | undefined
  let recoveringAuth = false
  let retriedAuth = false
  try {
    const SDK = await loadSDK()
    if (instance !== generation) return
    const next = new SDK.Player({
      name: 'MusicParty', volume: 0.8,
      getOAuthToken: callback => {
        void getToken().then(token => {
          if (generation !== instance) return
          sdkToken = token
          callback(token)
        }).catch(() => { if (generation === instance) fail('spotify-auth-required') })
      },
    })
    player = next
    next.addListener('ready', ({ device_id }) => {
      if (generation !== instance) return
      deviceId.value = device_id
      recoveringAuth = false
      retriedAuth = false
      if (localError.value === 'spotify-sdk-unavailable') localError.value = null
      reconcile()
    })
    next.addListener('not_ready', ({ device_id }) => {
      if (deviceId.value !== device_id) return
      deviceId.value = ''
      activated.value = false
      loadedTrack = ''
      confirmedTrack = false
      localError.value = 'spotify-sdk-unavailable'
    })
    next.addListener('player_state_changed', state => {
      if (generation !== instance || !loadedTrack || loadedTrack !== trackKey.value) return
      if (state?.track_window.current_track.uri === props.trackUri) {
        reconcile()
      } else if (confirmedTrack && !pending) {
        if (targetPosition() < props.duration - 1500) localError.value = 'spotify-playback-unavailable'
        // The SDK can move on before the room clock. Stop playback on this device only.
        if (state && !state.paused) {
          serializeDeviceCommand(async () => { if (player === next) await next.pause() })
        }
      }
    })
    next.addListener('initialization_error', () => { localError.value = 'spotify-sdk-unavailable' })
    next.addListener('account_error', () => fail('spotify-premium-required'))
    next.addListener('authentication_error', () => {
      if (recoveringAuth) return
      if (retriedAuth) { fail('spotify-auth-required'); return }
      recoveringAuth = true
      void getToken(sdkToken).then(token => {
        if (generation !== instance) return
        if (token === sdkToken) { fail('spotify-auth-required'); return }
        // Reconnect once using the refreshed grant. A ready event resets the recovery budget.
        recoveringAuth = false
        retriedAuth = true
        deviceId.value = ''
        activated.value = false
        loadedTrack = ''
        confirmedTrack = false
        next.disconnect()
        return next.connect().then(connected => {
          if (!connected && generation === instance) fail('spotify-auth-required')
        })
      }).catch(() => { if (generation === instance) fail('spotify-auth-required') })
    })
    next.addListener('playback_error', () => { localError.value = 'spotify-playback-unavailable' })
    next.addListener('autoplay_failed', () => {
      activationVersion++
      activated.value = false
      loadedTrack = ''
      confirmedTrack = false
    })
    const connected = await next.connect()
    if (!connected && generation === instance && !recoveringAuth && !retriedAuth) localError.value = 'spotify-sdk-unavailable'
  } catch {
    if (generation === instance) localError.value = 'spotify-sdk-unavailable'
  }
}

function restart() {
  teardown()
  localError.value = null
  void setup()
}

async function enableAudio() {
  const current = player
  if (!current) return
  try {
    await current.activateElement()
    if (current !== player) return
    activated.value = true
    reconcile()
  } catch {
    if (current === player) localError.value = 'spotify-sdk-unavailable'
  }
}

const playEndpoint = 'https://api.spotify.com/v1/me/player/play'

async function startTrack(isCurrent: () => boolean) {
  const uri = props.trackUri
  const device = deviceId.value
  const start = async (token: string) => {
    if (!isCurrent() || props.isPaused) return null
    return fetch(`${playEndpoint}?${new URLSearchParams({ device_id: device })}`, {
      method: 'PUT', redirect: 'error', credentials: 'omit', referrerPolicy: 'no-referrer', signal: AbortSignal.timeout(10_000),
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ uris: [uri], position_ms: targetPosition() }),
    })
  }
  try {
    const token = await getToken()
    let response = await start(token)
    if (response?.status === 401 && isCurrent()) response = await start(await getToken(token))
    if (response?.ok) return true
    if (!response || !isCurrent()) return false
    if (response.status === 401) fail('spotify-auth-required')
    else if (response.status === 403) localError.value = 'spotify-access-denied'
    else {
      localError.value = 'spotify-playback-unavailable'
      if (response.status === 429) {
        const seconds = Number(response.headers.get('Retry-After'))
        if (seconds > 0 && Number.isFinite(seconds)) {
          rateLimited.value = true
          retryTimer = window.setTimeout(() => { rateLimited.value = false }, Math.min(seconds * 1000, 2_147_483_647))
        }
      }
    }
  } catch {
    if (isCurrent()) localError.value = 'spotify-playback-unavailable'
  }
  return false
}

async function applyRoomState() {
  const current = player
  const device = deviceId.value
  const key = trackKey.value
  const epoch = props.playEpoch
  const isCurrent = () => player === current && deviceId.value === device && trackKey.value === key && props.playEpoch === epoch && activated.value && !localError.value
  if (!current || !device || !trackID.value || !isCurrent()) return
  if (loadedTrack !== key) {
    if (loadedTrack) {
      await current.pause()
      if (player !== current || deviceId.value !== device) return
      loadedTrack = ''
      confirmedTrack = false
    }
    // Spotify has no silent preload. A paused room starts the new item on resume.
    if (props.isPaused || targetPosition() >= props.duration || !isCurrent()) return
    const activation = activationVersion
    if (!await startTrack(isCurrent)) return
    // Track or transport state may have changed while Spotify processed the PUT.
    if (player !== current || deviceId.value !== device || activationVersion !== activation) return
    loadedTrack = key
    confirmedTrack = false
    seekEpoch = epoch
    if (!isCurrent()) return
  }
  const state = await current.getCurrentState()
  if (!isCurrent() || state?.track_window.current_track.uri !== props.trackUri) return
  confirmedTrack = true
  let paused = state.paused
  if (props.isPaused && !paused) {
    await current.pause()
    paused = true
  }
  if (!isCurrent()) return
  const target = targetPosition()
  if (target >= props.duration) {
    if (!paused) await current.pause()
    return
  }
  const now = performance.now()
  if (seekEpoch !== epoch || (Math.abs(state.position - target) > 1500 && now - lastCorrection >= 2000)) {
    seekEpoch = epoch
    lastCorrection = now
    await current.seek(target)
  }
  if (isCurrent() && paused && !props.isPaused) await current.resume()
}

function reconcile() {
  requested = true
  if (pending) return
  pending = true
  serializeDeviceCommand(async () => {
    try {
      while (requested) {
        requested = false
        const instance = generation
        const epoch = props.playEpoch
        const key = trackKey.value
        try { await applyRoomState() }
        catch {
          if (instance === generation && epoch === props.playEpoch && key === trackKey.value) {
            localError.value = 'spotify-playback-unavailable'
          }
        }
      }
    } finally {
      pending = false
    }
  })
}

watch([authStatus, () => ui.spotifyClientId], ([status, clientId]) => {
  if (status !== 'connected' || !clientId) teardown()
  else if (clientId !== grantedClientId.value) fail('spotify-auth-required')
  else restart()
}, { immediate: true })
// A room seek changes the epoch without replacing the queued track.
watch([trackKey, () => props.playEpoch, () => props.isPaused, () => props.currentPosition], reconcile)
onMounted(() => document.addEventListener('visibilitychange', reconcile))
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', reconcile)
  teardown()
})
</script>
