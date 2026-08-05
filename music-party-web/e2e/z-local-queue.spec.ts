import { expect, test, type Browser, type Page } from '@playwright/test'

const adminUsername = process.env.E2E_ADMIN_USERNAME || 'e2e-admin'
const adminPassword = process.env.E2E_ADMIN_PASSWORD || 'E2E-Password-2026!'

test.setTimeout(60_000)

async function loginAndEnterLounge(page: Page): Promise<void> {
  await page.goto('/')
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByLabel('管理员账号').fill(adminUsername)
  await page.getByLabel('管理员密码').fill(adminPassword)
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByRole('button', { name: /进入 Lounge/ }).click()
  await expect(page.getByRole('button', { name: /Search and add|搜索并添加/ })).toBeVisible({ timeout: 15_000 })
}

async function uploadFixture(page: Page, title: string): Promise<void> {
  await page.evaluate(async ({ title }) => {
    const sampleRate = 8_000
    const samples = sampleRate * 60
    const bytes = new ArrayBuffer(44 + samples * 2)
    const view = new DataView(bytes)
    const write = (offset: number, value: string) => [...value].forEach((character, index) => view.setUint8(offset + index, character.charCodeAt(0)))
    write(0, 'RIFF'); view.setUint32(4, 36 + samples * 2, true); write(8, 'WAVEfmt ')
    view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true)
    view.setUint32(24, sampleRate, true); view.setUint32(28, sampleRate * 2, true); view.setUint16(32, 2, true); view.setUint16(34, 16, true)
    write(36, 'data'); view.setUint32(40, samples * 2, true)
    const seed = [...title].reduce((total, character) => total + character.charCodeAt(0), 0)
    for (let sample = 0; sample < samples; sample += 1) {
      view.setInt16(44 + sample * 2, (sample + seed) % 128 < 64 ? 4_000 : -4_000, true)
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
  }, { title }), { timeout: 30_000 }).toBe('COMPLETED')
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
