import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { NowPlaying } from '../contracts/generated/models'
import { useAudioPlaybackStore } from '../domains/playback/audioPlaybackStore'
import { useLikedSongsStore } from '../domains/playback/likedSongs'
import { useLyricsStore } from '../domains/playback/lyrics'
import { useRoomCommandStore } from '../domains/realtime/roomCommandStore'
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore'
import { useRoomPresenceStore } from '../domains/realtime/roomPresenceStore'
import { useUiStore } from '../stores/ui'
import { useUserStore } from '../stores/user'

const platformLabelKeyMap: Record<string, string> = {
  netease: 'platforms.netease',
  bilibili: 'platforms.bilibili',
  youtube: 'platforms.youtube',
  navidrome: 'platforms.navidrome',
}

const resolveRequesterName = (nowPlaying: NowPlaying | null, user: ReturnType<typeof useUserStore>, presence: ReturnType<typeof useRoomPresenceStore>): string => {
  if (nowPlaying?.enqueuedByName) return nowPlaying.enqueuedByName
  return nowPlaying?.enqueuedById ? presence.resolveName(nowPlaying.enqueuedById, user.resolveName(nowPlaying.enqueuedById)) : ''
}

export function useNowPlayingViewModel(options: { artistSeparator?: string } = {}) {
  const runtime = useRoomRuntimeStore()
  const audio = useAudioPlaybackStore()
  const commands = useRoomCommandStore()
  const lyrics = useLyricsStore()
  const liked = useLikedSongsStore()
  const ui = useUiStore()
  const user = useUserStore()
  const presence = useRoomPresenceStore()
  const { t } = useI18n()
  const artistSeparator = options.artistSeparator ?? ' / '

  const player = reactive({
    get nowPlaying() { return runtime.nowPlaying },
    get isPaused() { return runtime.isPaused },
    get isShuffle() { return runtime.isShuffle },
    get isPauseLocked() { return runtime.isPauseLocked },
    get isSkipLocked() { return runtime.isSkipLocked },
    get isShuffleLocked() { return runtime.isShuffleLocked },
    get isLoading() { return runtime.isLoading },
    get playbackPositionMs() { return audio.playbackPositionMs },
    get bufferedMs() { return audio.bufferedMs },
    get isBuffering() { return audio.isBuffering },
    get isErrorState() { return audio.isErrorState },
    get lyricText() { return lyrics.lyricText },
    get lyricDetail() { return lyrics.lyricDetail },
    setSeekingPreview: (value: boolean) => { audio.isSeekingPreview = value },
    seek: commands.seek,
    toggleShuffle: commands.toggleShuffle,
    togglePause: commands.togglePause,
    playNext: commands.playNext,
  })

  const nowPlaying = computed(() => runtime.nowPlaying)
  const music = computed(() => nowPlaying.value?.music ?? null)
  const coverUrl = computed(() => music.value?.coverUrl ?? '')
  const trackTitle = computed(() => music.value?.name || t('player.waitingFirstTrack'))
  const artistLine = computed(() => music.value?.artists.length ? music.value.artists.join(artistSeparator) : t('app.brand'))
  const platformLabel = computed(() => {
    const key = music.value?.platform ? platformLabelKeyMap[music.value.platform] : undefined
    return key ? t(key) : t('platforms.room')
  })
  const requesterName = computed(() => resolveRequesterName(nowPlaying.value, user, presence))
  const durationMs = computed(() => music.value?.duration ?? 0)
  const progressMs = computed(() => audio.playbackPositionMs)
  const progressPercent = computed(() => durationMs.value
    ? `${Math.max(0, Math.min(100, progressMs.value / durationMs.value * 100))}%`
    : '0%')
  const bufferedPercent = computed(() => durationMs.value
    ? `${Math.max(0, Math.min(100, audio.bufferedMs / durationMs.value * 100))}%`
    : '0%')
  const isRequester = computed(() => nowPlaying.value?.enqueuedById === user.publicId)
  const canSeek = computed(() => Boolean(
    music.value
    && durationMs.value > 0
    && (!nowPlaying.value?.enqueuedById || isRequester.value || user.capabilities.canManageCurrentRoom),
  ))
  const isLiked = computed(() => liked.includes(music.value))
  const activeUserCount = computed(() => presence.count)
  const ambientAccent = computed(() => (ui.dynamicAccent as { accent?: string } | null)?.accent || '#ede1ff')

  function seekToRatio(ratio: number): void {
    if (!canSeek.value) return
    commands.seek(Math.floor(Math.max(0, Math.min(1, ratio)) * durationMs.value))
  }

  const toggleLike = (): Promise<void> => commands.toggleLike(music.value)

  watch(coverUrl, nextCoverUrl => ui.updateAccentFromCover(nextCoverUrl), { immediate: true })

  return {
    player, ui, user, nowPlaying, music, coverUrl, trackTitle, artistLine, platformLabel,
    requesterName, durationMs, progressMs, progressPercent, bufferedPercent, canSeek,
    isLiked, activeUserCount, ambientAccent, seekToRatio, toggleLike,
  }
}
