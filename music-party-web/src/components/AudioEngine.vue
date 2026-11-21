<template>
  <YouTubePlayerAdapter v-if="runtime.nowPlaying?.music.playerType === 'youtube-embed'" />
  <SoundCloudPlayerAdapter v-else-if="runtime.nowPlaying?.music.playerType === 'soundcloud-widget'" />
  <NativeAudioPlayerAdapter v-else />
</template>

<script setup lang="ts">
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore';
import NativeAudioPlayerAdapter from './NativeAudioPlayerAdapter.vue';
import SoundCloudPlayerAdapter from './SoundCloudPlayerAdapter.vue';
import YouTubePlayerAdapter from './YouTubePlayerAdapter.vue';

// Mount only the active engine so inactive timers cannot overwrite its state.
const runtime = useRoomRuntimeStore();
</script>
