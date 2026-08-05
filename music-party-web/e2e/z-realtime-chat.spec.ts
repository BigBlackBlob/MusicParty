import { expect, test, type Page } from '@playwright/test'

const uniqueMessage = (scope: string) => `${scope}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

async function enterAsGuest(page: Page, name: string): Promise<void> {
  await page.goto('/')
  await page.getByRole('button', { name: '以访客身份进入' }).click()
  await page.getByLabel('昵称').fill(name)
  await page.getByRole('button', { name: '立即进入' }).click()
  await page.getByRole('button', { name: /进入 Lounge/ }).click()
  await expect(page.getByRole('button', { name: /Chat|聊天/ })).toBeVisible({ timeout: 15_000 })
}

async function openChat(page: Page): Promise<void> {
  await page.getByRole('button', { name: /Chat|聊天/ }).click()
  await expect(page.getByPlaceholder(/Send a message|发送消息/)).toBeVisible()
}

test('room and public chat remain isolated while synchronizing between clients', async ({ browser }) => {
  const senderContext = await browser.newContext()
  const receiverContext = await browser.newContext()
  const sender = await senderContext.newPage()
  const receiver = await receiverContext.newPage()

  try {
    await enterAsGuest(sender, 'Realtime Sender')
    await enterAsGuest(receiver, 'Realtime Receiver')
    await openChat(sender)
    await openChat(receiver)

    const roomMessage = uniqueMessage('room-chat')
    await sender.getByPlaceholder(/Send a message|发送消息/).fill(roomMessage)
    await sender.getByRole('button', { name: /Send|发送/ }).click()
    await expect(receiver.getByText(roomMessage, { exact: true })).toBeVisible()

    await receiver.getByRole('button', { name: 'PUBLIC', exact: true }).click()
    await expect(receiver.getByText(roomMessage, { exact: true })).toBeHidden()

    await sender.getByRole('button', { name: 'PUBLIC', exact: true }).click()
    const publicMessage = uniqueMessage('public-chat')
    await sender.getByPlaceholder('Message public channel...').fill(publicMessage)
    await sender.getByRole('button', { name: /Send|发送/ }).click()
    await expect(receiver.getByText(publicMessage, { exact: true })).toBeVisible()
    await expect(receiver.getByText(roomMessage, { exact: true })).toBeHidden()
  } finally {
    await senderContext.close().catch(() => undefined)
    await receiverContext.close().catch(() => undefined)
  }
})
