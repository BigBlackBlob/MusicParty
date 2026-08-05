import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Music } from '../../contracts/generated/models'
import { musicApi, type LyricDetail } from '../../api/music'

const emptyLyrics = (): Required<LyricDetail> => ({ lyric: '', translatedLyric: '', romanizedLyric: '' })

export const useLyricsStore = defineStore('lyrics', () => {
  const lyricText = ref('')
  const lyricDetail = ref<Required<LyricDetail>>(emptyLyrics())
  const cache = new Map<string, Required<LyricDetail>>()
  let requestId = 0

  async function load(music: Music | null | undefined): Promise<void> {
    const currentRequest = ++requestId
    if (!music?.id || !music.platform) {
      lyricText.value = ''
      lyricDetail.value = emptyLyrics()
      return
    }
    const key = `${music.platform}:${music.id}`
    const cached = cache.get(key)
    if (cached) {
      lyricDetail.value = cached
      lyricText.value = cached.lyric
      return
    }

    lyricText.value = ''
    lyricDetail.value = emptyLyrics()
    let result: Required<LyricDetail>
    try {
      const detail = await musicApi.getLyricDetail(music.platform, music.id)
      result = {
        lyric: detail.lyric ?? '',
        translatedLyric: detail.translatedLyric ?? '',
        romanizedLyric: detail.romanizedLyric ?? ''
      }
    } catch (detailError) {
      console.error('Lyrics Error', detailError)
      try {
        result = { lyric: await musicApi.getLyric(music.platform, music.id), translatedLyric: '', romanizedLyric: '' }
      } catch (fallbackError) {
        console.error('Lyrics Fallback Error', fallbackError)
        return
      }
    }
    if (currentRequest !== requestId) return
    cache.set(key, result)
    if (cache.size > 10) {
      const oldestKey = cache.keys().next().value
      if (oldestKey) cache.delete(oldestKey)
    }
    lyricDetail.value = result
    lyricText.value = result.lyric
  }

  function reset(): void {
    requestId += 1
    lyricText.value = ''
    lyricDetail.value = emptyLyrics()
  }

  return { lyricText, lyricDetail, load, reset }
})
