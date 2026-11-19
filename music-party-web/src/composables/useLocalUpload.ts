import { markRaw, reactive, ref } from 'vue'
import { authApi } from '../api/auth'
import { extractErrorMessage } from '../utils/errors'
import { readAudioTagsFromFile } from '../utils/id3Tags'

interface LocalUploadEntry {
  id: number
  file: File
  title: string
  artists: string
  album: string
  status: 'queued' | 'uploading' | 'done' | 'failed'
  progress: number
  error: string
  readingTags: boolean
  tagReadFailed: boolean
}

async function forEachWithLimit<T>(items: T[], limit: number, work: (item: T) => Promise<void>) {
  const pending = items.values()
  await Promise.all(Array.from({ length: Math.min(items.length, limit) }, async () => {
    for (const item of pending) await work(item)
  }))
}

/** Keeps tag reads and uploads bounded while each file owns its metadata and status. */
export function useLocalUpload() {
  const queue = ref<LocalUploadEntry[]>([])
  const readingTags = ref(false)
  let nextId = 0

  async function readTags(entries = queue.value) {
    if (entries.length === 0) return
    readingTags.value = true
    for (const entry of entries) {
      entry.readingTags = true
      entry.tagReadFailed = false
    }
    try {
      await forEachWithLimit(entries, 4, async entry => {
        try {
          const tags: Partial<Pick<LocalUploadEntry, 'title' | 'artists' | 'album'>> = await readAudioTagsFromFile(entry.file)
          entry.title = tags.title || entry.title
          entry.artists = tags.artists || entry.artists
          entry.album = tags.album || entry.album
        } catch {
          entry.tagReadFailed = true
        } finally {
          entry.readingTags = false
        }
      })
    } finally {
      readingTags.value = false
    }
  }

  async function addFiles(files: File[], unknownArtist: string) {
    const entries = files.map(file => reactive<LocalUploadEntry>({
      id: nextId++,
      file: markRaw(file),
      title: file.name.replace(/\.[^.]+$/, '') || file.name,
      artists: unknownArtist,
      album: '',
      status: 'queued',
      progress: 0,
      error: '',
      readingTags: false,
      tagReadFailed: false
    }))
    queue.value.push(...entries)
    await readTags(entries)
  }

  function removeFile(id: number) {
    queue.value = queue.value.filter(entry => entry.id !== id)
  }

  async function upload(fallbackError: string) {
    const pending = queue.value.filter(entry => entry.status === 'queued' || entry.status === 'failed')
    let uploaded = 0
    await forEachWithLimit(pending, 2, async entry => {
      const { file, title, artists, album } = entry
      entry.status = 'uploading'
      entry.progress = 0
      entry.error = ''
      try {
        await authApi.uploadLocalTrack(file, { title, artists, album }, percent => {
          entry.progress = percent
        })
        entry.progress = 100
        entry.status = 'done'
        uploaded++
      } catch (cause) {
        entry.status = 'failed'
        entry.error = extractErrorMessage(cause, fallbackError)
      }
    })
    queue.value = queue.value.filter(entry => entry.status !== 'done')
    return uploaded
  }

  return { queue, readingTags, addFiles, readTags, removeFile, upload }
}
