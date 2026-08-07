import { expect, test, type Page } from '@playwright/test'

const adminUsername = process.env.E2E_ADMIN_USERNAME || 'e2e-admin'
const adminPassword = process.env.E2E_ADMIN_PASSWORD || 'E2E-Password-2026!'

const uniqueRoomName = (prefix: string) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

async function enterAsGuest(page: Page, name = 'E2E Guest') {
  await page.goto('/')
  await page.getByRole('button', { name: '以访客身份进入' }).click()
  await page.getByLabel('昵称').fill(name)
  await page.getByRole('button', { name: '立即进入' }).click()
  await expect(page.getByRole('heading', { name: '欢迎来到 MusicParty' })).toBeHidden()
  await expect(page.getByText('MUSIC PARTY', { exact: true })).toBeVisible()
}

async function loginAsAdmin(page: Page) {
  await page.goto('/')
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByLabel('管理员账号').fill(adminUsername)
  await page.getByLabel('管理员密码').fill(adminPassword)
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await expect(page.getByLabel('管理员账号')).toBeHidden()
  await expect(page.getByPlaceholder('Create a room...')).toBeVisible()
}

async function createRoom(page: Page, name: string, options?: { password?: string }) {
  await page.getByPlaceholder('Create a room...').fill(name)
  if (options?.password) {
    await page.getByRole('checkbox', { name: /Private room|私有房间/ }).check()
    await page.getByPlaceholder(/Set room password|设置房间密码/).fill(options.password)
  }
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.getByText(name, { exact: true })).toBeVisible()
}

test.describe('Go-first entry and room flow', () => {
  test('guest session is restored after a reload without a browser-readable token', async ({ page }) => {
    await enterAsGuest(page)

    const storage = await page.evaluate(() => ({ ...localStorage }))
    expect(Object.keys(storage)).not.toContain('mp_session_token')
    expect(Object.keys(storage)).not.toContain('mp_account_username')

    await page.reload()
    await expect(page.getByText('MUSIC PARTY', { exact: true })).toBeVisible()
    await expect(page.getByLabel('昵称')).toBeHidden()
  })

  test('administrator can create and enter a public room, then open room settings', async ({ page }) => {
    const roomName = uniqueRoomName('Public Room')
    await loginAsAdmin(page)
    await createRoom(page, roomName)

    await expect(page.getByRole('button', { name: new RegExp(`Current room: ${roomName}`) })).toBeVisible({ timeout: 15_000 })

    await page.getByRole('button', { name: /Settings|设置/ }).click()
    await expect(page.getByRole('navigation', { name: 'Settings sections' })).toBeVisible()
    await expect(page.getByRole('button', { name: /Current Room|当前房间/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /Invites|邀请码/ })).toHaveCount(0)
  })

  test('private room rejects a wrong password and accepts the correct password', async ({ browser }) => {
    const roomName = uniqueRoomName('Private Room')
    const password = 'Room-Password-2026!'

    const adminContext = await browser.newContext()
    const adminPage = await adminContext.newPage()
    await loginAsAdmin(adminPage)
    await createRoom(adminPage, roomName, { password })
    await adminContext.close()

    const guestContext = await browser.newContext()
    const guestPage = await guestContext.newPage()
    await enterAsGuest(guestPage, 'Private Room Guest')
    await guestPage.getByText(roomName, { exact: true }).click()

    await expect(guestPage.getByRole('dialog', { name: /加入私有房间|Join private room/ })).toBeVisible()
    await guestPage.getByLabel(/Room password|房间密码/).fill('wrong-password')
    await guestPage.getByRole('button', { name: /Verify and enter|进入房间/ }).click()
    const accessDialog = guestPage.getByRole('dialog')
    await expect(accessDialog.getByRole('alert')).toHaveText(/Invalid room password|密码/)
    await expect(accessDialog).toBeVisible()

    await guestPage.getByLabel(/Room password|房间密码/).fill(password)
    await guestPage.getByRole('button', { name: /Verify and enter|进入房间/ }).click()
    await expect(guestPage.getByRole('dialog')).toBeHidden()
    await expect(guestPage.getByRole('button', { name: `进入 ${roomName}` })).toBeVisible()
    await guestContext.close()
  })

  test('legacy browser state is removed during bootstrap', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('mp_session_token', 'legacy-token')
      localStorage.setItem('mp_account_username', 'legacy-user')
      localStorage.setItem('mp_connection_profile', JSON.stringify({ backend: 'java' }))
    })

    await enterAsGuest(page, 'Migration Guest')

    const legacyState = await page.evaluate(() => ({
      token: localStorage.getItem('mp_session_token'),
      username: localStorage.getItem('mp_account_username'),
      connectionProfile: localStorage.getItem('mp_connection_profile'),
    }))
    expect(legacyState).toEqual({ token: null, username: null, connectionProfile: null })
  })
})

test.describe('mobile flow', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test('administrator can reach room settings without horizontal overflow', async ({ page }) => {
    await loginAsAdmin(page)
    await page.getByRole('button', { name: /进入 Lounge/ }).click()
    await expect(page.getByRole('button', { name: /Settings|设置/ })).toBeVisible({ timeout: 15_000 })
    await page.getByRole('button', { name: /Settings|设置/ }).click()

    const settings = page.getByRole('navigation', { name: 'Settings sections' })
    await expect(settings).toBeVisible()
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
    expect(overflow).toBeLessThanOrEqual(1)
  })
})
