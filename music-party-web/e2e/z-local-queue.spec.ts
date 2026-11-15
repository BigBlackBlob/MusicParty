import { spawn } from 'node:child_process'
import { mkdir, readFile } from 'node:fs/promises'
import { join } from 'node:path'
import { chromium, expect, test, type Browser, type Page } from '@playwright/test'
import type { ClientEnvelope, ServerEnvelope } from '../src/contracts/generated/websocket'
import { decodeServerEnvelope } from '../src/transport/realtimeDecoder'

const adminUsername = process.env.E2E_ADMIN_USERNAME || 'e2e-admin'
const adminPassword = process.env.E2E_ADMIN_PASSWORD || 'E2E-Password-2026!'

test.setTimeout(180_000)

async function loginAsAdmin(page: Page, url = '/'): Promise<void> {
  await page.goto(url)
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByLabel('管理员账号').fill(adminUsername)
  await page.getByLabel('管理员密码').fill(adminPassword)
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await expect(page.getByPlaceholder('Create a room...')).toBeVisible()
}

// A normal Playwright context forces focus on every page. Attach to our own
// disposable browser without those overrides to exercise native tab visibility.
async function openDesktopBrowser(profilePath: string) {
  await mkdir(profilePath, { recursive: true })
  const browserProcess = spawn(chromium.executablePath(), [
    '--remote-debugging-port=0', `--user-data-dir=${profilePath}`,
    '--no-first-run', '--no-default-browser-check', 'about:blank',
  ], { windowsHide: true, stdio: 'ignore' })
  try {
    let port = ''
    await expect.poll(async () => {
      port = await readFile(join(profilePath, 'DevToolsActivePort'), 'utf8')
        .then(value => value.split('\n')[0] ?? '').catch(() => '')
      return port
    }, { timeout: 10_000 }).toMatch(/^\d+$/)
    const browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`, { noDefaults: true })
    return { browser, close: async () => { try { await browser.close() } finally { browserProcess.kill() } } }
  } catch (error) {
    browserProcess.kill()
    throw error
  }
}

async function createRoom(page: Page, name: string): Promise<void> {
  await page.getByPlaceholder('Create a room...').fill(name)
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.getByText(name, { exact: true })).toBeVisible()
  await page.getByText(name, { exact: true }).click()
  const searchButton = page.getByRole('button', { name: /Search and add|搜索并添加/ })
  if (!await searchButton.isVisible().catch(() => false)) {
    await page.getByRole('button', { name: new RegExp(`(Enter|进入) ${name}`) }).click()
  }
  await expect(searchButton).toBeVisible({ timeout: 15_000 })
}

async function loginAndEnterLounge(page: Page): Promise<void> {
  await page.goto('/')
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByLabel('管理员账号').fill(adminUsername)
  await page.getByLabel('管理员密码').fill(adminPassword)
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByRole('button', { name: /Enter Lounge|进入 Lounge/ }).click()
  await expect(page.getByRole('button', { name: /Search and add|搜索并添加/ })).toBeVisible({ timeout: 15_000 })
}

async function uploadFixture(page: Page, title: string): Promise<void> {
  await page.evaluate(async ({ title }) => {
    const sampleRate = 8_000
    const samples = sampleRate * 30
    const bytes = new ArrayBuffer(44 + samples * 2)
    const view = new DataView(bytes)
    const write = (offset: number, value: string) => [...value].forEach((character, index) => view.setUint8(offset + index, character.charCodeAt(0)))
    write(0, 'RIFF'); view.setUint32(4, 36 + samples * 2, true); write(8, 'WAVEfmt ')
    view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true)
    view.setUint32(24, sampleRate, true); view.setUint32(28, sampleRate * 2, true); view.setUint16(32, 2, true); view.setUint16(34, 16, true)
    write(36, 'data'); view.setUint32(40, samples * 2, true)
    for (let sample = 0; sample < samples; sample += 1) {
      // Preserve the title in the PCM bytes so deduplication cannot merge fixtures.
      const identity = sample < title.length ? title.charCodeAt(sample) : 0
      view.setInt16(44 + sample * 2, (sample % 128 < 64 ? 4_000 : -4_000) + identity, true)
    }
    const form = new FormData()
    form.set('file', new File([bytes], `${title}.wav`, { type: 'audio/wav' }))
    form.set('title', title)
    form.set('artists', 'E2E')
    const csrf = document.cookie.split('; ').find(value => value.startsWith('MP_CSRF='))?.slice('MP_CSRF='.length)
    const response = await fetch('/api/local/tracks/upload', { method: 'POST', headers: csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {}, body: form })
    if (!response.ok) throw new Error(`upload failed: ${response.status}`)
  }, { title })
  await expect.poll(async () => page.evaluate(async ({ title }) => {
    const response = await fetch('/api/local/tracks')
    const tracks = await response.json() as Array<{ title: string; status: string }>
    return tracks.find(track => track.title === title)?.status ?? ''
  }, { title }), { timeout: 120_000 }).toBe('COMPLETED')
}

async function enqueueLocalTrack(page: Page, title: string): Promise<void> {
  const dialog = page.getByRole('dialog')
  if (!await dialog.isVisible().catch(() => false)) {
    await page.getByRole('button', { name: /Search and add|搜索并添加/ }).click()
  }

  const localPlatform = dialog.getByRole('button', { name: /^local$/i })
  await expect(localPlatform).toBeVisible()
  await localPlatform.click()
  const search = dialog.getByPlaceholder(/Search for a track|搜索歌曲/)
  await search.fill(title)
  await search.press('Enter')
  await expect(dialog.getByText(title, { exact: true })).toBeVisible({ timeout: 15_000 })
  await dialog.getByLabel(/Add song|添加歌曲/).click()
}

async function expectQueueOrder(page: Page, titles: string[]): Promise<void> {
  await expect.poll(
    () => page.locator('[data-queue-id]').evaluateAll(
      (nodes, expectedTitles) => nodes.map(node => expectedTitles.find(title => node.textContent?.includes(title)) ?? ''),
      titles,
    ),
    { timeout: 15_000 },
  ).toEqual(titles)
}

async function openObserver(browser: Browser): Promise<{ context: Awaited<ReturnType<Browser['newContext']>>; page: Page }> {
  const context = await browser.newContext()
  const page = await context.newPage()
  await loginAndEnterLounge(page)
  return { context, page }
}

test('local tracks use the Go search, queue priority reorder, multi-client sync, and reconnect paths', async ({ browser, page }) => {
  const seed = Date.now()
  const first = `Queue Fixture A ${seed}`
  const second = `Queue Fixture B ${seed}`
  const third = `Queue Fixture C ${seed}`
  const fourth = `Queue Fixture D ${seed}`
  await loginAndEnterLounge(page)
  await uploadFixture(page, first)
  await uploadFixture(page, second)
  await uploadFixture(page, third)
  await uploadFixture(page, fourth)

  await enqueueLocalTrack(page, first)
  await expect(page.locator('.track-title-marquee').getByText(first, { exact: true }).first()).toBeVisible({ timeout: 15_000 })
  await enqueueLocalTrack(page, second)
  await enqueueLocalTrack(page, third)
  await expectQueueOrder(page, [second, third])
  await page.getByRole('dialog').getByRole('button', { name: /Close|关闭/ }).click()
  await expect(page.getByRole('dialog')).toBeHidden()

  const observer = await openObserver(browser)
  try {
    await expectQueueOrder(observer.page, [second, third])

    const thirdItem = page.locator('[data-queue-id]').filter({ hasText: third }).first()
    await thirdItem.hover()
    await thirdItem.getByRole('button', { name: /Play next|下一首播放/ }).click()
    await expectQueueOrder(page, [third, second])
    await expectQueueOrder(observer.page, [third, second])

    await page.context().setOffline(true)
    await expect.poll(() => page.evaluate(() => navigator.onLine)).toBe(false)
    await enqueueLocalTrack(observer.page, fourth)
    await expectQueueOrder(observer.page, [third, second, fourth])

    await page.context().setOffline(false)
    await expect.poll(() => page.evaluate(() => navigator.onLine)).toBe(true)
    await expectQueueOrder(page, [third, second, fourth])
  } finally {
    await observer.context.close().catch(() => undefined)
  }
})

test('playback like settles through the server and remains after reload', async ({ page }) => {
  const sent: Array<Extract<ClientEnvelope, { type: 'control.like' | 'control.unlike' }>> = []
  const received: ServerEnvelope[] = []
  page.on('websocket', socket => {
    socket.on('framesent', frame => {
      const envelope = JSON.parse(frame.payload.toString()) as ClientEnvelope
      if (envelope.type === 'control.like' || envelope.type === 'control.unlike') sent.push(envelope)
    })
    socket.on('framereceived', frame => {
      const envelope = decodeServerEnvelope(frame.payload.toString())
      if (envelope) received.push(envelope)
    })
  })
  async function expectControlAck(type: 'control.like' | 'control.unlike', index: number): Promise<void> {
    await expect.poll(() => sent.filter(frame => frame.type === type).length).toBe(index + 1)
    const command = sent.filter(frame => frame.type === type)[index]!
    const mutationId = command.payload.mutationId
    expect(mutationId).toBeTruthy()
    await expect.poll(() => received.find(frame => frame.type === 'control.ack'
      && frame.payload.mutationId === mutationId)).toMatchObject({
      payload: { mutationId, outcome: 'applied' },
    })
  }
  const title = `Like Fixture ${Date.now()}`
  const roomName = `Like Room ${Date.now()}`
  await loginAsAdmin(page)
  await createRoom(page, roomName)
  await uploadFixture(page, title)
  await enqueueLocalTrack(page, title)

  await page.getByRole('dialog').getByRole('button', { name: /Close|关闭/ }).click()
  await expect(page.locator('.track-title-marquee').getByText(title, { exact: true }).first()).toBeVisible({ timeout: 15_000 })
  const heart = page.locator('.player-console button[aria-label="Like"], .player-console button[aria-label="Remove from liked"], .player-console button[aria-label="喜欢"], .player-console button[aria-label="取消喜欢"]')
  await expect(heart).toBeVisible({ timeout: 15_000 })
  await expect(heart).toHaveAttribute('aria-label', /^(Like|喜欢)$/)
  await heart.click()
  await expectControlAck('control.like', 0)
  await expect(heart).toHaveAttribute('aria-label', /^(Remove from liked|取消喜欢)$/, { timeout: 5_000 })
  // A second command proves the first ACK released the pending mutation guard.
  await heart.click()
  await expectControlAck('control.unlike', 0)
  await expect(heart).toHaveAttribute('aria-label', /^(Like|喜欢)$/)
  await heart.click()
  await expectControlAck('control.like', 1)
  await expect.poll(async () => page.evaluate(async expectedTitle => {
    const response = await fetch('/api/me/liked-songs')
    if (!response.ok) return false
    const songs = await response.json() as Array<{ music: { name?: string } }>
    return songs.some(song => song.music?.name === expectedTitle)
  }, title)).toBe(true)

  await page.reload()
  await page.getByRole('button', { name: new RegExp(`(Enter|进入) ${roomName}`) }).click()
  await expect(page.getByRole('button', { name: /Settings|设置/ })).toBeVisible({ timeout: 15_000 })
  const restoredHeart = page.getByRole('button', { name: /^(Remove from liked|取消喜欢)$/ })
  await expect(restoredHeart).toBeVisible({ timeout: 15_000 })
})

test('desktop playback clock advances in a hidden tab and lyrics catch up on return', async ({ headless, baseURL }, testInfo) => {
  test.skip(process.platform !== 'win32' || headless, 'Real tab visibility acceptance runs on Windows with --headed.')
  const desktop = await openDesktopBrowser(testInfo.outputPath('browser-profile'))
  const context = desktop.browser.contexts()[0]!
  try {
    const page = await context.newPage()
    const title = `Background Fixture ${Date.now()}`
    const roomName = `Background Room ${Date.now()}`
    await page.route('**/api/music/lyric-detail/local/**', route => route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ lyric: '[00:00.00]First line\n[00:01.00]Second line\n[00:02.00]Third line\n[00:03.00]Fourth line\n[00:04.00]Fifth line\n[00:05.00]Sixth line' }),
    }))
    await loginAsAdmin(page, baseURL)
    await createRoom(page, roomName)
    await uploadFixture(page, title)
    await enqueueLocalTrack(page, title)
    await page.getByRole('dialog').getByRole('button', { name: /Close|关闭/ }).click()

    const audio = page.locator('audio').first()
    await expect.poll(() => audio.evaluate(element => (element as HTMLAudioElement).paused)).toBe(false)
    await expect(page.locator('.lyrics-line--active')).toBeVisible({ timeout: 15_000 })
    const before = await audio.evaluate(element => (element as HTMLAudioElement).currentTime)
    const foregroundPage = await context.newPage()
    await foregroundPage.bringToFront()
    await expect.poll(() => page.evaluate(() => document.hidden)).toBe(true)
    const hiddenVisibility = await page.evaluate(() => document.visibilityState)
    await page.waitForTimeout(2_000)
    let hiddenTime = before
    await expect.poll(async () => {
      hiddenTime = await audio.evaluate(element => (element as HTMLAudioElement).currentTime)
      return hiddenTime
    }).toBeGreaterThan(before + 1.5)
    await page.bringToFront()
    await expect.poll(() => page.evaluate(() => document.hidden)).toBe(false)
    await expect(page.locator('.lyrics-line--active')).toContainText(/Second|Third|Fourth|Fifth|Sixth/)
    await testInfo.attach('desktop-playback-observation', {
      contentType: 'application/json',
      body: JSON.stringify({ hiddenVisibility, before, hiddenTime, elapsed: hiddenTime - before,
        restoredVisibility: await page.evaluate(() => document.visibilityState),
        activeLyric: await page.locator('.lyrics-line--active').textContent() }),
    })
    await foregroundPage.close()
  } finally {
    await desktop.close()
  }
})

test('local standby survives queue consumption and becomes the active element', async ({ page }) => {
  const roomName = `Standby Room ${Date.now()}`
  const first = `Standby A ${Date.now()}`
  const second = `Standby B ${Date.now()}`
  await loginAsAdmin(page)
  await createRoom(page, roomName)
  await uploadFixture(page, first)
  await uploadFixture(page, second)
  await enqueueLocalTrack(page, first)
  await enqueueLocalTrack(page, second)
  await page.getByRole('dialog').getByRole('button', { name: /Close|关闭/ }).click()
  const elements = page.locator('audio')
  await expect.poll(() => elements.nth(1).evaluate(element => (element as HTMLAudioElement).readyState)).toBe(4)
  await expect.poll(() => elements.nth(0).evaluate(element => (element as HTMLAudioElement).paused)).toBe(false)
  // Use a real control with the preload already buffered. The consumed queue
  // patch arrives before player.state and must not replace that source.
  await page.locator('.player-console').getByRole('button', { name: /^(Next|下一首)$/ }).click()
  await expect.poll(() => elements.nth(1).evaluate(element => (element as HTMLAudioElement).paused)).toBe(false)
  await expect.poll(() => elements.nth(0).evaluate(element => (element as HTMLAudioElement).paused)).toBe(true)
  await expect(page.locator('.track-title-marquee').getByText(second, { exact: true }).first()).toBeVisible()
})
