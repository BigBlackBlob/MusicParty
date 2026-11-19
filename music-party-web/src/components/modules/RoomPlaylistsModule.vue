<template>
  <section class="playlist-module" :aria-label="t('roomPlaylists.title')">
    <p v-if="message || store.error" class="message-line" role="alert">
      {{ message || store.error }}
    </p>
    <p v-if="scope === 'personal' && user.isGuest" class="guest-state">
      {{ t("roomPlaylists.personalUnavailable") }}
    </p>
    <div v-else class="d3-shell">
      <div class="d3-tab-bar" role="tablist">
        <div
          class="scope-toggle"
          role="group"
          :aria-label="t('roomPlaylists.title')"
        >
          <button
            v-for="option in scopes"
            :key="option"
            type="button"
            class="d3-tab scope-btn"
            role="tab"
            :class="{ on: scope === option }"
            :aria-pressed="scope === option"
            :aria-selected="scope === option"
            @click="switchScope(option)"
          >
            {{
              t(
                option === "room"
                  ? "roomPlaylists.roomScope"
                  : "roomPlaylists.personalScope",
              )
            }}
          </button>
        </div>
        <template v-if="viewMode === 'detail'">
          <span class="d3-tab-sep" aria-hidden="true">›</span>
          <button
            class="d3-tab on"
            type="button"
            disabled
          >
            {{ store.selectedPlaylist?.name }}
          </button>
          <button
            type="button"
            class="icon-btn"
            :aria-label="t('common.back')"
            :title="t('common.back')"
            @click="goBack"
          >
            <span class="material-symbols-outlined" aria-hidden="true"
              >close</span
            >
          </button>
        </template>
      </div>

      <div v-if="viewMode === 'index'" class="d3-pane show">
        <form class="d3-index-toolbar" @submit.prevent="create">
          <input
            v-model.trim="newName"
            maxlength="64"
            class="text-field"
            :aria-label="t('roomPlaylists.newPlaceholder')"
            :placeholder="t('roomPlaylists.newPlaceholder')"
            :disabled="busy"
          /><button
            type="submit"
            class="chip-btn chip-primary"
            :disabled="busy || !newName"
          >
            {{ t("rooms.create") }}
          </button>
        </form>
        <div
          :ref="viewMode === 'index' ? setSortList : undefined"
          class="d3-playlist-list"
          :aria-busy="store.loading"
        >
          <div
            v-for="playlist in store.playlists"
            :key="playlist.id"
            class="d3-playlist-row-wrap"
            :data-reorder-id="
              canSort && !playlist.systemKey ? playlist.id : undefined
            "
          >
            <button
              type="button"
              class="d3-playlist-row"
              :class="{ selected: store.selectedPlaylistId === playlist.id }"
              :disabled="busy"
              @click="select(playlist.id)"
            >
              <CoverImage
                v-if="playlist.coverUrl"
                :src="playlist.coverUrl"
                alt=""
                class="cover-sq d3-playlist-cover"
              /><span
                v-else
                class="cover-sq d3-playlist-cover material-symbols-outlined"
                aria-hidden="true"
                >album</span
              ><span class="row-meta"
                ><span class="row-name">{{ playlist.name }}</span
                ><span class="row-sub"
                  ><template v-if="playlist.systemKey"
                    ><span class="sys-badge">System</span> &middot; </template
                  >{{ playlist.trackCount }} {{ t("queue.tracks") }}</span
                ></span
              ><span
                class="material-symbols-outlined row-chevron"
                aria-hidden="true"
                >chevron_right</span
              >
            </button>
            <button
              v-if="canSort && !playlist.systemKey"
              type="button"
              data-reorder-handle
              class="icon-btn drag-handle"
              :aria-label="t('roomPlaylists.reorder', { name: playlist.name })"
              :title="t('roomPlaylists.reorderHint')"
              :disabled="busy"
              @keydown.up.prevent="moveBy(playlist.id, -1)"
              @keydown.down.prevent="moveBy(playlist.id, 1)"
            >
              <span class="material-symbols-outlined" aria-hidden="true"
                >drag_indicator</span
              >
            </button>
          </div>
          <p
            v-if="!store.loading && !store.playlists.length"
            class="empty-state"
          >
            {{ t("roomPlaylists.empty") }}
          </p>
        </div>
      </div>

      <div v-else class="d3-pane show">
        <template v-if="store.selectedPlaylist">
          <header class="d3-detail-header">
            <div class="cover-wrap">
              <CoverImage
                v-if="coverUrl"
                :src="coverUrl"
                alt=""
                class="cover-image"
              /><span
                v-else
                class="material-symbols-outlined"
                aria-hidden="true"
                >album</span
              ><button
                v-if="canEdit"
                type="button"
                class="cover-cam-btn"
                :aria-label="t('roomPlaylists.uploadCover')"
                :title="t('roomPlaylists.uploadCover')"
                :disabled="busy"
                @click="coverInput?.click()"
              >
                <span class="material-symbols-outlined" aria-hidden="true"
                  >photo_camera</span
                >
              </button>
              <input
                ref="coverInput"
                type="file"
                accept="image/png,image/jpeg"
                hidden
                @change="uploadCover"
              />
            </div>
            <div class="hd-body">
              <h2 class="hd-name">
                {{ store.selectedPlaylist.name || t("roomPlaylists.loading") }}
              </h2>
              <p class="hd-sub">
                {{ store.selectedPlaylist.trackCount }}
                {{ t("queue.tracks") }} &middot;
                {{
                  t(
                    scope === "room"
                      ? "roomPlaylists.roomScope"
                      : "roomPlaylists.personalScope",
                  )
                }}
              </p>
              <p v-if="store.selectedPlaylist.description" class="hd-desc">
                {{ store.selectedPlaylist.description }}
              </p>
            </div>
            <button
              v-if="store.selectedTracks.length && hasRoom"
              type="button"
              class="chip-btn chip-primary play-all"
              :aria-label="t('roomPlaylists.playAll')"
              :title="t('roomPlaylists.playAll')"
              :disabled="busy || !commands.canOperate"
              @click="playSelected"
            >
              <span class="material-symbols-outlined" aria-hidden="true"
                >playlist_play</span
              >{{ t("roomPlaylists.playAll") }}
            </button>
          </header>
          <form
            v-if="canEdit"
            class="d3-import-bar"
            @submit.prevent="importExternal"
          >
            <select
              v-model="importPlatform"
              class="text-field"
              :aria-label="t('roomPlaylists.importPlatform')"
              :disabled="busy"
            >
              <option value="netease">NetEase</option>
              <option value="qq">QQ Music</option>
            </select>
            <input
              v-model.trim="externalPlaylistId"
              class="text-field import-id"
              :aria-label="t('roomPlaylists.externalPlaceholder')"
              :placeholder="t('roomPlaylists.externalPlaceholder')"
              :disabled="busy"
            /><button
              type="submit"
              class="chip-btn"
              :disabled="busy || !externalPlaylistId"
            >
              {{ t("search.importPlaylist") }}
            </button>
          </form>
          <div
            :ref="viewMode === 'detail' ? setSortList : undefined"
            class="d3-track-list"
            :aria-busy="store.loading"
          >
            <div
              v-for="(track, index) in store.selectedTracks"
              :key="track.id"
              class="track-entry"
              :data-reorder-id="canSort ? track.id : undefined"
            >
              <div
                class="d3-track-row"
                :class="{ selected: selectedTrackId === track.id }"
                @click="selectedTrackId = selectedTrackId === track.id ? null : track.id"
              >
                <div class="d3-track-num">{{ index + 1 }}</div>
                <CoverImage
                  v-if="track.music.coverUrl"
                  :src="track.music.coverUrl"
                  alt=""
                  class="cover-sq d3-track-cover"
                /><span
                  v-else
                  class="cover-sq d3-track-cover material-symbols-outlined"
                  aria-hidden="true"
                  >music_note</span
                >
                <div class="track-meta">
                  <p class="track-name">{{ track.music.name }}</p>
                  <p class="track-art">
                    {{
                      track.music.artists?.join(" / ") || track.music.platform
                    }}
                  </p>
                  <p
                    v-if="track.notes && editingTrackId !== track.id"
                    class="track-note-line"
                  >
                    {{ track.notes }}
                  </p>
                </div>
                <div class="d3-action-bar">
                  <button
                    v-if="scope === 'personal' && hasRoom"
                    type="button"
                    class="icon-btn"
                    :aria-label="t('roomPlaylists.addToQueue')"
                    :title="t('roomPlaylists.addToQueue')"
                    :disabled="busy || !commands.canOperate"
                    @click.stop="
                      commands.enqueue(track.music.platform, track.music.id)
                    "
                  >
                    <span class="material-symbols-outlined" aria-hidden="true"
                      >add</span
                    >
                  </button>
                  <button
                    v-if="canEdit"
                    type="button"
                    class="icon-btn"
                    :class="{ 'accent-icon': track.notes }"
                    :aria-expanded="editingTrackId === track.id"
                    :aria-label="t('roomPlaylists.editNotes')"
                    :title="t('roomPlaylists.editNotes')"
                    :disabled="busy"
                    @click.stop="editNotes(track)"
                  >
                    <span class="material-symbols-outlined" aria-hidden="true"
                      >edit_note</span
                    >
                  </button>
                  <button
                    v-if="canEdit || isLikedPlaylist"
                    type="button"
                    class="icon-btn"
                    :aria-label="t('queue.remove')"
                    :title="t('queue.remove')"
                    :disabled="busy"
                    @click.stop="deleteTrack(track)"
                  >
                    <span
                      class="material-symbols-outlined"
                      aria-hidden="true"
                      >{{ isLikedPlaylist ? "heart_minus" : "remove" }}</span
                    >
                  </button>
                </div>
                <button
                  v-if="canSort && viewMode === 'detail'"
                  type="button"
                  data-reorder-handle
                  class="icon-btn drag-handle"
                  :aria-label="
                    t('roomPlaylists.reorder', { name: track.music.name })
                  "
                  :title="t('roomPlaylists.reorderHint')"
                  :disabled="busy"
                  @click.stop
                  @keydown.up.prevent="moveBy(track.id, -1)"
                  @keydown.down.prevent="moveBy(track.id, 1)"
                >
                  <span class="material-symbols-outlined" aria-hidden="true"
                    >drag_indicator</span
                  >
                </button>
              </div>
              <div
                v-if="editingTrackId === track.id"
                class="d3-note-row show"
              >
                <input
                  v-model="trackNotes"
                  maxlength="1000"
                  data-playlist-note
                  class="text-field"
                  :aria-label="t('roomPlaylists.editNotes')"
                  :placeholder="t('roomPlaylists.editNotes')"
                  @blur="saveNotes(track)"
                  @keydown.enter.prevent="saveNotes(track)"
                  @keydown.esc.prevent="editingTrackId = null"
                />
              </div>
            </div>
            <p
              v-if="!store.loading && !store.selectedTracks.length"
              class="empty-state"
            >
              {{ t("roomPlaylists.emptyTracks") }}
            </p>
          </div>
          <footer class="d3-footer">
            <form
              v-if="editor === 'rename'"
              class="footer-edit"
              @submit.prevent="rename"
            >
              <input
                ref="renameInput"
                v-model.trim="renameValue"
                class="text-field rename-field"
                maxlength="64"
                :aria-label="t('roomPlaylists.namePrompt')"
                @keydown.esc.prevent="editor = null"
              /><button
                type="submit"
                class="chip-btn chip-primary"
                :disabled="busy || !renameValue"
              >
                {{ t("roomPlaylists.save") }}
              </button>
              <button
                type="button"
                class="chip-btn"
                :disabled="busy"
                @click="editor = null"
              >
                {{ t("common.cancel") }}
              </button>
            </form>
            <div v-else-if="editor === 'delete'" class="d3-delete-confirm">
              <span role="status">{{
                t("roomPlaylists.deleteConfirm", {
                  name: store.selectedPlaylist.name,
                })
              }}</span>
              <button
                type="button"
                class="chip-btn chip-danger"
                :disabled="busy"
                @click="deleteSelected"
              >
                {{ t("roomPlaylists.delete") }}
              </button>
              <button
                type="button"
                class="chip-btn"
                :disabled="busy"
                @click="editor = null"
              >
                {{ t("common.cancel") }}
              </button>
            </div>
            <div v-else class="footer-row">
              <div class="footer-left">
                <select
                  v-model="exportFormat"
                  class="text-field export-format"
                  :aria-label="t('common.export')"
                >
                  <option
                    v-for="format in exportFormats"
                    :key="format"
                    :value="format"
                  >
                    {{ format.toUpperCase() }}
                  </option>
                </select>
                <button
                  type="button"
                  class="chip-btn"
                  :disabled="busy"
                  @click="exportSelected"
                >
                  {{ t("common.export") }}
                </button>
              </div>
              <div v-if="canEdit" class="footer-right">
                <button
                  type="button"
                  class="chip-btn"
                  :disabled="busy"
                  @click="openRename"
                >
                  {{ t("roomPlaylists.rename") }}
                </button>
                <button
                  type="button"
                  class="chip-btn chip-danger"
                  :disabled="busy"
                  @click="editor = 'delete'"
                >
                  {{ t("roomPlaylists.delete") }}
                </button>
              </div>
            </div>
          </footer>
        </template>
        <p v-else class="empty-state">{{ t("roomPlaylists.empty") }}</p>
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PlaylistTrack } from '../../contracts/generated/models'
import { useLikedSongsStore } from '../../domains/playback/likedSongs'
import { bindPlaylistSort } from '../../domains/playlists/playlistSort'
import { moveUserPlaylist, reorderPlaylistTracks, updatePlaylistTrackNotes, uploadPlaylistCover, type PlaylistScope } from '../../domains/playlists/playlistService'
import { useRoomCommandStore } from '../../domains/realtime/roomCommandStore'
import { useRoomStore } from '../../stores/room'
import { useRoomPlaylistsStore } from '../../stores/roomPlaylists'
import { useUserStore } from '../../stores/user'
import { useUserPlaylistsStore } from '../../stores/userPlaylists'
import CoverImage from '../CoverImage.vue'

const roomPlaylists = useRoomPlaylistsStore()
const personalPlaylists = useUserPlaylistsStore()
const room = useRoomStore()
const user = useUserStore()
const commands = useRoomCommandStore()
const liked = useLikedSongsStore()
const { t } = useI18n()

const scopes = ['room', 'personal'] as const
const exportFormats = ['txt', 'csv', 'json'] as const
const scope = ref<PlaylistScope['type']>('room')
const viewMode = ref<'index' | 'detail'>('index')
const store = computed(() => scope.value === 'room' ? roomPlaylists : personalPlaylists)
const requestScope = computed<PlaylistScope>(() => scope.value === 'room'
  ? { type: 'room', roomId: room.currentRoomId }
  : { type: 'personal', publicId: user.publicId })
const hasRoom = computed(() => Boolean(room.currentRoomId))
const canEdit = computed(() => Boolean(store.value.selectedPlaylist && !store.value.selectedPlaylist.systemKey))
const isLikedPlaylist = computed(() => scope.value === 'personal' && store.value.selectedPlaylist?.systemKey === 'liked-songs')
const coverUrl = computed(() => store.value.selectedPlaylist?.coverUrl || store.value.selectedTracks[0]?.music.coverUrl || '')
const canSort = computed(() => scope.value === 'personal' && !user.isGuest && (viewMode.value === 'index'
  ? personalPlaylists.playlists.filter(playlist => !playlist.systemKey).length > 1
  : canEdit.value && store.value.selectedTracks.length > 1 && store.value.selectedTracks.length === store.value.selectedPlaylist?.trackCount))

const busy = ref(false)
const message = ref('')
const newName = ref('')
const importPlatform = ref<'netease' | 'qq'>('netease')
const externalPlaylistId = ref('')
const exportFormat = ref<(typeof exportFormats)[number]>('txt')
const editor = ref<'rename' | 'delete' | null>(null)
const renameValue = ref('')
const editingTrackId = ref<string | null>(null)
const selectedTrackId = ref<string | null>(null)
const trackNotes = ref('')
const sortList = ref<HTMLElement | null>(null)
const coverInput = ref<HTMLInputElement | null>(null)
const renameInput = ref<HTMLInputElement | null>(null)

watch([sortList, canSort, busy, viewMode], ([list, enabled, pending], _, onCleanup) => {
  if (list && enabled && !pending) onCleanup(bindPlaylistSort(list, move))
}, { flush: 'post' })
watch([() => user.publicId, () => room.currentRoomId], goBack)

async function run(action: () => Promise<unknown>) {
  if (busy.value) return
  busy.value = true
  message.value = ''
  try {
    await action()
  } catch (error) {
    message.value = error instanceof Error ? error.message : t('roomPlaylists.actionFailed')
  } finally {
    busy.value = false
  }
}

function goBack() {
  viewMode.value = 'index'
  editor.value = null
  editingTrackId.value = null
  selectedTrackId.value = null
  message.value = ''
}

function switchScope(next: PlaylistScope['type']) {
  if (busy.value || next === scope.value) return
  scope.value = next
  goBack()
  if (next === 'personal' && !user.isGuest) void run(() => personalPlaylists.loadPlaylists())
}

function select(playlistId: string) {
  store.value.selectedPlaylistId = playlistId
  viewMode.value = 'detail'
  selectedTrackId.value = null
  void run(() => store.value.loadTracks(playlistId))
}

function create() {
  if (!newName.value) return
  void run(async () => {
    await store.value.createPlaylist(newName.value)
    newName.value = ''
  })
}

async function openRename() {
  renameValue.value = store.value.selectedPlaylist?.name ?? ''
  editor.value = 'rename'
  await nextTick()
  renameInput.value?.select()
}

function rename() {
  const id = store.value.selectedPlaylistId
  if (!id || !renameValue.value) return
  void run(async () => {
    await store.value.renamePlaylist(id, renameValue.value)
    editor.value = null
  })
}

function deleteSelected() {
  const id = store.value.selectedPlaylistId
  if (!id) return
  void run(async () => {
    await store.value.deletePlaylist(id)
    goBack()
  })
}

function importExternal() {
  const id = store.value.selectedPlaylistId
  if (!id || !externalPlaylistId.value) return
  void run(async () => {
    if (scope.value === 'room') await roomPlaylists.importExternal(importPlatform.value, externalPlaylistId.value)
    else await personalPlaylists.importPlaylist(id, importPlatform.value, externalPlaylistId.value)
    externalPlaylistId.value = ''
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
  const id = store.value.selectedPlaylistId
  if (file && id && canEdit.value) void run(() => uploadPlaylistCover(requestScope.value, id, file))
}

async function editNotes(track: PlaylistTrack) {
  if (editingTrackId.value === track.id) return
  trackNotes.value = track.notes ?? ''
  editingTrackId.value = track.id
  await nextTick()
  sortList.value?.querySelector<HTMLInputElement>('[data-playlist-note]')?.focus()
}

function saveNotes(track: PlaylistTrack) {
  if (editingTrackId.value !== track.id || busy.value) return
  const id = store.value.selectedPlaylistId
  const notes = trackNotes.value
  if (notes === (track.notes ?? '')) {
    editingTrackId.value = null
    return
  }
  void run(async () => {
    await updatePlaylistTrackNotes(requestScope.value, id, track.id, notes)
    if (editingTrackId.value === track.id) editingTrackId.value = null
  })
}

function deleteTrack(track: PlaylistTrack) {
  void run(async () => {
    const playlistId = store.value.selectedPlaylistId
    if (isLikedPlaylist.value) {
      await liked.removeAndSync(`${track.music.platform}:${track.music.id}`)
      await personalPlaylists.loadPlaylists()
      await personalPlaylists.loadTracks(playlistId)
    } else {
      await store.value.deleteTrack(playlistId, track.id)
    }
  })
}

function reorderIds() {
  return viewMode.value === 'index'
    ? personalPlaylists.playlists.filter(playlist => !playlist.systemKey).map(playlist => playlist.id)
    : store.value.selectedTracks.map(track => track.id)
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
    const playlistId = store.value.selectedPlaylistId
    void run(() => reorderPlaylistTracks(target, playlistId, ids))
  }
}

function exportSelected() {
  const id = store.value.selectedPlaylistId
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

onMounted(() => { void run(() => roomPlaylists.loadPlaylists()) })
function setSortList(element: Element | Record<string, unknown> | null) {
  if (element instanceof HTMLElement) sortList.value = element
}
</script>
<style scoped>
.playlist-module {
  --bg: #0b0b0b;
  --surface: #111111;
  --surface-2: #181818;
  --surface-3: #222222;
  --surface-4: #2c2c2c;
  --border: #252525;
  --border-str: #3a3a3a;
  --text: #f0f0f0;
  --text-sec: #888888;
  --text-ter: #555555;
  --accent: #71bd99;
  --accent-h: #8dcbad;
  --accent-dim: rgba(113, 189, 153, 0.14);
  --accent-txt: #000000;
  --danger: #e05252;
  --danger-bg: rgba(224, 82, 82, 0.12);
  --radius-sm: 4px;
  --radius-md: 6px;
  --radius-lg: 10px;
  --playlist-accent: #71bd99;
  display: flex;
  width: 100%;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg);
  color: var(--text);
  font-size: 13px;
}
.playlist-module button,
.playlist-module input,
.playlist-module select {
  color: inherit;
  font: inherit;
}
.playlist-module :focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
.message-line {
  flex: none;
  padding: 4px 10px;
  color: var(--danger);
  font-size: 12px;
}
.guest-state,
.empty-state {
  padding: 28px 12px;
  color: var(--text-sec);
  font-size: 13px;
  text-align: center;
}
.guest-state {
  margin: auto;
}
.d3-shell {
  width: 100%;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid var(--border);
  background: var(--surface);
}
.d3-tab-bar {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border);
  padding: 0 14px;
  background: var(--surface-2);
  flex-shrink: 0;
  gap: 0;
  overflow-x: auto;
}
.d3-tab {
  padding: 11px 14px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-sec);
  border-bottom: 2px solid transparent;
  transition: color 0.12s, border-color 0.12s;
  min-height: 42px;
  white-space: nowrap;
  background: none;
  border-top: none;
  border-left: none;
  border-right: none;
  cursor: pointer;
}
.d3-tab:hover {
  color: var(--text);
}
.d3-tab.on {
  color: var(--accent);
  border-bottom-color: var(--accent);
}
.d3-tab-sep {
  color: var(--text-ter);
  font-size: 13px;
  padding: 0 2px;
}
.d3-pane {
  display: none;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.d3-pane.show {
  display: flex;
}
.scope-toggle {
  display: flex;
  gap: 2px;
}
.scope-btn {
  min-height: 28px;
  padding: 4px 10px;
  border-radius: 4px;
  color: var(--text-sec) !important;
  font-size: 12px !important;
}
.scope-btn:hover {
  background: var(--surface-3);
  color: var(--text) !important;
}
.scope-btn.on {
  color: var(--accent) !important;
  box-shadow: inset 0 -1.5px var(--accent);
}
.d3-tab.scope-btn {
  min-height: 42px;
  padding: 11px 14px;
  border-radius: 0;
  background: none;
  box-shadow: none;
  font-size: 13px !important;
}
.d3-tab.scope-btn.on {
  box-shadow: none;
  border-bottom-color: var(--accent);
}
.text-field {
  width: 100%;
  min-width: 0;
  min-height: 28px;
  padding: 5px 8px;
  border: 1px solid var(--border-str) !important;
  border-radius: 4px;
  background: var(--surface) !important;
  font-size: 12px !important;
}
.chip-btn {
  display: inline-flex;
  min-height: 28px;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 4px;
  background: var(--surface-3) !important;
  font-size: 12px !important;
  white-space: nowrap;
}
.chip-btn:hover:not(:disabled) {
  background: var(--surface-4) !important;
}
.chip-primary {
  background: var(--accent) !important;
  color: var(--accent-txt) !important;
  font-weight: 600 !important;
}
.chip-primary:hover:not(:disabled) {
  background: var(--accent-h) !important;
}
.chip-danger {
  color: var(--danger) !important;
}
.chip-btn:disabled,
.icon-btn:disabled {
  opacity: 0.4;
}
.cover-sq {
  display: grid;
  flex: none;
  place-items: center;
  overflow: hidden;
  border-radius: 4px;
  background: var(--surface-3);
  color: var(--text-ter);
  object-fit: cover;
}
.cover-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.row-meta,
.track-meta {
  flex: 1;
  min-width: 0;
}
.row-name,
.track-name,
.track-art {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sys-badge {
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--accent-dim);
  color: var(--accent);
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
}
.row-chevron {
  color: var(--text-ter);
}
.icon-btn {
  display: grid;
  width: 32px;
  height: 32px;
  flex: none;
  place-items: center;
  border: 0;
  border-radius: 4px;
  background: transparent;
}
.icon-btn:hover:not(:disabled) {
  background: var(--surface-3);
}
.drag-handle {
  color: var(--text-ter) !important;
  cursor: grab;
  touch-action: none;
}
.accent-icon {
  color: var(--accent) !important;
}
.note-line {
  margin: 2px 0 0;
  padding: 0 10px 6px 52px;
  color: var(--text-sec);
  font-size: 11px;
  white-space: pre-wrap;
  word-break: break-word;
}
.d3-index-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.d3-index-toolbar .text-field {
  flex: 1;
  min-width: 120px;
}
.d3-playlist-list {
  overflow-y: auto;
  flex: 1;
}
.d3-playlist-row-wrap {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--border);
}
.d3-playlist-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  cursor: pointer;
  transition: background 0.1s;
  flex: 1;
  min-width: 0;
  text-align: left;
  background: none;
  border: none;
  color: inherit;
}
.d3-playlist-row:hover {
  background: var(--surface-2);
}
.d3-playlist-row.selected {
  background: var(--accent-dim);
}
.d3-playlist-cover {
  width: 44px;
  height: 44px;
  border-radius: 6px;
}
.d3-playlist-row .row-name {
  font-size: 14px;
  font-weight: 500;
}
.d3-playlist-row .row-sub {
  font-size: 12px;
  color: var(--text-sec);
}
.d3-detail-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.cover-wrap {
  position: relative;
  width: 60px;
  height: 60px;
  flex-shrink: 0;
  border-radius: 6px;
  overflow: hidden;
  background: var(--surface-3);
  display: grid;
  place-items: center;
}
.cover-wrap .cover-cam-btn {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgba(0, 0, 0, 0.65);
  opacity: 0;
  transition: opacity 0.15s;
}
.cover-wrap:hover .cover-cam-btn {
  opacity: 1;
}
.hd-body {
  flex: 1;
  min-width: 0;
}
.hd-name {
  font-size: 16px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.hd-sub {
  font-size: 12px;
  color: var(--text-sec);
  margin-top: 2px;
}
.hd-desc {
  font-size: 12px;
  color: var(--text-sec);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
}
.play-all {
  flex: none;
}
.d3-import-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  background: var(--surface-2);
}
.d3-import-bar select {
  width: 110px;
}
.d3-import-bar .import-id {
  flex: 1;
  min-width: 80px;
}
.d3-track-list {
  overflow-y: auto;
  flex: 1;
  position: relative;
}
.track-entry {
  border-bottom: 1px solid var(--border);
}
.d3-track-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 16px 9px 10px;
  transition: background 0.1s;
  cursor: pointer;
}
.d3-track-row.selected {
  background: var(--accent-dim);
}
.d3-track-num {
  width: 20px;
  text-align: right;
  font-size: 12px;
  color: var(--text-ter);
  flex-shrink: 0;
}
.d3-track-cover {
  width: 38px;
  height: 38px;
  border-radius: 4px;
}
.d3-track-row .track-name {
  font-size: 13px;
  font-weight: 500;
}
.d3-track-row .track-art {
  font-size: 12px;
  color: var(--text-sec);
}
.d3-track-row .track-note-line {
  font-size: 11px;
  color: var(--text-sec);
  margin-top: 2px;
  white-space: pre-wrap;
  word-break: break-word;
}
.d3-action-bar {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
  background: var(--surface-3);
  border-radius: 4px;
  padding: 2px;
  border: 1px solid var(--border-str);
}
.d3-action-bar .icon-btn {
  width: 28px;
  height: 28px;
  border-radius: 3px;
}
.d3-track-row:not(.selected) .d3-action-bar {
  opacity: 0;
  pointer-events: none;
}
.d3-track-row.selected .d3-action-bar {
  opacity: 1;
}
.d3-note-row {
  display: none;
  padding: 0 16px 8px 48px;
}
.d3-note-row.show {
  display: flex;
  gap: 8px;
  align-items: center;
}
.d3-note-row .text-field {
  flex: 1;
}
.d3-footer {
  flex-shrink: 0;
  padding: 8px 16px;
  border-top: 1px solid var(--border);
  background: var(--surface-2);
}
.footer-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.footer-left {
  display: flex;
  gap: 4px;
  align-items: center;
  margin-right: auto;
}
.footer-right {
  display: flex;
  gap: 4px;
}
.footer-edit {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}
.footer-edit .rename-field {
  flex: 1;
  min-width: 120px;
}
.d3-delete-confirm {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}
.d3-delete-confirm span {
  flex: 1;
  font-size: 12px;
  color: var(--text-sec);
  min-width: 120px;
}
.rename-field {
  flex: 1;
}
.export-format {
  width: 72px;
}
</style>
