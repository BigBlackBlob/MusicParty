<template>
  <section class="flex h-full min-h-0 w-full flex-col gap-3 overflow-hidden bg-black p-3 text-white [--playlist-accent:#71bd99]" :aria-label="t('roomPlaylists.title')">
    <header v-if="viewMode === 'index'" class="flex shrink-0 flex-wrap items-center justify-between gap-2">
      <div class="flex gap-2" role="group" :aria-label="t('roomPlaylists.title')">
        <button v-for="option in scopes" :key="option" type="button" class="action" :aria-pressed="scope === option" @click="switchScope(option)">
          {{ t(option === 'room' ? 'roomPlaylists.roomScope' : 'roomPlaylists.personalScope') }}
        </button>
      </div>
      <span class="text-xs text-text-secondary">{{ store.playlists.length }} {{ t('roomPlaylists.title') }}</span>
    </header>

    <header v-else class="flex shrink-0 items-center gap-2">
      <button type="button" class="action icon-action" :aria-label="t('common.back')" :title="t('common.back')" @click="goBack">
        <span class="material-symbols-outlined" aria-hidden="true">arrow_back</span>
      </button>
      <div class="group relative h-14 w-14 shrink-0 overflow-hidden rounded bg-neutral-900">
        <CoverImage v-if="coverUrl" :src="coverUrl" alt="" class="h-full w-full" />
        <span v-else class="material-symbols-outlined grid h-full place-items-center text-neutral-500" aria-hidden="true">album</span>
        <button v-if="canEdit" type="button" class="action icon-action cover-action absolute inset-0 h-full w-full rounded-none bg-black/70" :aria-label="t('roomPlaylists.uploadCover')" :title="t('roomPlaylists.uploadCover')" :disabled="busy" @click="coverInput?.click()">
          <span class="material-symbols-outlined" aria-hidden="true">photo_camera</span>
        </button>
        <input ref="coverInput" type="file" accept="image/png,image/jpeg" hidden @change="uploadCover">
      </div>
      <div class="min-w-0 flex-1">
        <h2 class="truncate text-sm font-semibold">{{ store.selectedPlaylist?.name || t('roomPlaylists.loading') }}</h2>
        <p class="text-xs text-text-secondary">{{ store.selectedPlaylist?.trackCount ?? 0 }} {{ t('queue.tracks') }}</p>
        <p v-if="store.selectedPlaylist?.description" class="line-clamp-2 text-xs text-text-secondary">{{ store.selectedPlaylist.description }}</p>
      </div>
      <button v-if="store.selectedTracks.length && hasRoom" type="button" class="action icon-action primary-action" :aria-label="t('roomPlaylists.playAll')" :title="t('roomPlaylists.playAll')" :disabled="busy || !commands.canOperate" @click="playSelected">
        <span class="material-symbols-outlined" aria-hidden="true">playlist_play</span>
      </button>
    </header>

    <p v-if="message || store.error" class="shrink-0 text-xs text-error" role="alert">{{ message || store.error }}</p>
    <p v-if="scope === 'personal' && user.isGuest" class="py-6 text-sm text-text-secondary">{{ t('roomPlaylists.personalUnavailable') }}</p>

    <template v-else-if="viewMode === 'index'">
      <form class="flex shrink-0 gap-2" @submit.prevent="create">
        <input v-model.trim="newName" maxlength="64" class="field min-w-0 flex-1" :aria-label="t('roomPlaylists.newPlaceholder')" :placeholder="t('roomPlaylists.newPlaceholder')" :disabled="busy">
        <button type="submit" class="action primary-action" :disabled="busy || !newName">{{ t('rooms.create') }}</button>
      </form>
      <div ref="sortList" class="min-h-0 flex-1 overflow-y-auto" :aria-busy="store.loading">
        <div v-for="playlist in store.playlists" :key="playlist.id" :data-reorder-id="scope === 'personal' && !playlist.systemKey ? playlist.id : undefined" class="flex items-center gap-1 border-b border-neutral-900">
          <button type="button" class="flex min-w-0 flex-1 items-center gap-2 rounded px-1.5 py-2 text-left hover:bg-neutral-900 focus-visible:outline focus-visible:outline-1 focus-visible:outline-[var(--playlist-accent)]" :disabled="busy" @click="select(playlist.id)">
            <CoverImage v-if="playlist.coverUrl" :src="playlist.coverUrl" alt="" class="h-9 w-9 shrink-0 rounded" />
            <span v-else class="material-symbols-outlined grid h-9 w-9 shrink-0 place-items-center rounded bg-neutral-900 text-neutral-500" aria-hidden="true">album</span>
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm">{{ playlist.name }}</span>
              <span class="block text-xs text-text-secondary">{{ playlist.trackCount }} {{ t('queue.tracks') }}</span>
            </span>
            <span class="material-symbols-outlined text-neutral-500" aria-hidden="true">chevron_right</span>
          </button>
          <button v-if="canSort && !playlist.systemKey" type="button" data-reorder-handle class="action icon-action drag-handle" :aria-label="t('roomPlaylists.reorder', { name: playlist.name })" :title="t('roomPlaylists.reorderHint')" :disabled="busy"
            @keydown.up.prevent="moveBy(playlist.id, -1)" @keydown.down.prevent="moveBy(playlist.id, 1)">
            <span class="material-symbols-outlined" aria-hidden="true">drag_indicator</span>
          </button>
        </div>
        <p v-if="!store.loading && !store.playlists.length" class="py-6 text-sm text-text-secondary">{{ t('roomPlaylists.empty') }}</p>
      </div>
    </template>

    <template v-else>
      <form v-if="canEdit" class="flex shrink-0 flex-wrap gap-2" @submit.prevent="importExternal">
        <select v-model="importPlatform" class="field" :aria-label="t('roomPlaylists.importPlatform')" :disabled="busy">
          <option value="netease">NetEase</option>
          <option value="qq">QQ Music</option>
        </select>
        <input v-model.trim="externalPlaylistId" class="field min-w-20 flex-1" :aria-label="t('roomPlaylists.externalPlaceholder')" :placeholder="t('roomPlaylists.externalPlaceholder')" :disabled="busy">
        <button type="submit" class="action" :disabled="busy || !externalPlaylistId">{{ t('search.importPlaylist') }}</button>
      </form>

      <div ref="sortList" class="min-h-0 flex-1 overflow-y-auto" :aria-busy="store.loading">
        <div v-for="track in store.selectedTracks" :key="track.id" :data-reorder-id="canSort ? track.id : undefined" class="border-b border-neutral-900 px-1.5 py-1.5">
          <div class="flex items-center gap-2">
            <CoverImage v-if="track.music.coverUrl" :src="track.music.coverUrl" alt="" class="h-8 w-8 shrink-0 rounded" />
            <span v-else class="material-symbols-outlined grid h-8 w-8 shrink-0 place-items-center rounded bg-neutral-900 text-neutral-500" aria-hidden="true">music_note</span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm">{{ track.music.name }}</p>
              <p class="truncate text-xs text-text-secondary">{{ track.music.artists?.join(' / ') || track.music.platform }}</p>
            </div>
            <button v-if="scope === 'personal' && hasRoom" type="button" class="action icon-action" :aria-label="t('roomPlaylists.addToQueue')" :title="t('roomPlaylists.addToQueue')" :disabled="busy || !commands.canOperate" @click="commands.enqueue(track.music.platform, track.music.id)">
              <span class="material-symbols-outlined" aria-hidden="true">add</span>
            </button>
            <button v-if="canEdit" type="button" class="action icon-action" :class="{ 'text-[var(--playlist-accent)]': track.notes }" :aria-expanded="editingTrackId === track.id" :aria-label="t('roomPlaylists.editNotes')" :title="t('roomPlaylists.editNotes')" :disabled="busy" @click="editNotes(track)">
              <span class="material-symbols-outlined" aria-hidden="true">edit_note</span>
            </button>
            <button v-if="canEdit || isLikedPlaylist" type="button" class="action icon-action" :aria-label="t('queue.remove')" :title="t('queue.remove')" :disabled="busy" @click="deleteTrack(track)">
              <span class="material-symbols-outlined" aria-hidden="true">{{ isLikedPlaylist ? 'heart_minus' : 'remove' }}</span>
            </button>
            <button v-if="canSort" type="button" data-reorder-handle class="action icon-action drag-handle" :aria-label="t('roomPlaylists.reorder', { name: track.music.name })" :title="t('roomPlaylists.reorderHint')" :disabled="busy"
              @keydown.up.prevent="moveBy(track.id, -1)" @keydown.down.prevent="moveBy(track.id, 1)">
              <span class="material-symbols-outlined" aria-hidden="true">drag_indicator</span>
            </button>
          </div>
          <input v-if="editingTrackId === track.id" v-model="trackNotes" maxlength="1000" data-playlist-note class="field mt-1 w-full" :aria-label="t('roomPlaylists.editNotes')" :placeholder="t('roomPlaylists.editNotes')"
            @blur="saveNotes(track)" @keydown.enter.prevent="saveNotes(track)" @keydown.esc.prevent="editingTrackId = null">
          <p v-else-if="track.notes" class="mt-1 whitespace-pre-wrap break-words pl-10 text-xs text-text-secondary">{{ track.notes }}</p>
        </div>
        <p v-if="!store.loading && !store.selectedTracks.length" class="py-6 text-sm text-text-secondary">{{ t('roomPlaylists.emptyTracks') }}</p>
      </div>

      <footer v-if="store.selectedPlaylist" class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t border-neutral-800 pt-2">
        <form v-if="editor === 'rename'" class="flex w-full flex-wrap gap-2" @submit.prevent="rename">
          <input ref="renameInput" v-model.trim="renameValue" class="field min-w-24 flex-1" maxlength="64" :aria-label="t('roomPlaylists.namePrompt')" @keydown.esc.prevent="editor = null">
          <button type="submit" class="action primary-action" :disabled="busy || !renameValue">{{ t('roomPlaylists.save') }}</button>
          <button type="button" class="action" :disabled="busy" @click="editor = null">{{ t('common.cancel') }}</button>
        </form>
        <div v-else-if="editor === 'delete'" class="flex w-full flex-wrap items-center gap-2">
          <span class="mr-auto text-xs" role="status">{{ t('roomPlaylists.deleteConfirm', { name: store.selectedPlaylist.name }) }}</span>
          <button type="button" class="action text-error" :disabled="busy" @click="deleteSelected">{{ t('roomPlaylists.delete') }}</button>
          <button type="button" class="action" :disabled="busy" @click="editor = null">{{ t('common.cancel') }}</button>
        </div>
        <template v-else>
          <div class="flex gap-1">
            <select v-model="exportFormat" class="field" :aria-label="t('common.export')">
              <option v-for="format in exportFormats" :key="format" :value="format">{{ format.toUpperCase() }}</option>
            </select>
            <button type="button" class="action" :disabled="busy" @click="exportSelected">{{ t('common.export') }}</button>
          </div>
          <div v-if="canEdit" class="flex gap-1">
            <button type="button" class="action" :disabled="busy" @click="openRename">{{ t('roomPlaylists.rename') }}</button>
            <button type="button" class="action" :disabled="busy" @click="editor = 'delete'">{{ t('roomPlaylists.delete') }}</button>
          </div>
        </template>
      </footer>
    </template>
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
</script>

<style scoped>
.action, .field {
  min-height: 2rem;
  border-radius: 0.25rem;
  font-size: 0.75rem;
}
.action {
  padding: 0.25rem 0.5rem;
  white-space: nowrap;
}
.action:hover:not(:disabled) { background: #222; }
.action[aria-pressed='true'] { color: var(--playlist-accent); box-shadow: inset 0 -1px var(--playlist-accent); }
.action:disabled, .field:disabled { opacity: 0.45; }
.action:focus-visible, .field:focus-visible { outline: 1px solid var(--playlist-accent); outline-offset: 1px; }
.primary-action { background: var(--playlist-accent); color: #000; font-weight: 600; }
.primary-action:hover:not(:disabled) { background: #8dcbad; }
.field { padding: 0.25rem 0.5rem; border: 1px solid #444; background: #111; }
.icon-action { display: grid; flex-shrink: 0; place-items: center; padding: 0.25rem; min-width: 1.75rem; }
.icon-action .material-symbols-outlined { font-size: 1.125rem; }
.drag-handle { cursor: grab; touch-action: none; }
.drag-handle:active { cursor: grabbing; }
@media (hover: hover) {
  .cover-action:not(:focus-visible) { opacity: 0; }
  .group:hover .cover-action { opacity: 1; }
}
</style>
