<template>
  <YouTubePlayerAdapter v-if="useEmbed" />
  <SoundCloudPlayerAdapter v-else-if="runtime.nowPlaying?.music.playerType === 'soundcloud-widget'" />
  <NativeAudioPlayerAdapter v-else @youtube-fallback="useYouTubeFallback" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { shouldUseYouTubeEmbed } from '../utils/audioEngine'
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore'
import NativeAudioPlayerAdapter from './NativeAudioPlayerAdapter.vue'
import SoundCloudPlayerAdapter from './SoundCloudPlayerAdapter.vue'
import YouTubePlayerAdapter from './YouTubePlayerAdapter.vue'

// Mount only the active engine so inactive timers cannot overwrite its state.
const runtime = useRoomRuntimeStore()
const playbackKey = computed(() => {
  const now = runtime.nowPlaying
  return now?.music.platform === 'youtube' ? `${now.queueId}:${now.playEpoch}:${now.music.id}` : ''
})
const fallbackKey = ref('')
const useEmbed = computed(() => shouldUseYouTubeEmbed({
  platform: runtime.nowPlaying?.music.platform,
  playerType: runtime.nowPlaying?.music.playerType,
  playbackKey: playbackKey.value,
  fallbackKey: fallbackKey.value,
}))
watch(playbackKey, () => { fallbackKey.value = '' })
const useYouTubeFallback = () => { fallbackKey.value = playbackKey.value }
</script>
