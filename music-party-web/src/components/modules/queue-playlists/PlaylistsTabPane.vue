<template>
<div class="qp-pane">
  <Transition :name="playlistNavTransition" mode="out-in">
  <!-- Index View -->
  <div v-if="viewMode === 'index'" key="pl-index" class="qp-pane">
    <div class="qp-scope-bar">
      <div class="qp-capsule-group" role="tablist" :aria-label="t('roomPlaylists.title')">
        <button
          v-for="option in scopes"
          :key="option"
          type="button"
          role="tab"
          class="qp-capsule-btn"
          :class="{ 'is-active': scope === option }"
          :aria-selected="scope === option"
          :disabled="busy"
          @click="switchScope(option)"
        >
          {{ t(option === 'room' ? 'roomPlaylists.roomScope' : 'roomPlaylists.personalScope') }}
        </button>
      </div>

      <button
        v-if="!(scope === 'personal' && user.isGuest)"
        type="button"
        class="qp-pill-btn qp-pill-btn--accent"
        :disabled="busy"
        @click="openCreatePlaylist"
      >
        <Plus :size="15" :stroke-width="1.75" aria-hidden="true" />
        <span>{{ t('rooms.create') }}</span>
      </button>
    </div>

    <Transition name="qp-slide">
      <form
        v-if="showCreateForm && !(scope === 'personal' && user.isGuest)"
        class="qp-create-row"
        @submit.prevent="create"
      >
        <input
          ref="createInput"
          v-model.trim="newName"
          maxlength="64"
          class="qp-input"
          :aria-label="t('roomPlaylists.newPlaceholder')"
          :placeholder="t('roomPlaylists.newPlaceholder')"
          :disabled="busy"
          @keydown.esc.prevent="closeCreatePlaylist"
        />
        <button type="submit" class="qp-primary-btn qp-primary-btn--sm" :disabled="busy || !newName">
          {{ t('rooms.create') }}
        </button>
      </form>
    </Transition>

    <div
      :ref="viewMode === 'index' ? setSortList : undefined"
      class="qp-scroll qp-pl-list"
      :aria-busy="playlistStore.loading"
    >
      <!-- Pinned Liked Songs entry inside Personal scope -->
      <div v-if="showVirtualLikedEntry" class="qp-pl-row-wrap" data-no-drag>
        <button
          type="button"
          class="qp-pl-row"
          :class="{ 'is-selected': viewingVirtualLiked }"
          :disabled="busy"
          @click="openVirtualLiked"
        >
          <span class="qp-cover qp-cover--40 qp-cover--liked" aria-hidden="true">
            <Heart :size="18" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
          </span>

          <span class="qp-pl-meta">
            <span class="qp-pl-name">{{ t('queue.liked') }}</span>
            <span class="qp-pl-sub">
              {{ liked.likedSongs.length }} {{ t('queue.tracks') }}
            </span>
          </span>

          <ChevronRight class="qp-chevron" :size="18" :stroke-width="1.75" aria-hidden="true" />
        </button>
      </div>

      <template v-if="scope === 'personal' && user.isGuest">
        <div class="qp-guest-note">
          <Lock :size="15" :stroke-width="1.75" aria-hidden="true" />
          <span>{{ t('roomPlaylists.personalUnavailable') }}</span>
        </div>
      </template>

      <template v-else>
      <div
        v-for="playlist in playlistStore.playlists"
        :key="playlist.id"
        class="qp-pl-row-wrap"
        :class="{ 'is-draggable': canSort && !playlist.systemKey }"
        :data-reorder-id="canSort && !playlist.systemKey ? playlist.id : undefined"
        @keydown.up.prevent="canSort && !playlist.systemKey && moveBy(playlist.id, -1)"
        @keydown.down.prevent="canSort && !playlist.systemKey && moveBy(playlist.id, 1)"
      >
        <button
          type="button"
          class="qp-pl-row"
          :class="{ 'is-selected': !viewingVirtualLiked && playlistStore.selectedPlaylistId === playlist.id }"
          :disabled="busy"
          @click="select(playlist.id)"
        >
          <CoverImage
            v-if="playlist.coverUrl"
            :src="playlist.coverUrl"
            alt=""
            class="qp-cover qp-cover--40"
          />
          <span
            v-else
            class="qp-cover qp-cover--40"
            :class="playlist.systemKey === 'liked-songs' ? 'qp-cover--liked' : 'qp-cover--placeholder'"
            aria-hidden="true"
          >
            <Heart v-if="playlist.systemKey === 'liked-songs'" :size="18" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
            <Disc3 v-else :size="18" :stroke-width="1.75" aria-hidden="true" />
          </span>

          <span class="qp-pl-meta">
            <span class="qp-pl-name">{{ playlist.name }}</span>
            <span class="qp-pl-sub">
              <template v-if="playlist.systemKey">
                <span class="qp-sys-tag">{{ t('roomPlaylists.systemTag') }}</span> &middot;
              </template>
              {{ playlist.trackCount }} {{ t('queue.tracks') }}
            </span>
          </span>

          <ChevronRight class="qp-chevron" :size="18" :stroke-width="1.75" aria-hidden="true" />
        </button>
      </div>

      <div
        v-if="playlistStore.loading && !playlistStore.playlists.length"
        class="qp-skeleton-list"
        aria-busy="true"
        :aria-label="t('roomPlaylists.loading')"
      >
        <div v-for="i in 4" :key="i" class="qp-skeleton-row qp-skeleton-row--pl">
          <div class="qp-skeleton-box qp-cover--40 animate-shimmer"></div>
          <div class="qp-skeleton-lines">
            <div class="qp-skeleton-bar animate-shimmer" :style="{ width: `${52 + (i * 11) % 30}%` }"></div>
            <div class="qp-skeleton-bar qp-skeleton-bar--sub animate-shimmer" :style="{ width: `${28 + (i * 7) % 18}%` }"></div>
          </div>
        </div>
      </div>

      <div v-else-if="!playlistStore.loading && !playlistStore.playlists.length && !showVirtualLikedEntry" class="qp-empty">
        <span class="qp-empty-icon" aria-hidden="true">
          <ListMusic :size="22" :stroke-width="1.75" aria-hidden="true" />
        </span>
        <div class="qp-empty-desc">{{ t('roomPlaylists.empty') }}</div>
      </div>
      </template>
    </div>
  </div>

  <!-- Virtual Liked Songs Detail View (inside Personal) -->
  <div v-else-if="viewingVirtualLiked" key="pl-liked-detail" class="qp-pane">
    <header class="qp-detail-header">
      <button
        type="button"
        class="qp-icon-btn"
        :aria-label="t('common.back')"
        :title="t('common.back')"
        @click="goBack"
      >
        <ChevronLeft :size="17" :stroke-width="1.75" aria-hidden="true" />
      </button>

      <div class="qp-detail-cover-wrap">
        <span class="qp-cover qp-cover--44 qp-cover--liked" aria-hidden="true">
          <Heart :size="18" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
        </span>
      </div>

      <div class="qp-detail-title-wrap">
        <span class="qp-detail-name">{{ t('queue.liked') }}</span>
      </div>

      <div class="qp-detail-actions">
        <button
          v-if="liked.likedSongs.length && hasRoom"
          type="button"
          class="qp-primary-btn qp-primary-btn--sm"
          :aria-label="t('roomPlaylists.playAll')"
          :title="t('roomPlaylists.playAll')"
          :disabled="busy || !commands.canOperate"
          @click="queueAllLiked"
        >
          <Play :size="15" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
          <span class="qp-play-all-label">{{ t('roomPlaylists.playAll') }}</span>
        </button>
      </div>
    </header>

    <div ref="likedListRef" class="qp-scroll qp-detail-tracks">
      <div v-if="liked.likedSongs.length === 0" class="qp-empty">
        <span class="qp-empty-icon" aria-hidden="true">
          <Heart :size="22" :stroke-width="1.75" aria-hidden="true" />
        </span>
        <div class="qp-empty-title">{{ t('queue.noLiked') }}</div>
        <div class="qp-empty-desc">{{ t('queue.likedDesc') }}</div>
      </div>

      <div
        v-for="(song, index) in liked.likedSongs"
        v-else
        :key="song.key"
        class="qp-track-entry"
        :class="{ 'is-editing': editingTrackId === song.key }"
      >
        <div
          class="qp-pl-track-row is-clickable"
          @click="onLikedSongRowClick(song)"
        >
          <span class="qp-track-index">{{ index + 1 }}</span>

          <CoverImage
            v-if="song.coverUrl"
            :src="song.coverUrl"
            alt=""
            class="qp-cover qp-cover--36"
          />
          <span v-else class="qp-cover qp-cover--36 qp-cover--placeholder" aria-hidden="true">
            <Music :size="16" :stroke-width="1.75" aria-hidden="true" />
          </span>

          <div class="qp-track-meta">
            <p class="qp-track-title">{{ song.name }}</p>
            <p class="qp-track-artist">{{ formatArtists(song.artists) }}</p>
          </div>

          <div class="qp-track-actions" data-no-drag @click.stop>
            <button
              v-if="hasRoom"
              type="button"
              class="qp-icon-btn qp-icon-btn--sm qp-icon-btn--accent"
              :aria-label="t('roomPlaylists.addToQueue')"
              :title="t('roomPlaylists.addToQueue')"
              :disabled="busy || !commands.canOperate"
              @click="commands.enqueue(song.platform, song.id)"
            >
              <Plus :size="15" :stroke-width="1.75" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="qp-icon-btn qp-icon-btn--sm qp-icon-btn--danger"
              :aria-label="t('queue.remove')"
              :title="t('queue.remove')"
              @click="liked.removeAndSync(song.key)"
            >
              <HeartMinus :size="15" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </div>
        </div>

        <!-- Commentary Display -->
        <p
          v-if="song.notes && editingTrackId !== song.key"
          class="qp-commentary-display is-clickable"
          @click="editLikedNotes(song)"
        >
          {{ song.notes }}
        </p>

        <!-- Inline Commentary Textarea -->
        <Transition name="qp-slide">
          <div v-if="editingTrackId === song.key" class="qp-commentary-wrap">
            <textarea
              v-model="trackNotes"
              rows="2"
              maxlength="1000"
              data-playlist-note
              class="qp-commentary-textarea"
              :aria-label="t('roomPlaylists.editNotes')"
              :placeholder="t('roomPlaylists.editNotes')"
              @keydown.enter.exact.prevent="saveLikedNotes(song)"
              @keydown.esc.prevent="cancelNotes"
              @blur="cancelNotes"
            ></textarea>
          </div>
        </Transition>
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

  <!-- Detail View -->
  <div v-else-if="playlistStore.selectedPlaylist" key="pl-detail" class="qp-pane">
    <!-- Compact 44px Detail Header -->
    <header class="qp-detail-header">
      <button
        type="button"
        class="qp-icon-btn"
        :aria-label="t('common.back')"
        :title="t('common.back')"
        @click="goBack"
      >
        <ChevronLeft :size="17" :stroke-width="1.75" aria-hidden="true" />
      </button>

      <div class="qp-detail-cover-wrap">
        <CoverImage
          v-if="coverUrl"
          :src="coverUrl"
          alt=""
          class="qp-cover qp-cover--44"
        />
        <span v-else class="qp-cover qp-cover--44 qp-cover--placeholder" aria-hidden="true">
          <Disc3 :size="18" :stroke-width="1.75" aria-hidden="true" />
        </span>
        <button
          v-if="canEdit"
          type="button"
          class="qp-cover-cam"
          :aria-label="t('roomPlaylists.uploadCover')"
          :title="t('roomPlaylists.uploadCover')"
          :disabled="busy"
          @click="coverInput?.click()"
        >
          <Camera :size="13" :stroke-width="1.75" aria-hidden="true" />
        </button>
        <input
          ref="coverInput"
          type="file"
          accept="image/png,image/jpeg"
          hidden
          @change="uploadCover"
        />
      </div>

      <div class="qp-detail-title-wrap">
        <input
          v-if="editor === 'rename' && canEdit"
          ref="renameInput"
          v-model.trim="renameValue"
          class="qp-inline-title-input"
          maxlength="64"
          :aria-label="t('roomPlaylists.namePrompt')"
          :disabled="busy"
          @keydown.enter.prevent="commitRename"
          @keydown.esc.prevent="cancelRename"
          @blur="commitRename"
        />
        <button
          v-else-if="canEdit"
          type="button"
          class="qp-detail-name-btn"
          :title="t('roomPlaylists.rename')"
          @click="openRename"
        >
          <span class="qp-detail-name">{{ playlistStore.selectedPlaylist.name || t('roomPlaylists.loading') }}</span>
        </button>
        <span v-else class="qp-detail-name">{{ playlistStore.selectedPlaylist.name || t('roomPlaylists.loading') }}</span>
      </div>

      <div class="qp-detail-actions">
        <button
          v-if="canEdit"
          type="button"
          class="qp-icon-btn"
          :class="{ 'is-accent': showImportBar }"
          :aria-label="t('search.importPlaylist')"
          :title="t('search.importPlaylist')"
          :aria-expanded="showImportBar"
          :disabled="busy"
          @click="toggleImportBar"
        >
          <Download :size="17" :stroke-width="1.75" aria-hidden="true" />
        </button>

        <button
          v-if="playlistStore.selectedTracks.length && hasRoom"
          type="button"
          class="qp-primary-btn qp-primary-btn--sm"
          :aria-label="t('roomPlaylists.playAll')"
          :title="t('roomPlaylists.playAll')"
          :disabled="busy || !commands.canOperate"
          @click="playSelected"
        >
          <Play :size="15" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
          <span class="qp-play-all-label">{{ t('roomPlaylists.playAll') }}</span>
        </button>
      </div>
    </header>

    <!-- Collapsible Single-Line Import Bar -->
    <Transition name="qp-slide">
      <form v-if="canEdit && showImportBar" class="qp-import-bar" @submit.prevent="importExternal">
        <select
          v-model="importPlatform"
          class="qp-select qp-import-select"
          :aria-label="t('roomPlaylists.importPlatform')"
          :disabled="busy || !importPlatforms.length"
        >
          <option v-if="!importPlatforms.length" value="">
            {{ t(platformsQuery.isPending.value ? 'roomPlaylists.loadingProviders' : 'roomPlaylists.noProviders') }}
          </option>
          <option v-for="platform in importPlatforms" :key="platform.id" :value="platform.id">
            {{ t(`platforms.${platform.id}`, platform.label) }}
          </option>
        </select>
        <input
          ref="importInput"
          v-model.trim="externalPlaylistId"
          class="qp-input qp-import-input"
          :aria-label="t('roomPlaylists.externalPlaceholder')"
          :placeholder="t('roomPlaylists.externalPlaceholder')"
          :disabled="busy"
          @keydown.esc.prevent="showImportBar = false"
        />
        <button
          type="submit"
          class="qp-bar-btn qp-bar-btn--accent"
          :disabled="busy || !importPlatform || !externalPlaylistId"
        >
          {{ t('search.importPlaylist') }}
        </button>
      </form>
    </Transition>

    <!-- Track List with Inline Commentary -->
    <div
      :ref="viewMode === 'detail' ? setSortList : undefined"
      class="qp-scroll qp-detail-tracks"
      :aria-busy="playlistStore.loading"
    >
      <div
        v-for="(track, index) in playlistStore.selectedTracks"
        :key="track.id"
        class="qp-track-entry"
        :class="{ 'is-editing': editingTrackId === track.id, 'is-draggable': canSort }"
        :data-reorder-id="canSort ? track.id : undefined"
        @keydown.up.prevent="canSort && moveBy(track.id, -1)"
        @keydown.down.prevent="canSort && moveBy(track.id, 1)"
      >
        <div
          class="qp-pl-track-row"
          :class="{ 'is-clickable': canComment }"
          @click="onTrackRowClick(track)"
        >
          <span class="qp-track-index">{{ index + 1 }}</span>

          <CoverImage
            v-if="track.music.coverUrl"
            :src="track.music.coverUrl"
            alt=""
            class="qp-cover qp-cover--36"
          />
          <span v-else class="qp-cover qp-cover--36 qp-cover--placeholder" aria-hidden="true">
            <Music :size="16" :stroke-width="1.75" aria-hidden="true" />
          </span>

          <div class="qp-track-meta">
            <p class="qp-track-title">{{ track.music.name }}</p>
            <p class="qp-track-artist">{{ track.music.artists?.join(' / ') || track.music.platform }}</p>
          </div>

          <div class="qp-track-actions" data-no-drag @click.stop>
            <button
              v-if="scope === 'personal' && hasRoom"
              type="button"
              class="qp-icon-btn qp-icon-btn--sm qp-icon-btn--accent"
              :aria-label="t('roomPlaylists.addToQueue')"
              :title="t('roomPlaylists.addToQueue')"
              :disabled="busy || !commands.canOperate"
              @click="commands.enqueue(track.music.platform, track.music.id)"
            >
              <Plus :size="15" :stroke-width="1.75" aria-hidden="true" />
            </button>
            <button
              v-if="canEdit || isLikedPlaylist"
              type="button"
              class="qp-icon-btn qp-icon-btn--sm qp-icon-btn--danger"
              :aria-label="t('queue.remove')"
              :title="t('queue.remove')"
              :disabled="busy"
              @click="deleteTrack(track)"
            >
              <HeartMinus v-if="isLikedPlaylist" :size="15" :stroke-width="1.75" aria-hidden="true" />
              <Minus v-else :size="15" :stroke-width="1.75" aria-hidden="true" />
            </button>
          </div>
        </div>

        <!-- Commentary Display -->
        <p
          v-if="track.notes && editingTrackId !== track.id"
          class="qp-commentary-display"
          :class="{ 'is-clickable': canComment }"
          @click="canComment && editNotes(track)"
        >
          {{ track.notes }}
        </p>

        <!-- Inline Commentary Textarea (no label, no hint text) -->
        <Transition name="qp-slide">
          <div v-if="editingTrackId === track.id" class="qp-commentary-wrap">
            <textarea
              v-model="trackNotes"
              rows="2"
              maxlength="1000"
              data-playlist-note
              class="qp-commentary-textarea"
              :aria-label="t('roomPlaylists.editNotes')"
              :placeholder="t('roomPlaylists.editNotes')"
              @keydown.enter.exact.prevent="saveNotes(track)"
              @keydown.esc.prevent="cancelNotes"
              @blur="cancelNotes"
            ></textarea>
          </div>
        </Transition>
      </div>

      <div
        v-if="playlistStore.loading && !playlistStore.selectedTracks.length"
        class="qp-skeleton-list"
        aria-busy="true"
        :aria-label="t('roomPlaylists.loading')"
      >
        <div v-for="i in 5" :key="i" class="qp-skeleton-row qp-skeleton-row--track">
          <div class="qp-skeleton-bar qp-skeleton-bar--idx animate-shimmer"></div>
          <div class="qp-skeleton-box qp-cover--36 animate-shimmer"></div>
          <div class="qp-skeleton-lines">
            <div class="qp-skeleton-bar animate-shimmer" :style="{ width: `${48 + (i * 13) % 34}%` }"></div>
            <div class="qp-skeleton-bar qp-skeleton-bar--sub animate-shimmer" :style="{ width: `${30 + (i * 9) % 22}%` }"></div>
          </div>
        </div>
      </div>

      <div v-else-if="!playlistStore.loading && !playlistStore.selectedTracks.length" class="qp-empty">
        <span class="qp-empty-icon" aria-hidden="true">
          <Music :size="22" :stroke-width="1.75" aria-hidden="true" />
        </span>
        <div class="qp-empty-desc">{{ t('roomPlaylists.emptyTracks') }}</div>
      </div>
    </div>

    <!-- Single 38px Footer Bar -->
    <footer class="qp-bottom-bar">
      <div v-if="editor === 'delete'" class="qp-delete-confirm">
        <span class="qp-delete-text" role="status">
          {{ t('roomPlaylists.deleteConfirm', { name: playlistStore.selectedPlaylist.name }) }}
        </span>
        <div class="qp-bottom-actions">
          <button type="button" class="qp-bar-btn qp-bar-btn--danger" :disabled="busy" @click="deleteSelected">
            {{ t('roomPlaylists.delete') }}
          </button>
          <button type="button" class="qp-bar-btn" :disabled="busy" @click="editor = null">
            {{ t('common.cancel') }}
          </button>
        </div>
      </div>

      <template v-else>
        <div class="qp-bottom-actions">
          <select
            v-model="exportFormat"
            class="qp-select qp-export-select"
            :aria-label="t('common.export')"
          >
            <option v-for="format in exportFormats" :key="format" :value="format">
              {{ format.toUpperCase() }}
            </option>
          </select>
          <button type="button" class="qp-bar-btn" :disabled="busy" @click="exportSelected">
            {{ t('common.export') }}
          </button>
        </div>

        <div v-if="canEdit" class="qp-bottom-actions">
          <button type="button" class="qp-bar-btn" :disabled="busy" @click="openRename">
            {{ t('roomPlaylists.rename') }}
          </button>
          <button type="button" class="qp-bar-btn qp-bar-btn--danger" :disabled="busy" @click="editor = 'delete'">
            {{ t('roomPlaylists.delete') }}
          </button>
        </div>
      </template>
    </footer>
  </div>
  </Transition>
</div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Camera,
  ChevronLeft,
  ChevronRight,
  Disc3,
  Download,
  Heart,
  HeartMinus,
  ListMusic,
  Lock,
  Minus,
  Music,
  Play,
  Plus,
} from 'lucide-vue-next'
import { useQuery } from '@tanstack/vue-query'
import { musicApi } from '../../../api/music'
import type { PlaylistTrack } from '../../../contracts/generated/models'
import { useLikedSongsStore, type LikedSong } from '../../../domains/playback/likedSongs'
import { createLikedSongsFilename, createLikedSongsText } from '../../../utils/likedSongs'
import { useRoomCommandStore } from '../../../domains/realtime/roomCommandStore'
import { bindPlaylistSort } from '../../../domains/playlists/playlistSort'
import {
  moveUserPlaylist,
  reorderPlaylistTracks,
  updatePlaylistTrackNotes,
  uploadPlaylistCover,
  type PlaylistScope,
} from '../../../domains/playlists/playlistService'
import { queryKeys } from '../../../domains/queryKeys'
import { useRoomStore } from '../../../stores/room'
import { useRoomPlaylistsStore } from '../../../stores/roomPlaylists'
import { useUserStore } from '../../../stores/user'
import { useUserPlaylistsStore } from '../../../stores/userPlaylists'
import CoverImage from '../../CoverImage.vue'

const props = defineProps<{
  scope: PlaylistScope['type']
}>()

const emit = defineEmits<{
  'update:scope': [value: PlaylistScope['type']]
  'update:busy': [value: boolean]
  'update:error': [value: string]
}>()

const commands = useRoomCommandStore()
const liked = useLikedSongsStore()
const room = useRoomStore()
const user = useUserStore()
const roomPlaylists = useRoomPlaylistsStore()
const personalPlaylists = useUserPlaylistsStore()
const { t } = useI18n()

const scopes = ['room', 'personal'] as const
const exportFormats = ['txt', 'csv', 'json'] as const
const scope = computed(() => props.scope)
const viewMode = ref<'index' | 'detail'>('index')
const viewingVirtualLiked = ref(false)
const playlistNavTransition = ref<'qp-push' | 'qp-pop'>('qp-push')
const playlistStore = computed(() => (scope.value === 'room' ? roomPlaylists : personalPlaylists))
const showVirtualLikedEntry = computed(() => (
  scope.value === 'personal'
  && !personalPlaylists.playlists.some(playlist => playlist.systemKey === 'liked-songs')
))
const requestScope = computed<PlaylistScope>(() => (
  scope.value === 'room'
    ? { type: 'room', roomId: room.currentRoomId }
    : { type: 'personal', publicId: user.publicId }
))
const hasRoom = computed(() => Boolean(room.currentRoomId))
const canEdit = computed(() => Boolean(playlistStore.value.selectedPlaylist && !playlistStore.value.selectedPlaylist.systemKey))
const isLikedPlaylist = computed(() => scope.value === 'personal' && playlistStore.value.selectedPlaylist?.systemKey === 'liked-songs')
const canComment = computed(() => canEdit.value || isLikedPlaylist.value)
const coverUrl = computed(() => playlistStore.value.selectedPlaylist?.coverUrl || playlistStore.value.selectedTracks[0]?.music.coverUrl || '')
const canSort = computed(() => (
  scope.value === 'personal'
  && !user.isGuest
  && (viewMode.value === 'index'
    ? personalPlaylists.playlists.filter(playlist => !playlist.systemKey).length > 1
    : canEdit.value
      && playlistStore.value.selectedTracks.length > 1
      && playlistStore.value.selectedTracks.length === playlistStore.value.selectedPlaylist?.trackCount)
))

const busy = ref(false)
const showCreateForm = ref(false)
const showImportBar = ref(false)
const newName = ref('')
const importPlatform = ref<string>('')
const playlistProviders = new Set(['netease', 'qq', 'bilibili', 'youtube', 'soundcloud', 'squidify'])
const platformsQuery = useQuery({
  queryKey: computed(() => queryKeys.platforms.byRoom(room.currentRoomId)),
  queryFn: ({ queryKey: [, roomId], signal }) => musicApi.getPlatforms(roomId, signal),
})
const importPlatforms = computed(() => platformsQuery.data.value?.filter(platform => playlistProviders.has(platform.id)) ?? [])

watch(importPlatforms, (platforms) => {
  if (!platforms.some(platform => platform.id === importPlatform.value)) {
    importPlatform.value = platforms[0]?.id ?? ''
  }
}, { immediate: true })

watch(busy, (val) => emit('update:busy', val))

const externalPlaylistId = ref('')
const exportFormat = ref<(typeof exportFormats)[number]>('txt')
const editor = ref<'rename' | 'delete' | null>(null)
const renameValue = ref('')
const editingTrackId = ref<string | null>(null)
const trackNotes = ref('')
const sortList = ref<HTMLElement | null>(null)
const likedListRef = ref<HTMLElement | null>(null)
const coverInput = ref<HTMLInputElement | null>(null)
const renameInput = ref<HTMLInputElement | null>(null)
const createInput = ref<HTMLInputElement | null>(null)
const importInput = ref<HTMLInputElement | null>(null)

watch([sortList, canSort, busy, viewMode], ([list, enabled, pending], _, onCleanup) => {
  if (list && enabled && !pending) onCleanup(bindPlaylistSort(list, move))
}, { flush: 'post' })

watch([() => user.publicId, () => room.currentRoomId], goBack)

async function run(action: () => Promise<unknown>) {
  if (busy.value) return
  busy.value = true
  emit('update:error', '')
  try {
    await action()
  } catch (error) {
    emit('update:error', error instanceof Error ? error.message : t('roomPlaylists.actionFailed'))
  } finally {
    busy.value = false
  }
}

function goBack() {
  playlistNavTransition.value = 'qp-pop'
  viewMode.value = 'index'
  viewingVirtualLiked.value = false
  showImportBar.value = false
  editor.value = null
  editingTrackId.value = null
  emit('update:error', '')
}

function openVirtualLiked() {
  playlistNavTransition.value = 'qp-push'
  viewingVirtualLiked.value = true
  viewMode.value = 'detail'
  showCreateForm.value = false
  showImportBar.value = false
  editingTrackId.value = null
  editor.value = null
}

function queueAllLiked() {
  if (!hasRoom.value || !commands.canOperate || !liked.likedSongs.length) return
  for (const song of liked.likedSongs) {
    commands.enqueue(song.platform, song.id)
  }
}

function exportLikedSongs() {
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

const formatArtists = (artists?: string[] | null) => (
  Array.isArray(artists) && artists.length ? artists.join(' / ') : t('common.unknownArtist')
)

async function toggleImportBar() {
  showImportBar.value = !showImportBar.value
  if (showImportBar.value) {
    await nextTick()
    importInput.value?.focus()
  }
}

function switchScope(next: PlaylistScope['type']) {
  if (busy.value || next === scope.value) return
  emit('update:scope', next)
  showCreateForm.value = false
  goBack()
  if (next === 'personal' && !user.isGuest) void run(() => personalPlaylists.loadPlaylists())
}

async function openCreatePlaylist() {
  if (viewMode.value === 'detail') goBack()
  showCreateForm.value = true
  await nextTick()
  createInput.value?.focus()
}

function closeCreatePlaylist() {
  showCreateForm.value = false
  newName.value = ''
}

function select(playlistId: string) {
  playlistNavTransition.value = 'qp-push'
  viewingVirtualLiked.value = false
  playlistStore.value.selectedPlaylistId = playlistId
  viewMode.value = 'detail'
  showCreateForm.value = false
  showImportBar.value = false
  editingTrackId.value = null
  editor.value = null
  void run(() => playlistStore.value.loadTracks(playlistId))
}

function create() {
  if (!newName.value) return
  void run(async () => {
    await playlistStore.value.createPlaylist(newName.value)
    newName.value = ''
    showCreateForm.value = false
  })
}

async function openRename() {
  if (!canEdit.value) return
  renameValue.value = playlistStore.value.selectedPlaylist?.name ?? ''
  editor.value = 'rename'
  await nextTick()
  renameInput.value?.select()
}

function cancelRename() {
  editor.value = null
}

function commitRename() {
  if (editor.value !== 'rename') return
  const id = playlistStore.value.selectedPlaylistId
  const current = playlistStore.value.selectedPlaylist?.name ?? ''
  if (!id || !renameValue.value || renameValue.value === current) {
    editor.value = null
    return
  }
  void run(async () => {
    await playlistStore.value.renamePlaylist(id, renameValue.value)
    editor.value = null
  })
}

function deleteSelected() {
  const id = playlistStore.value.selectedPlaylistId
  if (!id) return
  void run(async () => {
    await playlistStore.value.deletePlaylist(id)
    goBack()
  })
}

function importExternal() {
  const id = playlistStore.value.selectedPlaylistId
  if (!id || !importPlatform.value || !externalPlaylistId.value) return
  void run(async () => {
    if (scope.value === 'room') await roomPlaylists.importExternal(importPlatform.value, externalPlaylistId.value)
    else await personalPlaylists.importPlaylist(id, importPlatform.value, externalPlaylistId.value)
    externalPlaylistId.value = ''
    showImportBar.value = false
  })
}

function playSelected() {
  if (scope.value === 'room') roomPlaylists.playPlaylist()
  else void run(() => personalPlaylists.enqueue())
}

function uploadCover(event: Event) {
  if (!(event.target instanceof HTMLInputElement)) return
  const file = event.target.files?.[0]
  event.target.value = ''
  const id = playlistStore.value.selectedPlaylistId
  if (file && id && canEdit.value) void run(() => uploadPlaylistCover(requestScope.value, id, file))
}

function onTrackRowClick(track: PlaylistTrack) {
  if (!canComment.value) return
  if (editingTrackId.value === track.id) {
    cancelNotes()
    return
  }
  void editNotes(track)
}

function onLikedSongRowClick(song: LikedSong) {
  if (editingTrackId.value === song.key) {
    cancelNotes()
    return
  }
  void editLikedNotes(song)
}

async function editLikedNotes(song: LikedSong) {
  if (editingTrackId.value === song.key) return
  trackNotes.value = song.notes ?? ''
  editingTrackId.value = song.key
  await nextTick()
  likedListRef.value?.querySelector<HTMLTextAreaElement>('[data-playlist-note]')?.focus()
}

function saveLikedNotes(song: LikedSong) {
  if (editingTrackId.value !== song.key || busy.value) return
  const notes = trackNotes.value
  editingTrackId.value = null
  if (notes.trim() === (song.notes ?? '').trim()) return
  void run(() => liked.updateTrackNotes(song.key, notes))
}

async function editNotes(track: PlaylistTrack) {
  if (editingTrackId.value === track.id) return
  trackNotes.value = track.notes ?? ''
  editingTrackId.value = track.id
  await nextTick()
  sortList.value?.querySelector<HTMLTextAreaElement>('[data-playlist-note]')?.focus()
}

function cancelNotes() {
  editingTrackId.value = null
}

function saveNotes(track: PlaylistTrack) {
  if (editingTrackId.value !== track.id || busy.value) return
  const id = playlistStore.value.selectedPlaylistId
  const notes = trackNotes.value
  editingTrackId.value = null
  if (notes === (track.notes ?? '')) return
  void run(async () => {
    await updatePlaylistTrackNotes(requestScope.value, id, track.id, notes)
  })
}

function deleteTrack(track: PlaylistTrack) {
  void run(async () => {
    const playlistId = playlistStore.value.selectedPlaylistId
    if (isLikedPlaylist.value) {
      await liked.removeAndSync(`${track.music.platform}:${track.music.id}`)
      await personalPlaylists.loadPlaylists()
      await personalPlaylists.loadTracks(playlistId)
    } else {
      await playlistStore.value.deleteTrack(playlistId, track.id)
    }
  })
}

function reorderIds() {
  return viewMode.value === 'index'
    ? personalPlaylists.playlists.filter(playlist => !playlist.systemKey).map(playlist => playlist.id)
    : playlistStore.value.selectedTracks.map(track => track.id)
}

function moveBy(id: string, delta: number) {
  const ids = reorderIds()
  move(id, ids.indexOf(id) + delta)
}

function move(id: string, position: number) {
  const target = requestScope.value
  if (!canSort.value || target.type !== 'personal') return
  const ids = reorderIds()
  const previous = ids.indexOf(id)
  if (previous < 0 || position === previous || position < 0 || position >= ids.length) return
  if (viewMode.value === 'index') {
    void run(() => moveUserPlaylist(target, id, position))
  } else {
    ids.splice(previous, 1)
    ids.splice(position, 0, id)
    const playlistId = playlistStore.value.selectedPlaylistId
    void run(() => reorderPlaylistTracks(target, playlistId, ids))
  }
}

function exportSelected() {
  const id = playlistStore.value.selectedPlaylistId
  const format = exportFormat.value
  const currentScope = scope.value
  if (!id) return
  void run(async () => {
    const content = currentScope === 'room'
      ? await roomPlaylists.exportPlaylist(format, id)
      : await personalPlaylists.exportPlaylist(id, format)
    const mime = { txt: 'text/plain', csv: 'text/csv', json: 'application/json' }[format]
    const url = URL.createObjectURL(new Blob([content], { type: `${mime};charset=utf-8` }))
    const link = document.createElement('a')
    link.href = url
    link.download = `musicparty-${currentScope}-playlist-${id}.${format}`
    link.click()
    setTimeout(() => URL.revokeObjectURL(url), 0)
  })
}

function setSortList(element: Element | ComponentPublicInstance | null) {
  if (element instanceof HTMLElement) sortList.value = element
}

defineExpose({
  openCreatePlaylist,
})
</script>

<style scoped>
/* ============================================================
   PLAYLISTS INDEX VIEW (36px scope bar + flat rows)
   ============================================================ */
.qp-scope-bar {
  height: 40px;
  padding: 0 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-1);
  flex-shrink: 0;
}

.qp-capsule-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  border-radius: var(--radius-full);
  background: var(--surface-control);
  border: 1px solid var(--border-subtle);
}

.qp-capsule-btn {
  height: 24px;
  padding: 0 11px;
  border-radius: var(--radius-full);
  font-size: 11.5px;
  font-weight: 500;
  color: var(--text-tertiary);
  transition: background var(--motion-exit) var(--motion-ease-standard),
    color var(--motion-exit) var(--motion-ease-standard),
    transform var(--motion-press) var(--motion-ease-standard);
}

.qp-capsule-btn:hover:not(:disabled) {
  color: var(--text-primary);
}

.qp-capsule-btn.is-active {
  background: var(--accent-muted);
  color: var(--accent);
  font-weight: 600;
}

.qp-create-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: var(--spacing-sm) 10px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-2);
  flex-shrink: 0;
}

.qp-pl-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.qp-pl-row-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.qp-pl-row {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px var(--spacing-sm);
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  text-align: left;
  transition: background var(--motion-exit) var(--motion-ease-out),
    border-color var(--motion-exit) var(--motion-ease-out);
}

.qp-pl-row:hover:not(:disabled),
.qp-pl-row.is-selected {
  background: var(--surface-2);
  border-color: var(--border-subtle);
}

.qp-pl-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.qp-pl-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.qp-pl-sub {
  font-size: 11.5px;
  color: var(--text-tertiary);
}

.qp-sys-tag {
  color: var(--accent);
  font-weight: 500;
}

.qp-chevron {
  color: var(--text-tertiary);
  flex-shrink: 0;
  transition: transform var(--motion-exit) var(--motion-ease-standard),
    color var(--motion-exit) var(--motion-ease-standard);
}

.qp-pl-row:hover:not(:disabled) .qp-chevron {
  transform: translateX(2px);
  color: var(--text-secondary);
}

.qp-pl-row-wrap.is-draggable .qp-pl-row,
.qp-track-entry.is-draggable .qp-pl-track-row {
  cursor: grab;
}

.qp-pl-row-wrap.is-draggable .qp-pl-row:active,
.qp-track-entry.is-draggable .qp-pl-track-row:active {
  cursor: grabbing;
}

.qp-cover--liked {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--accent-subtle);
  border: 1px solid var(--border-accent);
  color: var(--accent);
}

.qp-guest-note {
  margin-top: 8px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  background: var(--surface-control);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-tertiary);
}

/* ============================================================
   COVERS
   ============================================================ */
.qp-cover {
  flex-shrink: 0;
  object-fit: cover;
  border-radius: var(--radius-sm);
}

.qp-cover--36 { width: 36px; height: 36px; }
.qp-cover--40 { width: 40px; height: 40px; border-radius: var(--radius-md); }
.qp-cover--44 { width: 44px; height: 44px; border-radius: var(--radius-md); }

.qp-cover--placeholder {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--surface-3);
  border: 1px solid var(--border-subtle);
  color: var(--accent);
}

/* ============================================================
   PLAYLIST DETAIL VIEW (44px header + import bar + commentary)
   ============================================================ */
.qp-detail-header {
  min-height: 44px;
  padding: 6px 10px 6px 6px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-1);
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-shrink: 0;
}

.qp-detail-cover-wrap {
  position: relative;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
}

.qp-cover-cam {
  position: absolute;
  inset: 0;
  border-radius: var(--radius-md);
  background: var(--surface-scrim);
  color: var(--text-primary);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity var(--motion-exit) var(--motion-ease-out);
}

.qp-detail-cover-wrap:hover .qp-cover-cam,
.qp-detail-cover-wrap:focus-within .qp-cover-cam {
  opacity: 1;
}

.qp-detail-title-wrap {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
}

.qp-detail-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  flex-shrink: 0;
}

.qp-detail-name-btn {
  max-width: 100%;
  text-align: left;
  padding: 2px 4px;
  margin-left: -4px;
  border-radius: var(--radius-xs);
  transition: background var(--motion-exit) var(--motion-ease-out);
}

.qp-detail-name-btn:hover {
  background: var(--surface-control);
}

.qp-detail-name {
  display: block;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.qp-inline-title-input {
  width: 100%;
  height: 28px;
  padding: 0 var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--surface-2);
  border: 1px solid var(--border-accent);
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.qp-import-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--surface-2);
  flex-shrink: 0;
}

.qp-input,
.qp-select {
  height: 26px;
  padding: 0 var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--surface-1);
  border: 1px solid var(--border-subtle);
  font-size: 11.5px;
  color: var(--text-primary);
}

.qp-input::placeholder {
  color: var(--text-tertiary);
}

.qp-input:focus,
.qp-select:focus {
  border-color: var(--border-accent);
}

.qp-import-select {
  flex: 0 0 auto;
  max-width: 104px;
}

.qp-import-input {
  flex: 1;
  min-width: 0;
}

.qp-export-select {
  width: 62px;
  padding: 0 6px;
  background: var(--surface-1);
}

/* Playlist Track Rows + Commentary */
.qp-detail-tracks {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.qp-track-entry {
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  transition: background var(--motion-exit) var(--motion-ease-out),
    border-color var(--motion-exit) var(--motion-ease-out);
}

.qp-track-entry:hover,
.qp-track-entry.is-editing {
  background: var(--surface-2);
  border-color: var(--border-subtle);
}

.qp-pl-track-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: 6px var(--spacing-sm);
  position: relative;
}

.qp-pl-track-row.is-clickable {
  cursor: pointer;
}

.qp-track-index {
  width: 18px;
  font-size: 11px;
  color: var(--text-tertiary);
  text-align: center;
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.qp-track-meta {
  flex: 1;
  min-width: 0;
}

.qp-track-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.qp-track-artist {
  font-size: 11.5px;
  color: var(--text-tertiary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.qp-track-actions {
  display: flex;
  align-items: center;
  gap: 2px;
  opacity: 0;
  pointer-events: none;
  flex-shrink: 0;
  transition: opacity var(--motion-exit) var(--motion-ease-out);
}

.qp-track-entry:hover .qp-track-actions,
.qp-track-entry:focus-within .qp-track-actions,
.qp-track-entry.is-editing .qp-track-actions {
  opacity: 1;
  pointer-events: auto;
}

.qp-commentary-display {
  margin: 0 var(--spacing-sm) var(--spacing-sm) 34px;
  padding: 5px var(--spacing-sm);
  border-left: 2px solid var(--border-accent);
  background: var(--accent-subtle);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  font-size: 11.5px;
  line-height: 1.45;
  color: var(--text-secondary);
  word-break: break-word;
}

.qp-commentary-display.is-clickable {
  cursor: pointer;
}

.qp-commentary-display.is-clickable:hover {
  background: var(--accent-muted);
}

.qp-commentary-wrap {
  margin: 0 var(--spacing-sm) var(--spacing-sm) 34px;
}

.qp-commentary-textarea {
  width: 100%;
  min-height: 48px;
  padding: 6px var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: var(--surface-1);
  border: 1px solid var(--border-accent);
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.45;
  resize: vertical;
  display: block;
}

.qp-commentary-textarea::placeholder {
  color: var(--text-tertiary);
}

.qp-delete-confirm {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--spacing-sm);
}

.qp-delete-text {
  font-size: 11.5px;
  color: var(--error);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Skeleton Loading Rows */
.qp-skeleton-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 2px 0;
}

.qp-skeleton-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  border-radius: var(--radius-md);
}

.qp-skeleton-row--track {
  padding: 5px 8px;
}

.qp-skeleton-box {
  flex-shrink: 0;
  border-radius: var(--radius-sm);
}

.qp-skeleton-lines {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.qp-skeleton-bar {
  height: 11px;
  border-radius: var(--radius-xs);
}

.qp-skeleton-bar--sub {
  height: 9px;
  opacity: 0.72;
}

.qp-skeleton-bar--idx {
  width: 14px;
  height: 10px;
  flex-shrink: 0;
}

@container (max-width: 315px) {
  .qp-play-all-label {
    display: none;
  }

  .qp-import-select {
    max-width: 84px;
  }
}
</style>
