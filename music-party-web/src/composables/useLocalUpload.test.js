import { beforeEach, describe, expect, it, vi } from 'vitest'
import { httpClient } from '../transport/httpClient'
import { readAudioTagsFromFile } from '../utils/id3Tags'
import { useLocalUpload } from './useLocalUpload'

vi.mock('../transport/httpClient', () => ({ httpClient: { post: vi.fn() } }))
vi.mock('../utils/id3Tags', () => ({ readAudioTagsFromFile: vi.fn() }))

beforeEach(() => {
  vi.resetAllMocks()
  readAudioTagsFromFile.mockResolvedValue({})
  httpClient.post.mockResolvedValue({})
})

const audioFile = name => new File(['audio'], name, { type: 'audio/mpeg' })

function uploadedMetadata() {
  return httpClient.post.mock.calls.map(([url, body]) => [url, body.get('file').name, body.get('title'), body.get('artists'), body.get('album')])
}

describe('local upload queue', () => {
  it('sends separate tags, fallbacks and edits for each selected file', async () => {
    const first = audioFile('first.mp3')
    const second = audioFile('second.demo.mp3')
    const removed = audioFile('removed.mp3')
    const { queue, addFiles, readTags, removeFile, upload } = useLocalUpload()
    readAudioTagsFromFile.mockImplementation(async file => file === first
      ? { title: 'First song', artists: 'Alice', album: 'First album' }
      : {})

    await addFiles([first, second, removed], 'Unknown')
    removeFile(queue.value[2].id)
    await readTags()
    expect(readAudioTagsFromFile.mock.calls.map(([file]) => file)).toEqual([first, second, removed, first, second])
    expect(queue.value[1]).toMatchObject({ title: 'second.demo', artists: 'Unknown', album: '' })
    queue.value[1].title = 'Second song'

    expect(await upload('Upload failed')).toBe(2)
    expect(uploadedMetadata()).toEqual([
      ['/api/local/tracks/upload', 'first.mp3', 'First song', 'Alice', 'First album'],
      ['/api/local/tracks/upload', 'second.demo.mp3', 'Second song', 'Unknown', null]
    ])
    expect(queue.value).toEqual([])
  })

  it('limits concurrent requests to two and retries only the failed file', async () => {
    const firstRequest = Promise.withResolvers()
    const secondRequest = Promise.withResolvers()
    const thirdRequest = Promise.withResolvers()
    httpClient.post.mockImplementationOnce((_url, _body, options) => {
      options.onUploadProgress({ loaded: 40, total: 100 })
      return firstRequest.promise
    }).mockReturnValueOnce(secondRequest.promise).mockReturnValueOnce(thirdRequest.promise)
    const { queue, addFiles, upload } = useLocalUpload()
    await addFiles(['one.mp3', 'two.mp3', 'three.mp3'].map(audioFile), 'Unknown')

    const uploading = upload('Upload failed')
    expect(httpClient.post).toHaveBeenCalledTimes(2)
    expect(queue.value.map(entry => entry.status)).toEqual(['uploading', 'uploading', 'queued'])
    expect(queue.value[0].progress).toBe(40)

    firstRequest.resolve({})
    await vi.waitFor(() => expect(httpClient.post).toHaveBeenCalledTimes(3))
    expect(queue.value[0]).toMatchObject({ status: 'done', progress: 100 })
    secondRequest.reject(new Error('Upload capacity reached'))
    thirdRequest.resolve({})
    expect(await uploading).toBe(2)
    expect(queue.value).toEqual([expect.objectContaining({ title: 'two', status: 'failed', error: 'Upload capacity reached' })])

    expect(await upload('Upload failed')).toBe(1)
    expect(uploadedMetadata().map(([, file]) => file)).toEqual(['one.mp3', 'two.mp3', 'three.mp3', 'two.mp3'])
    expect(queue.value).toEqual([])
  })
})
