<template>
  <div class="qp-pane">
    <div class="qp-scroll">
      <div v-if="liked.likedSongs.length === 0" class="qp-empty">
        <span class="qp-empty-icon" aria-hidden="true">
          <Heart :size="22" :stroke-width="1.75" aria-hidden="true" />
        </span>
        <div class="qp-empty-title">{{ t('queue.noLiked') }}</div>
        <div class="qp-empty-desc">{{ t('queue.likedDesc') }}</div>
      </div>

      <div v-else class="qp-track-stack">
        <TrackListItem
          v-for="song in liked.likedSongs"
          :key="song.key"
          :title="song.name"
          :artist="formatArtists(song.artists)"
          :cover-url="song.coverUrl"
        >
          <template #meta>
            {{ song.platform }}
          </template>
          <template #suffix>
            <button
              type="button"
              class="qp-icon-btn qp-icon-btn--sm qp-icon-btn--danger"
              :aria-label="t('queue.remove')"
              :title="t('queue.remove')"
              @click="removeLikedSong(song.key)"
            >
              <Trash2 :size="15" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </template>
        </TrackListItem>
      </div>
    </div>

    <footer class="qp-bottom-bar">
      <span class="qp-bottom-meta">{{ liked.likedSongs.length }} {{ t('queue.tracks') }}</span>
      <button
        type="button"
        class="qp-bar-btn"
        :disabled="liked.likedSongs.length === 0"
        @click="exportLikedSongs"
      >
        {{ t('common.export') }}
      </button>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Heart, Trash2 } from 'lucide-vue-next'
import { useLikedSongsStore } from '../../../domains/playback/likedSongs'
import { createLikedSongsFilename, createLikedSongsText } from '../../../utils/likedSongs'
import TrackListItem from '../../ui/TrackListItem.vue'

const liked = useLikedSongsStore()
const { t } = useI18n()

const formatArtists = (artists?: string[] | null) => (
  Array.isArray(artists) && artists.length ? artists.join(' / ') : t('common.unknownArtist')
)

const removeLikedSong = (key: string) => liked.removeAndSync(key)

const exportLikedSongs = () => {
  if (!liked.likedSongs.length) return
  const blob = new Blob([createLikedSongsText(liked.likedSongs)], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = createLikedSongsFilename()
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}
</script>
