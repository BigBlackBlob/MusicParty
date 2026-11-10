import { expect, test, type Browser, type Page } from '@playwright/test'

const uniqueMessage = (scope: string) => `${scope}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

// The default desktop layout does not place the chat module, so seed a layout
// that does. This keeps the spec about realtime chat, not layout defaults.
const CHAT_LAYOUT = JSON.stringify({
  version: 3,
  columnCount: 3,
  columns: [
    { id: 'col-0', order: 0, widthPercent: 28, modules: ['nowplaying'] },
    { id: 'col-1', order: 1, widthPercent: 44, modules: ['chat'] },
    { id: 'col-2', order: 2, widthPercent: 28, modules: ['queue'] },
  ],
})

const roomPlaceholder = /Send a message|发送消息/
const publicPlaceholder = /Message public channel|发送到公频/
const publicTab = /^(Public|公频)$/
const sendButton = /^(Send|发送)$/

async function newChatPage(browser: Browser): Promise<Page> {
  const context = await browser.newContext()
  await context.addInitScript((layout) => {
    localStorage.setItem('mp-layout-v1', layout)
  }, CHAT_LAYOUT)
  return context.newPage()
}

async function enterAsGuest(page: Page, name: string): Promise<void> {
  await page.goto('/')
  await page.getByRole('button', { name: '以访客身份进入' }).click()
  await page.getByLabel('昵称').fill(name)
  await page.getByRole('button', { name: '立即进入' }).click()
  await page.getByRole('button', { name: /Enter Lounge|进入 Lounge/ }).click()
  await expect(page.getByPlaceholder(roomPlaceholder)).toBeVisible({ timeout: 15_000 })
}

test('room and public chat remain isolated while synchronizing between clients', async ({ browser }) => {
  const sender = await newChatPage(browser)
  const receiver = await newChatPage(browser)

  try {
    await enterAsGuest(sender, 'Realtime Sender')
    await enterAsGuest(receiver, 'Realtime Receiver')

    const roomMessage = uniqueMessage('room-chat')
    await sender.getByPlaceholder(roomPlaceholder).fill(roomMessage)
    await sender.getByRole('button', { name: sendButton }).click()
    await expect(receiver.getByText(roomMessage, { exact: true })).toBeVisible()

    await receiver.getByRole('button', { name: publicTab }).click()
    await expect(receiver.getByText(roomMessage, { exact: true })).toBeHidden()

    await sender.getByRole('button', { name: publicTab }).click()
    const publicMessage = uniqueMessage('public-chat')
    await sender.getByPlaceholder(publicPlaceholder).fill(publicMessage)
    await sender.getByRole('button', { name: sendButton }).click()
    await expect(receiver.getByText(publicMessage, { exact: true })).toBeVisible()
    await expect(receiver.getByText(roomMessage, { exact: true })).toBeHidden()
  } finally {
    await sender.context().close().catch(() => undefined)
    await receiver.context().close().catch(() => undefined)
  }
})
