import { expect, test, type Page } from '@playwright/test'

const adminUsername = process.env.E2E_ADMIN_USERNAME || 'stage8-admin'
const adminPassword = process.env.E2E_ADMIN_PASSWORD || 'Stage8-Password-2026!'
const uniqueRoomName = () => `Invite Room-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

async function loginAsAdmin(page: Page): Promise<void> {
  await page.goto('/')
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByLabel('管理员账号').fill(adminUsername)
  await page.getByLabel('管理员密码').fill(adminPassword)
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await expect(page.getByPlaceholder('Create a room...')).toBeVisible()
}

test('a generated room invite can establish a member session through the invite entry flow', async ({ browser }) => {
  const roomName = uniqueRoomName()
  const adminContext = await browser.newContext()
  const adminPage = await adminContext.newPage()

  try {
    await loginAsAdmin(adminPage)
    await adminPage.getByPlaceholder('Create a room...').fill(roomName)
    await adminPage.getByRole('button', { name: 'Create', exact: true }).click()
    await expect(adminPage.getByRole('button', { name: new RegExp(`Current room: ${roomName}`) })).toBeVisible({ timeout: 15_000 })

    await adminPage.getByRole('button', { name: /Settings|设置/ }).click()
    const settingsNavigation = adminPage.getByRole('navigation', { name: 'Settings sections' })
    await settingsNavigation.getByRole('button', { name: /Invites|邀请码/ }).click()
    await adminPage.getByRole('button', { name: /Generate permanent invite|创建永久邀请码/ }).click()
    const inviteUrl = await adminPage.getByLabel(/Full invite link|完整邀请链接/).inputValue()
    expect(inviteUrl).toMatch(/\/join\//)

    const memberContext = await browser.newContext()
    const memberPage = await memberContext.newPage()
    try {
      await memberPage.goto(inviteUrl)
      await expect(memberPage.getByLabel('显示名')).toBeVisible()
      await memberPage.getByLabel('显示名').fill('Invited Member')
      await memberPage.getByRole('button', { name: '使用邀请码加入' }).click()
      await expect(memberPage.getByText('欢迎来到 MusicParty')).toBeHidden()
      await memberPage.getByText(roomName, { exact: true }).click()
      await expect(memberPage.getByRole('button', { name: new RegExp(`进入 ${roomName}`) })).toBeVisible({ timeout: 15_000 })
    } finally {
      await memberContext.close().catch(() => undefined)
    }
  } finally {
    await adminContext.close().catch(() => undefined)
  }
})
