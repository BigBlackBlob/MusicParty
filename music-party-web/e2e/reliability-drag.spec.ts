import { expect, test, type Browser, type Page, type WebSocketRoute } from '@playwright/test'
import { createServer, connect, type Socket } from 'node:net'
import { appendFile } from 'node:fs/promises'
import type { ClientEnvelope, ServerEnvelope } from '../src/contracts/generated/websocket'
import { decodeServerEnvelope } from '../src/transport/realtimeDecoder'

// Reliability probe: real mouse drags on the desktop queue, measured against
// the actor's DOM and store, the observer, and a freshly joined client.
const adminUsername = process.env.E2E_ADMIN_USERNAME || 'e2e-admin'
const adminPassword = process.env.E2E_ADMIN_PASSWORD || 'E2E-Password-2026!'
const repeats = Number(process.env.DRAG_REPEATS || 10)
const sizes = (process.env.DRAG_SIZES || '3,10,30').split(',').map(Number)
const clientCounts = (process.env.DRAG_CLIENT_COUNTS || '2,3,5').split(',').map(Number)
const step = (label: string) => {
  const line = `[${new Date().toISOString().slice(11, 23)}] ${label}`
  console.log(line)
  if (process.env.RELIABILITY_PROGRESS_FILE) void appendFile(process.env.RELIABILITY_PROGRESS_FILE, `${line}\n`)
}

test.setTimeout(Math.max(900_000, clientCounts.length * sizes.length * repeats * 3 * 3_000 + 180_000))
// Opt-in: run with RELIABILITY_PROBE=1 against an isolated server.
test.skip(!process.env.RELIABILITY_PROBE, 'reliability probe is opt-in')

type Wire = { sent: ClientEnvelope[]; received: Array<{ at: number; envelope: ServerEnvelope }>; sentBytes: number; receivedBytes: number }

function createTcpFaultProxy(targetHost: string, targetPort: number) {
  type Link = { client: Socket; upstream: Socket }
  let mode: 'pass' | 'drop-server-to-client' = 'pass'
  let droppedServerBytes = 0
  const links = new Set<Link>()
  const server = createServer(client => {
    const upstream = connect(targetPort, targetHost)
    const link = { client, upstream }
    let handshakeResponse = Buffer.alloc(0)
    let handshakeForwarded = false
    links.add(link)
    client.on('data', chunk => { if (!upstream.destroyed) upstream.write(chunk) })
    upstream.on('data', chunk => {
      if (client.destroyed) return
      if (!handshakeForwarded) {
        handshakeResponse = Buffer.concat([handshakeResponse, chunk])
        const headersEnd = handshakeResponse.indexOf('\r\n\r\n')
        if (headersEnd < 0) return
        const bodyStart = headersEnd + 4
        client.write(handshakeResponse.subarray(0, bodyStart))
        handshakeForwarded = true
        const payload = handshakeResponse.subarray(bodyStart)
        handshakeResponse = Buffer.alloc(0)
        if (mode === 'drop-server-to-client') droppedServerBytes += payload.length
        else if (payload.length > 0) client.write(payload)
        return
      }
      if (mode === 'drop-server-to-client') droppedServerBytes += chunk.length
      else client.write(chunk)
    })
    const close = () => {
      links.delete(link)
      client.destroy()
      upstream.destroy()
    }
    client.on('error', close)
    upstream.on('error', close)
    client.on('close', close)
    upstream.on('close', close)
  })

  return {
    async listen(): Promise<number> {
      await new Promise<void>((resolve, reject) => {
        server.once('error', reject)
        server.listen(0, '127.0.0.1', () => { server.removeListener('error', reject); resolve() })
      })
      const address = server.address()
      if (!address || typeof address === 'string') throw new Error('TCP proxy failed to bind')
      return address.port
    },
    setMode(next: 'pass' | 'drop-server-to-client') { mode = next },
    snapshot() { return { mode, droppedServerBytes, connections: links.size } },
    async close(): Promise<void> {
      for (const link of links) { link.client.destroy(); link.upstream.destroy() }
      await new Promise<void>(resolve => server.close(() => resolve()))
    }
  }
}

function recordWire(page: Page): Wire {
  const wire: Wire = { sent: [], received: [], sentBytes: 0, receivedBytes: 0 }
  page.on('websocket', socket => {
    socket.on('framesent', frame => {
      const payload = frame.payload.toString()
      wire.sentBytes += Buffer.byteLength(payload)
      try { wire.sent.push(JSON.parse(payload) as ClientEnvelope) } catch { /* binary */ }
    })
    socket.on('framereceived', frame => {
      const payload = frame.payload.toString()
      wire.receivedBytes += Buffer.byteLength(payload)
      const envelope = decodeServerEnvelope(payload)
      if (envelope) wire.received.push({ at: Date.now(), envelope })
    })
  })
  return wire
}

async function login(page: Page): Promise<void> {
  await page.goto('/')
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await page.getByLabel('管理员账号').fill(adminUsername)
  await page.getByLabel('管理员密码').fill(adminPassword)
  await page.getByRole('button', { name: '平台管理员登录' }).click()
  await expect(page.getByPlaceholder('Create a room...')).toBeVisible()
}

async function enterRoom(page: Page, name: string, create = false): Promise<void> {
  if (create) {
    await page.getByPlaceholder('Create a room...').fill(name)
    await page.getByRole('button', { name: 'Create', exact: true }).click()
  }
  await expect(page.getByText(name, { exact: true })).toBeVisible()
  await page.getByText(name, { exact: true }).click()
  const searchButton = page.getByRole('button', { name: /Search and add|搜索并添加/ })
  if (!await searchButton.isVisible().catch(() => false)) {
    await page.getByRole('button', { name: new RegExp(`进入 ${name}`) }).click()
  }
  await expect(searchButton).toBeVisible({ timeout: 15_000 })
  await expect.poll(() => storeQueue(page).then(() => true).catch(() => false)).toBe(true)
}

async function uploadFixtures(page: Page, titles: string[]): Promise<Map<string, string>> {
  await page.evaluate(async titles => {
    const csrf = document.cookie.split('; ').find(value => value.startsWith('MP_CSRF='))?.slice('MP_CSRF='.length)
    for (const title of titles) {
      const sampleRate = 8_000
      const samples = sampleRate * 60
      const bytes = new ArrayBuffer(44 + samples * 2)
      const view = new DataView(bytes)
      const write = (offset: number, value: string) => [...value].forEach((c, i) => view.setUint8(offset + i, c.charCodeAt(0)))
      write(0, 'RIFF'); view.setUint32(4, 36 + samples * 2, true); write(8, 'WAVEfmt ')
      view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true)
      view.setUint32(24, sampleRate, true); view.setUint32(28, sampleRate * 2, true); view.setUint16(32, 2, true); view.setUint16(34, 16, true)
      write(36, 'data'); view.setUint32(40, samples * 2, true)
      for (let s = 0; s < samples; s += 1) view.setInt16(44 + s * 2, (s % 128 < 64 ? 4_000 : -4_000) + (s < title.length ? title.charCodeAt(s) : 0), true)
      const form = new FormData()
      form.set('file', new File([bytes], `${title}.wav`, { type: 'audio/wav' }))
      form.set('title', title)
      form.set('artists', 'E2E')
      // The transcode queue is bounded and answers 503 while full; wait and retry.
      for (let attempt = 0; ; attempt += 1) {
        const response = await fetch('/api/local/tracks/upload', { method: 'POST', headers: csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {}, body: form })
        if (response.ok) break
        if (response.status !== 503 || attempt > 120) throw new Error(`upload failed: ${response.status}`)
        await new Promise(resolve => setTimeout(resolve, 500))
      }
    }
  }, titles)
  const ids = new Map<string, string>()
  await expect.poll(async () => {
    const tracks = await page.evaluate(async () => (await fetch('/api/local/tracks')).json()) as Array<{ id: string; title: string; status: string }>
    for (const track of tracks) if (track.status === 'COMPLETED' && titles.includes(track.title)) ids.set(track.title, track.id)
    return ids.size
  }, { timeout: 180_000 }).toBe(titles.length)
  return ids
}

// Pinia is reachable from the mounted app; the probe reads it, it never writes it.
async function storeQueue(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { queue: Array<{ music: { name: string } }> }> } } } } }).__vue_app__
    const runtime = app.config.globalProperties.$pinia._s.get('room-runtime')
    if (!runtime) throw new Error('runtime store missing')
    return runtime.queue.map(item => item.music.name)
  })
}

async function enqueueById(page: Page, trackId: string): Promise<void> {
  const sent = await page.evaluate(id => {
    const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { enqueue: (p: string, id: string) => boolean }> } } } } }).__vue_app__
    return app.config.globalProperties.$pinia._s.get('room-commands')!.enqueue('local', id)
  }, trackId)
  expect(sent).toBe(true)
}

// Sortable stores its instance on the element under an expando key.
async function sortableBound(page: Page): Promise<boolean> {
  return page.evaluate(() => {
    const list = document.querySelector('[data-queue-id]')?.parentElement
    return Boolean(list && Object.keys(list).some(key => key.startsWith('Sortable')))
  })
}

// Pausing keeps the queue from draining while drags are measured.
async function pausePlayback(page: Page): Promise<void> {
  await expect.poll(() => page.evaluate(() => {
    const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, Record<string, unknown>> } } } } }).__vue_app__
    const stores = app.config.globalProperties.$pinia._s
    const runtime = stores.get('room-runtime') as { nowPlaying: unknown; isPaused: boolean; idempotencyScopeId: string }
    if (!runtime.nowPlaying || !runtime.idempotencyScopeId) return 'waiting'
    if (runtime.isPaused) return 'paused'
    ;(stores.get('room-commands') as { togglePause: () => boolean }).togglePause()
    return 'toggled'
  }), { timeout: 30_000, intervals: [700] }).toBe('paused')
}

async function removeAll(page: Page): Promise<void> {
  await page.evaluate(() => {
    const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, Record<string, unknown>> } } } } }).__vue_app__
    const stores = app.config.globalProperties.$pinia._s
    const ids = (stores.get('room-runtime') as { queue: Array<{ queueId: string }> }).queue.map(item => item.queueId)
    ;(stores.get('room-commands') as { removeSongs: (ids: string[]) => boolean }).removeSongs(ids)
  })
}

async function visibleRows(page: Page): Promise<number> {
  return page.evaluate(() => {
    const rows = [...document.querySelectorAll('[data-queue-id]')]
    let scroller = rows[0]?.parentElement ?? null
    while (scroller && getComputedStyle(scroller).overflowY !== 'auto') scroller = scroller.parentElement
    const bounds = (scroller ?? document.documentElement).getBoundingClientRect()
    return rows.filter(row => { const box = row.getBoundingClientRect(); return box.top >= bounds.top && box.bottom <= bounds.bottom }).length
  })
}

async function queueScrollTop(page: Page): Promise<number> {
  return page.locator('[data-queue-id]').first().evaluate(element => {
    let current: HTMLElement | null = element.parentElement
    while (current && getComputedStyle(current).overflowY !== 'auto') current = current.parentElement
    return current?.scrollTop ?? 0
  })
}

async function domQueue(page: Page): Promise<string[]> {
  return page.locator('[data-queue-id]').evaluateAll(nodes => nodes.map(node => node.textContent?.trim() ?? ''))
}

function normalize(rows: string[], titles: string[]): string[] {
  return rows.map(row => titles.find(title => row.includes(title)) ?? `?${row.slice(0, 20)}`)
}

async function realMouseDrag(page: Page, fromIndex: number, toIndex: number): Promise<void> {
  const rows = page.locator('[data-queue-id]')
  // Each visible-row probe starts at the same scroll position. A bounding box
  // alone does not prove a row is inside its scroll container.
  const scrollTop = await queueScrollTop(page)
  if (scrollTop > 0) {
    const box = await rows.first().evaluate(element => {
      let scroller = element.parentElement
      while (scroller && getComputedStyle(scroller).overflowY !== 'auto') scroller = scroller.parentElement
      return scroller!.getBoundingClientRect().toJSON()
    })
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
    await page.mouse.wheel(0, -scrollTop - 100)
    await expect.poll(() => queueScrollTop(page)).toBe(0)
  }
  const source = await rows.nth(fromIndex).boundingBox()
  const target = await rows.nth(toIndex).boundingBox()
  if (!source || !target) throw new Error('row not visible')
  const startX = source.x + source.width * 0.35
  const startY = source.y + source.height / 2
  // Cross the midpoint without entering the 80px auto-scroll edge band.
  // Long-distance auto-scroll has a separate mouse driver below.
  const endY = target.y + target.height * (toIndex > fromIndex ? 0.55 : 0.45)
  const geometry = async () => page.evaluate(({ startX, startY, endY }) => {
    const rows = [...document.querySelectorAll<HTMLElement>('[data-queue-id]')]
    let scroller = rows[0]?.parentElement ?? null
    while (scroller && getComputedStyle(scroller).overflowY !== 'auto') scroller = scroller.parentElement
    const hit = (y: number) => document.elementFromPoint(startX, y)?.closest<HTMLElement>('[data-queue-id]')?.dataset.queueId
    return { scrollTop: scroller?.scrollTop, bounds: scroller?.getBoundingClientRect().toJSON(), sourceHit: hit(startY), targetHit: hit(endY), order: rows.map(row => row.dataset.queueId) }
  }, { startX, startY, endY })
  const beforeGeometry = process.env.DRAG_DIAGNOSTICS === '1' ? await geometry() : null
  const visible = await page.evaluate(({ startX, startY, endY }) => {
    const hit = (y: number) => document.elementFromPoint(startX, y)?.closest<HTMLElement>('[data-queue-id]')?.dataset.queueId
    return { source: hit(startY), target: hit(endY) }
  }, { startX, startY, endY })
  expect(visible.source, 'mouse must start on the intended visible row').toBe(await rows.nth(fromIndex).getAttribute('data-queue-id'))
  expect(visible.target, 'mouse must end on the intended visible row').toBe(await rows.nth(toIndex).getAttribute('data-queue-id'))
  await page.mouse.move(startX, startY)
  await page.mouse.down()
  await page.mouse.move(startX, startY + (toIndex > fromIndex ? 6 : -6), { steps: 3 })
  await page.mouse.move(startX, endY, { steps: 12 })
  await page.waitForTimeout(60)
  await page.mouse.up()
  if (beforeGeometry) console.log(`@@DRAG_GEOMETRY ${JSON.stringify({ fromIndex, toIndex, startX, startY, endY, before: beforeGeometry, after: await geometry() })}`)
  expect(await queueScrollTop(page), 'visible-row baseline must not enter auto-scroll').toBe(0)
}

// Move a row through the scroll edge so Sortable's auto-scroll path is exercised.
async function realMouseAutoScrollDrag(page: Page, fromIndex: number, toIndex: number): Promise<void> {
  const rows = page.locator('[data-queue-id]')
  const source = await rows.nth(fromIndex).boundingBox()
  if (!source) throw new Error('source row not visible')
  const bounds = await page.locator('[data-queue-id]').first().evaluate(element => {
    let current: HTMLElement | null = element.parentElement
    while (current && getComputedStyle(current).overflowY !== 'auto') current = current.parentElement
    const box = current?.getBoundingClientRect()
    return box ? { x: box.x, y: box.y, width: box.width, height: box.height } : null
  })
  if (!bounds) throw new Error('queue scroller not visible')
  const x = source.x + source.width * 0.35
  await page.mouse.move(x, source.y + source.height / 2)
  await page.mouse.down()
  await page.mouse.move(x, source.y + source.height / 2 + 8, { steps: 3 })
  for (let step = 0; step < 18; step += 1) {
    await page.mouse.move(x, bounds.y + bounds.height - 8, { steps: 3 })
    await page.waitForTimeout(80)
  }
  const target = await rows.nth(toIndex).boundingBox()
  if (!target) throw new Error('target row did not become visible during auto-scroll')
  await page.mouse.move(x, target.y + target.height * 0.75, { steps: 8 })
  await page.mouse.up()
}

function expectedMove(order: string[], from: number, to: number): string[] {
  const next = [...order]
  const [item] = next.splice(from, 1)
  next.splice(to, 0, item!)
  return next
}

type Sample = { scenario: string; clients: number; size: number; from: number; to: number; ok: boolean; reason: string; latencyMs: number; sent: number; ack: string }

async function measureDrag(actor: Page, wire: Wire, observers: Page[], titles: string[], from: number, to: number, scenario: string): Promise<Sample> {
  const before = normalize(await domQueue(actor), titles)
  const expected = expectedMove(before, from, to)
  const sentBefore = wire.sent.filter(e => e.type === 'queue.reorder').length
  const startedAt = Date.now()
  await realMouseDrag(actor, from, to)
  let reason = ''
  const deadline = startedAt + 2_000
  let converged = false
  while (Date.now() < deadline) {
    const views = await Promise.all([domQueue(actor), storeQueue(actor), ...observers.map(o => domQueue(o))])
    if (views.every(view => JSON.stringify(normalize(view, titles)) === JSON.stringify(expected))) { converged = true; break }
    await actor.waitForTimeout(25)
  }
  const latencyMs = Date.now() - startedAt
  const reorders = wire.sent.filter(e => e.type === 'queue.reorder').slice(sentBefore) as Array<Extract<ClientEnvelope, { type: 'queue.reorder' }>>
  const ids = new Set(reorders.map(e => e.payload.mutationId))
  const acks = wire.received.filter(r => (r.envelope.type === 'queue.reorder.ack' || r.envelope.type === 'queue.reorder.nack') && ids.has(r.envelope.payload.mutationId))
  const ack = acks.map(r => r.envelope.type === 'queue.reorder.nack' ? `nack:${r.envelope.payload.reason}` : 'ack').join(',') || 'none'
  if (!converged) {
    const actorDom = normalize(await domQueue(actor), titles)
    const actorStore = normalize(await storeQueue(actor), titles)
    const observerDom = await Promise.all(observers.map(async o => normalize(await domQueue(o), titles)))
    reason = JSON.stringify({ before, expected, actorDom, actorStore, observerDom, sent: reorders.map(e => e.payload), ack })
  }
  // Let late messages settle so samples stay independent.
  await actor.waitForTimeout(700)
  const sample = { scenario, clients: observers.length + 1, size: before.length, from, to, ok: converged, reason, latencyMs, sent: reorders.length, ack }
  console.log(`@@RELIABILITY_SAMPLE ${JSON.stringify({ recordedAt: new Date().toISOString(), ...sample })}`)
  return sample
}

async function openClient(browser: Browser, roomName: string) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const page = await context.newPage()
  const wire = recordWire(page)
  await login(page)
  await enterRoom(page, roomName)
  return { context, page, wire }
}

async function openObservers(browser: Browser, roomName: string, clientCount: number) {
  const observers: Awaited<ReturnType<typeof openClient>>[] = []
  for (let index = 1; index < clientCount; index += 1) observers.push(await openClient(browser, roomName))
  return observers
}

async function closeObservers(observers: Awaited<ReturnType<typeof openClient>>[]): Promise<void> {
  await Promise.all(observers.map(observer => observer.context.close().catch(() => undefined)))
}

async function waitForQueueOrder(actor: Page, observers: Awaited<ReturnType<typeof openClient>>[], titles: string[], expected: string[], timeout = 2_000, startedAt = Date.now()): Promise<number> {
  await expect.poll(async () => {
    const views = await Promise.all([
      domQueue(actor), storeQueue(actor),
      ...observers.map(observer => domQueue(observer.page)),
    ])
    return views.every(view => JSON.stringify(normalize(view, titles)) === JSON.stringify(expected))
  }, { timeout }).toBe(true)
  return Date.now() - startedAt
}

async function waitForMatchingQueue(actor: Page, observers: Awaited<ReturnType<typeof openClient>>[], titles: string[], length: number, timeout = 2_000, startedAt = Date.now()): Promise<number> {
  await expect.poll(async () => {
    const views = await Promise.all([
      domQueue(actor), ...observers.map(observer => domQueue(observer.page)),
    ])
    const normalized = views.map(view => normalize(view, titles))
    return normalized[0]?.length === length && normalized.every(view => JSON.stringify(view) === JSON.stringify(normalized[0]))
  }, { timeout }).toBe(true)
  return Date.now() - startedAt
}

function recordScenario(scenario: string, clients: number, size: number, latencyMs: number, details: Record<string, number | string | boolean>): void {
  console.log(`@@RELIABILITY_SCENARIO ${JSON.stringify({ scenario, clients, size, latencyMs, ...details })}`)
}

function summarize(samples: Sample[]): string {
  const groups = new Map<string, Sample[]>()
  for (const sample of samples) groups.set(`${sample.scenario} clients=${sample.clients} n=${sample.size}`, [...(groups.get(`${sample.scenario} clients=${sample.clients} n=${sample.size}`) ?? []), sample])
  const lines: string[] = []
  for (const [key, group] of groups) {
    const ok = group.filter(s => s.ok)
    const lat = ok.map(s => s.latencyMs).sort((a, b) => a - b)
    const pct = (p: number) => lat.length ? lat[Math.min(lat.length - 1, Math.floor(p * lat.length))] : NaN
    lines.push(`${key}: ${ok.length}/${group.length} ok, p50=${pct(0.5)}ms p95=${pct(0.95)}ms max=${lat.at(-1) ?? NaN}ms, sent=${group.map(s => s.sent).join('')}`)
  }
  return lines.join('\n')
}

export { enqueueById, enterRoom, login, measureDrag, normalize, domQueue, openClient, recordWire, storeQueue, summarize, uploadFixtures, realMouseDrag, realMouseAutoScrollDrag, type Sample }

test('desktop queue real-mouse drag baseline', async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: Math.max(...sizes) + 1 }, (_, i) => `Drag ${String(i).padStart(2, '0')} ${seed}`)
  const samples: Sample[] = []
  const setup = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const setupPage = await setup.newPage()
  await login(setupPage)
  step('uploading')
  const ids = await uploadFixtures(setupPage, titles)
  step('uploaded')
  await setup.close()

  // Long drags that need autoscroll are out of scope for this probe; moves stay
  // inside the rows that are fully visible without scrolling.
  let visible = 3
  const pattern = (n: number, i: number): [number, number] => {
    const last = Math.min(n, visible) - 1
    const moves: Array<[number, number]> = [[last, 0], [0, last], [1, 0], [0, 1], [last - 1, last]]
    return moves[i % moves.length]!
  }

  for (const clientCount of clientCounts) for (const size of sizes) {
    const roomName = `Drag Room c${clientCount} n${size} ${seed}`
    const actorContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
    const actor = await actorContext.newPage()
    const wire = recordWire(actor)
    await login(actor)
    // Scenario A: room created empty, so the queue list mounts after the first enqueue.
    await enterRoom(actor, roomName, true)
    const observers = await Promise.all(Array.from({ length: clientCount - 1 }, () => openClient(browser, roomName)))
    const observer = observers[0]!
    const extras = observers.slice(1)
    const watchers = [observer.page, ...extras.map(extra => extra.page)]
    try {
      step(`n=${size} room entered, enqueueing`)
      for (const title of titles.slice(0, size + 1)) await enqueueById(actor, ids.get(title)!)
      await pausePlayback(actor)
      await expect.poll(async () => (await domQueue(actor)).length, { timeout: 30_000 }).toBe(size)
      for (const watcher of watchers) await expect.poll(async () => (await domQueue(watcher)).length, { timeout: 30_000 }).toBe(size)
      visible = await visibleRows(actor)
      step(`n=${size} fully visible rows: ${visible}`)
      for (let i = 0; i < repeats; i += 1) {
        const [from, to] = pattern(size, i)
        const sample = await measureDrag(actor, wire, watchers, titles, from, to, 'empty-then-add')
        samples.push(sample); step(`${sample.scenario} n=${size} ${from}->${to} ok=${sample.ok} ${sample.latencyMs}ms sent=${sample.sent} ack=${sample.ack}`)
      }
      step(`n=${size} sortable bound to current list: ${await sortableBound(actor)}`)
      // Scenario C: the list existed, the queue empties, then songs return.
      await removeAll(actor)
      await expect.poll(async () => (await domQueue(actor)).length, { timeout: 15_000 }).toBe(0)
      for (const title of titles.slice(1, size + 1)) await enqueueById(actor, ids.get(title)!)
      await expect.poll(async () => (await domQueue(actor)).length, { timeout: 30_000 }).toBe(size)
      await expect.poll(async () => (await domQueue(observer.page)).length, { timeout: 30_000 }).toBe(size)
      step(`n=${size} after clear+re-add sortable bound to current list: ${await sortableBound(actor)}`)
      for (let i = 0; i < repeats; i += 1) {
        const [from, to] = pattern(size, i)
        const sample = await measureDrag(actor, wire, watchers, titles, from, to, 'clear-then-readd')
        samples.push(sample); step(`${sample.scenario} n=${size} ${from}->${to} ok=${sample.ok} ${sample.latencyMs}ms sent=${sample.sent} ack=${sample.ack}`)
      }
      // Scenario B: a client that joins with a non-empty queue mounts the list immediately.
      await actorContext.close()
      const joiner = await openClient(browser, roomName)
      await expect.poll(async () => (await domQueue(joiner.page)).length, { timeout: 30_000 }).toBe(size)
      step(`n=${size} joiner sortable bound to current list: ${await sortableBound(joiner.page)}`)
      for (let i = 0; i < repeats; i += 1) {
        const [from, to] = pattern(size, i)
        const sample = await measureDrag(joiner.page, joiner.wire, watchers, titles, from, to, 'mounted-with-queue')
        samples.push(sample); step(`${sample.scenario} n=${size} ${from}->${to} ok=${sample.ok} ${sample.latencyMs}ms sent=${sample.sent} ack=${sample.ack}`)
      }
      // Fresh join reads the server's authoritative snapshot.
      const actorFinal = normalize(await domQueue(joiner.page), titles)
      await joiner.context.close()
      const fresh = await openClient(browser, roomName)
      const server = normalize(await domQueue(fresh.page), titles)
      expect(server, `authoritative snapshot must match after clients=${clientCount} n=${size}`).toEqual(actorFinal)
      const dupes = server.length - new Set(server).size
      expect(dupes, `queue must not contain duplicate tracks after clients=${clientCount} n=${size}`).toBe(0)
      console.log(`@@RELIABILITY_INTEGRITY ${JSON.stringify({ clients: clientCount, size, queueLength: server.length, duplicates: dupes, matchesFreshSnapshot: true })}`)
      await fresh.context.close()
    } finally {
      await Promise.all(observers.map(client => client.context.close()))
      await actorContext.close()
    }
  }
  for (const sample of samples.filter(s => !s.ok)) console.log(`FAIL ${sample.scenario} n=${sample.size} ${sample.from}->${sample.to}: ${sample.reason}`)
  console.log(summarize(samples))
  expect(samples.filter(sample => !sample.ok), 'every measured reorder must converge within two seconds').toEqual([])
  expect(samples.filter(sample => sample.sent !== 1 || sample.ack !== 'ack'), 'each drag must be sent and acknowledged exactly once').toEqual([])
})

test('desktop queue real-mouse drag baseline restores a clipped source row', async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: 11 }, (_, i) => `Scroll reset ${i} ${seed}`)
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const actor = await context.newPage()
  const wire = recordWire(actor)
  let observer: Awaited<ReturnType<typeof openClient>> | undefined
  try {
    await login(actor)
    const ids = await uploadFixtures(actor, titles)
    const room = `Scroll reset ${seed}`
    await enterRoom(actor, room, true)
    observer = await openClient(browser, room)
    for (const title of titles) await enqueueById(actor, ids.get(title)!)
    await pausePlayback(actor)
    await expect.poll(() => domQueue(actor).then(rows => rows.length)).toBe(10)
    const before = normalize(await domQueue(actor), titles)
    const expected = expectedMove(before, 0, 1)
    const scroller = await actor.locator('[data-queue-id]').first().evaluate(element => {
      let parent = element.parentElement
      while (parent && getComputedStyle(parent).overflowY !== 'auto') parent = parent.parentElement
      return parent!.getBoundingClientRect().toJSON()
    })
    await actor.mouse.move(scroller.x + scroller.width / 2, scroller.y + scroller.height / 2)
    await actor.mouse.wheel(0, 80)
    await expect.poll(() => queueScrollTop(actor)).toBeGreaterThan(60)
    const clipped = await actor.locator('[data-queue-id]').first().boundingBox()
    expect(clipped!.y + clipped!.height / 2, 'regression must begin with a clipped mouse source').toBeLessThan(scroller.y)
    const sentBefore = wire.sent.length
    await realMouseDrag(actor, 0, 1)
    for (const page of [actor, observer.page]) {
      await expect.poll(async () => normalize(await domQueue(page), titles)).toEqual(expected)
      await expect.poll(async () => normalize(await storeQueue(page), titles)).toEqual(expected)
    }
    const commands = wire.sent.slice(sentBefore).filter(command => command.type === 'queue.reorder')
    expect(commands).toHaveLength(1)
    const mutationId = commands[0]!.payload.mutationId
    await expect.poll(() => wire.received.filter(({ envelope }) => envelope.type === 'queue.reorder.ack' && envelope.payload.mutationId === mutationId).length).toBe(1)
    step('clipped-source regression: scroll reset, intended order, one command and one ACK verified')
  } finally {
    await observer?.context.close()
    await context.close()
  }
})

// Offline contract: while disconnected, drag and skip are unavailable and no
// new command reaches the wire; after reconnect the client resyncs in time.
test('desktop queue offline gating and recovery baseline', async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: 6 + repeats }, (_, i) => `Offline ${String(i).padStart(2, '0')} ${seed}`)
  const roomName = `Offline Room ${seed}`
  const actorContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const actor = await actorContext.newPage()
  const wire = recordWire(actor)
  await login(actor)
  const ids = await uploadFixtures(actor, titles)
  await enterRoom(actor, roomName, true)
  const observer = await openClient(browser, roomName)
  const results: string[] = []
  let violations = 0
  const recoveries: number[] = []
  try {
    for (const title of titles.slice(0, 5)) await enqueueById(actor, ids.get(title)!)
    await pausePlayback(actor)
    await expect.poll(async () => (await domQueue(actor)).length, { timeout: 30_000 }).toBe(4)
    for (let round = 0; round < repeats; round += 1) {
      await actor.context().setOffline(true)
      await expect.poll(() => actor.evaluate(() => navigator.onLine)).toBe(false)
      await actor.waitForTimeout(300)
      const state = await actor.evaluate(() => {
        const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, Record<string, unknown>> } } } } }).__vue_app__
        const connection = app.config.globalProperties.$pinia._s.get('realtime-connection') as { connected: boolean }
        const list = document.querySelector('[data-queue-id]')?.parentElement as (HTMLElement & Record<string, { options?: { disabled?: boolean } }>) | undefined
        const key = list ? Object.keys(list).find(k => k.startsWith('Sortable')) : undefined
        return { storeConnected: connection.connected, sortableDisabled: key ? Boolean(list![key]!.options?.disabled) : null }
      })
      const next = actor.getByRole('button', { name: /^(Next|下一首)$/ })
      const nextDisabled = await next.isDisabled()
      const sentBefore = wire.sent.length
      const domBefore = normalize(await domQueue(actor), titles)
      await realMouseDrag(actor, 2, 0)
      await next.click({ force: true }).catch(() => undefined)
      await actor.waitForTimeout(400)
      const domOffline = normalize(await domQueue(actor), titles)
      const commands = wire.sent.slice(sentBefore).filter(e => e.type === 'queue.reorder' || e.type === 'control.next').length
      const moved = JSON.stringify(domOffline) !== JSON.stringify(domBefore)
      // Remote change while the actor is offline.
      await enqueueById(observer.page, ids.get(titles[5 + round]!)!)
      const expected = await expect.poll(async () => normalize(await domQueue(observer.page), titles), { timeout: 10_000 }).not.toEqual(domBefore).then(() => domQueue(observer.page)).then(rows => normalize(rows, titles))
      const offlineOk = !moved && commands === 0 && nextDisabled && state.sortableDisabled === true && !state.storeConnected
      if (moved || commands > 0) violations += 1
      const restoredAt = Date.now()
      await actor.context().setOffline(false)
      let recovered = -1
      while (Date.now() - restoredAt < 10_000) {
        const ready = await actor.evaluate(() => {
          const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, Record<string, unknown>> } } } } }).__vue_app__
          return Boolean((app.config.globalProperties.$pinia._s.get('realtime-connection') as { connected: boolean }).connected)
        })
        const rows = normalize(await domQueue(actor), titles)
        const enabled = !await next.isDisabled()
        if (ready && enabled && JSON.stringify(rows) === JSON.stringify(expected)) { recovered = Date.now() - restoredAt; break }
        await actor.waitForTimeout(50)
      }
      recoveries.push(recovered)
      const after = await measureDrag(actor, wire, [observer.page], titles, 1, 0, 'post-recovery')
      results.push(`round ${round}: offlineGate=${offlineOk} (storeConnected=${state.storeConnected} sortableDisabled=${state.sortableDisabled} nextDisabled=${nextDisabled}) commandsSentOffline=${commands} domMovedOffline=${moved} recoveryMs=${recovered} postRecoveryDrag=${after.ok}/${after.latencyMs}ms`)
      step(results.at(-1)!)
      // Keep the queue length stable across rounds.
      await actor.evaluate(() => {
        const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, Record<string, unknown>> } } } } }).__vue_app__
        const stores = app.config.globalProperties.$pinia._s
        const queue = (stores.get('room-runtime') as { queue: Array<{ queueId: string }> }).queue
        ;(stores.get('room-commands') as { removeSong: (id: string) => boolean }).removeSong(queue.at(-1)!.queueId)
      })
      await expect.poll(async () => (await domQueue(actor)).length, { timeout: 10_000 }).toBe(4)
      await actor.waitForTimeout(600)
    }
  } finally {
    await observer.context.close().catch(() => undefined)
    await actorContext.close().catch(() => undefined)
  }
  const ok = recoveries.filter(ms => ms >= 0 && ms <= 5_000).sort((a, b) => a - b)
  console.log(`offline: violations=${violations}/${repeats}, recovered<=5s ${ok.length}/${repeats}, p50=${ok[Math.floor(ok.length / 2)]}ms max=${ok.at(-1)}ms, all=${recoveries.join(',')}`)
})

for (const clients of clientCounts) test(`desktop queue auto-scroll drag converges clients=${clients} size=30`, async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: 31 }, (_, i) => `Auto ${String(i).padStart(2, '0')} ${seed}`)
  const actorContext = await browser.newContext({ viewport: { width: 900, height: 700 } })
  const actor = await actorContext.newPage()
  const wire = recordWire(actor)
  await login(actor)
  const ids = await uploadFixtures(actor, titles)
  const room = `Auto Room ${seed}`
  await enterRoom(actor, room, true)
  const observers = await openObservers(browser, room, clients)
  try {
    for (const title of titles) await enqueueById(actor, ids.get(title)!)
    await pausePlayback(actor)
    await expect.poll(() => domQueue(actor).then(rows => rows.length), { timeout: 30_000 }).toBe(titles.length - 1)
    const before = normalize(await domQueue(actor), titles)
    const expected = expectedMove(before, 0, before.length - 1)
    const scrollTopBefore = await queueScrollTop(actor)
    await realMouseAutoScrollDrag(actor, 0, 29)
    const startedAt = Date.now()
    await expect.poll(() => queueScrollTop(actor), { timeout: 5_000 }).toBeGreaterThan(scrollTopBefore)
    const latencyMs = await waitForQueueOrder(actor, observers, titles, expected, 2_000, startedAt)
    const reorders = wire.sent.filter(event => event.type === 'queue.reorder')
    expect(reorders).toHaveLength(1)
    const mutationId = reorders[0]!.payload.mutationId
    await expect.poll(() => wire.received.filter(item => item.envelope.type === 'queue.reorder.ack' && item.envelope.payload.mutationId === mutationId).length).toBe(1)
    recordScenario('auto-scroll', clients, 30, latencyMs, { sent: reorders.length, ack: 1, scrolled: true })
    const fresh = await openClient(browser, room)
    try {
      await expect.poll(async () => normalize(await domQueue(fresh.page), titles), { timeout: 5_000 }).toEqual(expected)
    } finally {
      await fresh.context.close()
    }
  } finally {
    await closeObservers(observers)
    await actorContext.close().catch(() => undefined)
  }
})

for (const clients of clientCounts) for (const size of sizes) test(`desktop queue drag converges with concurrent enqueue and remove clients=${clients} size=${size}`, async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: size + 2 }, (_, i) => `Race ${String(i).padStart(2, '0')} ${seed}`)
  const actorContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const actor = await actorContext.newPage()
  const wire = recordWire(actor)
  await login(actor)
  const ids = await uploadFixtures(actor, titles)
  const room = `Race Room ${seed}`
  await enterRoom(actor, room, true)
  const observers = await openObservers(browser, room, clients)
  let removedTitle = ''
  try {
    for (const title of titles.slice(0, size + 1)) await enqueueById(actor, ids.get(title)!)
    await pausePlayback(actor)
    await expect.poll(() => domQueue(actor).then(rows => rows.length), { timeout: 30_000 }).toBe(size)
    const initial = normalize(await domQueue(actor), titles)
    const startedAt = Date.now()
    const mutation = (async () => {
      const observer = observers[0]!
      await observer.page.waitForTimeout(120)
      await enqueueById(observer.page, ids.get(titles[size + 1])!)
      await observer.page.waitForTimeout(120)
      const queue = await observer.page.evaluate(removeIndex => {
        const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { queue: Array<{ queueId: string; music: { name: string } }> }> } } } } }).__vue_app__
        const item = app.config.globalProperties.$pinia._s.get('room-runtime')!.queue.at(removeIndex)!
        return { queueId: item.queueId, title: item.music.name }
      }, Math.min(2, size - 1))
      removedTitle = queue.title
      await observer.page.evaluate(queueId => {
        const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { removeSong: (id: string) => boolean }> } } } } }).__vue_app__
        app.config.globalProperties.$pinia._s.get('room-commands')!.removeSong(queueId)
      }, queue.queueId)
    })()
    await Promise.all([realMouseDrag(actor, 0, Math.min(7, size - 1)), mutation])
    const latencyMs = await waitForMatchingQueue(actor, observers, titles, size, 2_000, startedAt)
    const final = normalize(await domQueue(actor), titles)
    expect(new Set(final).size).toBe(final.length)
    expect(final).toContain(titles[size + 1]!)
    expect(final).not.toContain(removedTitle)
    expect([...final].sort()).toEqual([...initial.filter(title => title !== removedTitle), titles[size + 1]!].sort())
    recordScenario('concurrent-enqueue-remove', clients, size, latencyMs, { added: 1, removed: 1, duplicates: final.length - new Set(final).size })
    const fresh = await openClient(browser, room)
    try {
      await expect.poll(async () => normalize(await domQueue(fresh.page), titles)).toEqual(final)
    } finally {
      await fresh.context.close()
    }
  } finally {
    await closeObservers(observers)
    await actorContext.close().catch(() => undefined)
  }
})

for (const clients of clientCounts) for (const size of sizes) test(`desktop queue recovers after a single dropped reorder ACK clients=${clients} size=${size}`, async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: size + 1 }, (_, i) => `Loss ${String(i).padStart(2, '0')} ${seed}`)
  const actorContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const wire: Wire = { sent: [], received: [], sentBytes: 0, receivedBytes: 0 }
  let dropNextAck = false
  let droppedAck = false
  let resentMutation = false
  let droppedMutationId = ''
  let droppedAckCount = 0
  let droppedAt = 0
  let forwardedReorderCount = 0
  let replayedReorderCount = 0
  let initialReorder = ''
  let replayedEnvelope = ''
  let socketCount = 0
  await actorContext.routeWebSocket('**/ws?**', socket => {
    const server = socket.connectToServer()
    let lastReorder: string | null = null
    socket.onMessage(message => {
      if (typeof message === 'string') {
        try {
          const envelope = JSON.parse(message) as { type?: string }
          if (envelope.type === 'queue.reorder') {
            lastReorder = message
            forwardedReorderCount += 1
            if (!initialReorder) initialReorder = message
          }
        } catch { /* forward non-JSON frames */ }
      }
      server.send(message)
    })
    server.onMessage(message => {
      if (!dropNextAck || droppedAck || typeof message !== 'string') {
        socket.send(message)
        return
      }
      try {
        const envelope = JSON.parse(message) as { type?: string; payload?: { mutationId?: string } }
        if (envelope.type === 'queue.reorder.ack' && envelope.payload?.mutationId) {
          droppedAck = true
          droppedAt = Date.now()
          droppedMutationId = envelope.payload.mutationId
          droppedAckCount += 1
          dropNextAck = false
          if (lastReorder) {
            setTimeout(() => {
              replayedEnvelope = lastReorder!
              server.send(replayedEnvelope)
              replayedReorderCount += 1
              resentMutation = true
            }, 100)
          }
          return
        }
      } catch { /* forward non-JSON frames */ }
      socket.send(message)
    })
  })
  const actor = await actorContext.newPage()
  actor.on('websocket', () => { socketCount += 1 })
  actor.on('websocket', socket => {
    socket.on('framesent', frame => {
      try { wire.sent.push(JSON.parse(frame.payload.toString()) as ClientEnvelope) } catch { /* binary */ }
    })
    socket.on('framereceived', frame => {
      const envelope = decodeServerEnvelope(frame.payload.toString())
      if (envelope) wire.received.push({ at: Date.now(), envelope })
    })
  })
  await login(actor)
  const ids = await uploadFixtures(actor, titles)
  const room = `Loss Room ${seed}`
  await enterRoom(actor, room, true)
  const observers = await openObservers(browser, room, clients)
  try {
    for (const title of titles) await enqueueById(actor, ids.get(title)!)
    await pausePlayback(actor)
    await expect.poll(() => actor.evaluate(() => navigator.onLine)).toBe(true)
    const before = normalize(await domQueue(actor), titles)
    const queueVersionBefore = await actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { lastQueueVersion: number }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('room-runtime')!.lastQueueVersion
    })
    const socketCountBeforeDrag = socketCount
    const expected = expectedMove(before, 1, 0)
    dropNextAck = true
    await realMouseDrag(actor, 1, 0)
    await expect.poll(() => droppedAck).toBe(true)
    const latencyMs = await waitForQueueOrder(actor, observers, titles, expected, 2_000, droppedAt)
    const mutationId = wire.sent.filter(event => event.type === 'queue.reorder').at(-1)?.payload.mutationId
    expect(mutationId).toBeTruthy()
    await expect.poll(() => resentMutation).toBe(true)
    await expect.poll(() => wire.sent.filter(event => event.type === 'queue.reorder' && event.payload.mutationId === mutationId).length).toBe(2)
    const matchingCommands = wire.sent.filter(event => event.type === 'queue.reorder' && event.payload.mutationId === mutationId)
    expect(matchingCommands).toHaveLength(2)
    expect(forwardedReorderCount).toBe(1)
    expect(replayedReorderCount).toBe(1)
    expect(replayedEnvelope).toBe(initialReorder)
    expect(JSON.parse(replayedEnvelope).payload.mutationId).toBe(mutationId)
    expect(droppedMutationId).toBe(mutationId)
    expect(droppedAckCount).toBe(1)
    await expect.poll(() => wire.received.some(item => item.envelope.type === 'queue.reorder.ack' && item.envelope.payload.mutationId === mutationId), { timeout: 10_000 }).toBe(true)
    await expect.poll(() => actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { lastQueueVersion: number }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('room-runtime')!.lastQueueVersion
    })).toBe(queueVersionBefore + 1)
    expect(normalize(await domQueue(actor), titles)).toEqual(expected)
    for (const observer of observers) expect(normalize(await domQueue(observer.page), titles)).toEqual(expected)
    const connection = await actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { connected: boolean }> } } } } }).__vue_app__
      return { online: navigator.onLine, connected: app.config.globalProperties.$pinia._s.get('realtime-connection')!.connected }
    })
    expect(connection).toEqual({ online: true, connected: true })
    expect(socketCount).toBe(socketCountBeforeDrag)
    recordScenario('dropped-reorder-ack', clients, size, latencyMs, { sent: forwardedReorderCount + replayedReorderCount, droppedAcks: droppedAckCount, ack: 'replayed', versionDelta: 1 })
    const fresh = await openClient(browser, room)
    try {
      await expect.poll(async () => normalize(await domQueue(fresh.page), titles), { timeout: 5_000 }).toEqual(expected)
    } finally {
      await fresh.context.close()
    }
  } finally {
    await closeObservers(observers)
    await actorContext.close().catch(() => undefined)
  }
})

for (const clients of clientCounts) test(`desktop queue recovers from a real WebSocket disconnect while browser stays online clients=${clients} size=5`, async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: 7 }, (_, i) => `Disconnect ${String(i).padStart(2, '0')} ${seed}`)
  const actorContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  let pageSocket: WebSocketRoute | undefined
  let socketCount = 0
  await actorContext.routeWebSocket('**/ws?**', socket => {
    pageSocket = socket
    socket.connectToServer()
  })
  const actor = await actorContext.newPage()
  const wire = recordWire(actor)
  actor.on('websocket', () => { socketCount += 1 })
  try {
    await login(actor)
    const ids = await uploadFixtures(actor, titles)
    const room = `Disconnect Room ${seed}`
    await enterRoom(actor, room, true)
    const observers = await openObservers(browser, room, clients)
    try {
      for (const title of titles.slice(0, 6)) await enqueueById(actor, ids.get(title)!)
      await pausePlayback(actor)
      await expect.poll(() => actor.evaluate(() => navigator.onLine)).toBe(true)
      const before = normalize(await domQueue(actor), titles)
      const priorSocketCount = socketCount
      const priorReorders = wire.sent.filter(event => event.type === 'queue.reorder').length
      const nextButton = actor.getByRole('button', { name: /^(Next|下一首)$/ })
      const disconnectedAt = Date.now()
      await pageSocket!.close({ code: 1011, reason: 'injected transport disconnect' })
      await expect.poll(async () => actor.evaluate(() => {
        const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { connected: boolean }> } } } } }).__vue_app__
        return app.config.globalProperties.$pinia._s.get('realtime-connection')!.connected
      })).toBe(false)
      expect(await actor.evaluate(() => navigator.onLine)).toBe(true)
      await expect(nextButton).toBeDisabled()
      await realMouseDrag(actor, 2, 0)
      expect(wire.sent.filter(event => event.type === 'queue.reorder')).toHaveLength(priorReorders)
      expect(normalize(await domQueue(actor), titles)).toEqual(before)

      const addedTitle = titles[6]!
      await enqueueById(observers[0]!.page, ids.get(addedTitle)!)
      await expect.poll(async () => normalize(await domQueue(observers[0]!.page), titles)).toContain(addedTitle)
      const expected = normalize(await domQueue(observers[0]!.page), titles)
      await expect.poll(async () => actor.evaluate(() => {
        const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { connected: boolean }> } } } } }).__vue_app__
        return app.config.globalProperties.$pinia._s.get('realtime-connection')!.connected
      }), { timeout: 5_000 }).toBe(true)
      await expect.poll(async () => normalize(await domQueue(actor), titles), { timeout: 5_000 }).toEqual(expected)
      for (const observer of observers) await expect.poll(async () => normalize(await domQueue(observer.page), titles), { timeout: 5_000 }).toEqual(expected)
      expect(await actor.evaluate(() => navigator.onLine)).toBe(true)
      expect(socketCount).toBeGreaterThan(priorSocketCount)
      const recoveredAt = Date.now()
      expect(recoveredAt - disconnectedAt).toBeLessThanOrEqual(5_000)
      recordScenario('websocket-disconnect', clients, 5, recoveredAt - disconnectedAt, { connected: true, online: true, snapshot: true })
      const fresh = await openClient(browser, room)
      try {
        await expect.poll(async () => normalize(await domQueue(fresh.page), titles), { timeout: 5_000 }).toEqual(expected)
      } finally {
        await fresh.context.close()
      }
      step(`real WebSocket disconnect converged in ${recoveredAt - disconnectedAt}ms while navigator.onLine stayed true`)
    } finally {
      await closeObservers(observers)
    }
  } finally {
    await actorContext.close().catch(() => undefined)
  }
})

test('pending reorder does not cross room scope after switching rooms', async ({ browser }) => {
  test.setTimeout(180_000)
  const seed = Date.now()
  const titles = Array.from({ length: 3 }, (_, i) => `Scope ${String(i).padStart(2, '0')} ${seed}`)
  const actorContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  let dropNextAck = false
  let droppedAck = false
  const actorWire: Wire = { sent: [], received: [], sentBytes: 0, receivedBytes: 0 }
  await actorContext.routeWebSocket('**/ws?**', socket => {
    const server = socket.connectToServer()
    socket.onMessage(message => server.send(message))
    server.onMessage(message => {
      if (dropNextAck && !droppedAck && typeof message === 'string') {
        try {
          const envelope = JSON.parse(message) as { type?: string }
          if (envelope.type === 'queue.reorder.ack') {
            droppedAck = true
            dropNextAck = false
            return
          }
        } catch { /* forward non-JSON frames */ }
      }
      socket.send(message)
    })
  })
  const actor = await actorContext.newPage()
  actor.on('websocket', socket => {
    socket.on('framesent', frame => {
      const payload = frame.payload.toString()
      actorWire.sentBytes += Buffer.byteLength(payload)
      try { actorWire.sent.push(JSON.parse(payload) as ClientEnvelope) } catch { /* binary */ }
    })
    socket.on('framereceived', frame => {
      const payload = frame.payload.toString()
      actorWire.receivedBytes += Buffer.byteLength(payload)
      const envelope = decodeServerEnvelope(payload)
      if (envelope) actorWire.received.push({ at: Date.now(), envelope })
    })
  })
  let roomBClient: Awaited<ReturnType<typeof openClient>> | undefined
  let observerA: Awaited<ReturnType<typeof openClient>> | undefined
  try {
    await login(actor)
    step('scope: logged in')
    const ids = await uploadFixtures(actor, titles)
    step('scope: fixtures ready')
    const roomA = `Scope A ${seed}`
    const roomB = `Scope B ${seed}`
    await enterRoom(actor, roomA, true)
    step('scope: room A joined')
    const roomAId = await actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { currentRoomId: string }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('room')!.currentRoomId
    })
    observerA = await openClient(browser, roomA)
    step('scope: observer A joined')
    for (const title of titles) await enqueueById(actor, ids.get(title)!)
    await pausePlayback(actor)
    await expect.poll(() => domQueue(actor).then(rows => rows.length)).toBe(2)
    step('scope: queue seeded')

    await actor.evaluate(name => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { createRoom: (name: string) => boolean }> } } } } }).__vue_app__
      app.config.globalProperties.$pinia._s.get('room')!.createRoom(name)
    }, roomB)
    let roomBId = ''
    await expect.poll(async () => {
      roomBId = await actor.evaluate(async name => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { fetchRooms: () => Promise<unknown>; rooms: Array<{ name: string; roomId: string }> }> } } } } }).__vue_app__
      const roomStore = app.config.globalProperties.$pinia._s.get('room')!
      await roomStore.fetchRooms()
      return roomStore.rooms.find(room => room.name === name)?.roomId ?? ''
      }, roomB)
      return roomBId
    }, { timeout: 10_000 }).toBeTruthy()
    expect(roomBId).toBeTruthy()
    step('scope: room B created')
    step('scope: room B created')
    roomBClient = await openClient(browser, roomB)
    step('scope: room B observer joined')
    await enqueueById(roomBClient.page, ids.get(titles[2]!)!)
    await expect.poll(() => storeQueue(roomBClient!.page)).toEqual([])
    step('scope: room B seeded')

    await actor.evaluate(async targetRoomId => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { switchRoom: (roomId: string) => Promise<void> }> } } } } }).__vue_app__
      await app.config.globalProperties.$pinia._s.get('room-realtime-coordinator')!.switchRoom(targetRoomId)
    }, roomAId)
    await expect.poll(() => storeQueue(actor)).toHaveLength(2)
    step('scope: actor confirmed in room A')
    const mutationStart = actorWire.sent.filter(message => message.type === 'queue.reorder').length
    dropNextAck = true
    await realMouseDrag(actor, 1, 0)
    step('scope: reorder sent')
    const reorder = actorWire.sent.filter(message => message.type === 'queue.reorder').at(-1)
    expect(reorder).toBeTruthy()
    await expect.poll(() => droppedAck).toBe(true)
    await expect.poll(() => actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { pendingCount: number }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('room-command-bus')!.pendingCount
    })).toBeGreaterThan(0)
    step('scope: pending mutation observed')
    const roomAOrder = await storeQueue(observerA.page)
    await expect.poll(() => storeQueue(actor)).toEqual(roomAOrder)

    await actor.evaluate(async targetRoomId => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { switchRoom: (roomId: string) => Promise<void> }> } } } } }).__vue_app__
      await app.config.globalProperties.$pinia._s.get('room-realtime-coordinator')!.switchRoom(targetRoomId)
    }, roomBId)
    await expect.poll(async () => actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { currentRoomId: string }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('room')!.currentRoomId
    }), { timeout: 10_000 }).toBe(roomBId)
    await expect(actor.locator('header').getByRole('button', { name: /Current room:/ })).toBeVisible()
    step('scope: actor switched to room B')
    await expect.poll(() => actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { idempotencyScopeId: string }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('room-runtime')!.idempotencyScopeId
    }), { timeout: 10_000 }).toBeTruthy()
    const scopeAfterSwitch = await actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { idempotencyScopeId: string; pendingCount?: number }> } } } } }).__vue_app__
      const stores = app.config.globalProperties.$pinia._s
      return { scope: stores.get('room-runtime')!.idempotencyScopeId, pending: stores.get('room-command-bus')?.pendingCount ?? 0 }
    })
    await actor.waitForTimeout(1_500)
    const replayedAcrossScope = actorWire.sent.filter(message => message.type === 'queue.reorder')
      .slice(mutationStart + 1).some(message => message.payload.mutationId === reorder!.payload.mutationId)
    expect(replayedAcrossScope).toBe(false)
    expect(scopeAfterSwitch.scope).toBeTruthy()
    expect(scopeAfterSwitch.pending).toBe(0)
    const roomBPlayback = await roomBClient.page.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { nowPlaying: { music: { name: string } } | null }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('room-runtime')!.nowPlaying?.music.name ?? ''
    })
    expect(roomBPlayback).toBe(titles[2])
    expect(await storeQueue(roomBClient.page)).toEqual([])
    const freshA = await openClient(browser, roomA)
    const freshB = await openClient(browser, roomB)
    try {
      expect(await storeQueue(freshA.page)).toEqual(roomAOrder)
      const freshBPlayback = await freshB.page.evaluate(() => {
        const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { nowPlaying: { music: { name: string } } | null }> } } } } }).__vue_app__
        return app.config.globalProperties.$pinia._s.get('room-runtime')!.nowPlaying?.music.name ?? ''
      })
      expect(freshBPlayback).toBe(titles[2])
      expect(await storeQueue(freshB.page)).toEqual([])
      expect(await storeQueue(actor)).toEqual(await storeQueue(roomBClient.page))
    } finally {
      await freshA.context.close()
      await freshB.context.close()
    }
    step('scope: fresh snapshots verified')
    console.log(`@@RELIABILITY_SCOPE ${JSON.stringify({ scenario: 'room-switch-pending-reorder', roomA, roomB, mutationId: reorder!.payload.mutationId, droppedAck, pendingAfterSwitch: scopeAfterSwitch.pending, replayedAcrossScope, roomAQueue: roomAOrder, roomBQueue: await storeQueue(roomBClient.page) })}`)
  } finally {
    await roomBClient?.context.close().catch(() => undefined)
    await observerA?.context.close().catch(() => undefined)
    await actorContext.close().catch(() => undefined)
  }
})

test('local WAV playback alignment and websocket frame cost are measured separately', async ({ browser }) => {
  const seed = Date.now()
  const title = `Audio ${seed}`
  const actor = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
  const actorWire = recordWire(actor)
  let observer: Awaited<ReturnType<typeof openClient>> | undefined
  try {
    await login(actor)
    const ids = await uploadFixtures(actor, [title])
    const room = `Audio Room ${seed}`
    await enterRoom(actor, room, true)
    observer = await openClient(browser, room)
    const sentBefore = actorWire.sent.length
    const receivedBefore = actorWire.received.length
    const clientBytesBefore = actorWire.sentBytes
    const serverBytesBefore = actorWire.receivedBytes
    await enqueueById(actor, ids.get(title)!)
    const audioSelector = 'audio:not([aria-hidden="true"])'
    await expect.poll(() => actor.locator(audioSelector).first().evaluate(audio => (audio as HTMLAudioElement).readyState), { timeout: 15_000 }).toBeGreaterThanOrEqual(2)
    await expect.poll(() => observer!.page.locator(audioSelector).first().evaluate(audio => (audio as HTMLAudioElement).readyState), { timeout: 15_000 }).toBeGreaterThanOrEqual(2)
    await actor.locator(audioSelector).first().evaluate(audio => (audio as HTMLAudioElement).play())
    await observer.page.locator(audioSelector).first().evaluate(audio => (audio as HTMLAudioElement).play())
    await actor.waitForTimeout(2_000)
    const audioTimes = await Promise.all([actor, observer.page].map(page => page.locator(audioSelector).first().evaluate(audio => {
      const element = audio as HTMLAudioElement
      return { currentTime: element.currentTime, paused: element.paused, readyState: element.readyState }
    })))
    expect(audioTimes.every(sample => !sample.paused && sample.readyState >= 2)).toBe(true)
    const mediaDriftMs = Math.abs(audioTimes[0]!.currentTime - audioTimes[1]!.currentTime) * 1000
    const sentAfter = actorWire.sent.length
    const frames = actorWire.sent.slice(sentBefore)
    const sample = { scenario: 'audio-and-cost', mediaDriftMs, audioTimes, clientFrames: frames.length, clientEnvelopeBytes: actorWire.sentBytes - clientBytesBefore, serverFrames: actorWire.received.length - receivedBefore, serverEnvelopeBytes: actorWire.receivedBytes - serverBytesBefore, totalFramesObserved: sentAfter - sentBefore }
    console.log(`@@RELIABILITY_AUDIO ${JSON.stringify(sample)}`)
  } finally {
    await observer?.context.close().catch(() => undefined)
    await actor.close().catch(() => undefined)
  }
})

test('TCP silent half-open is detected and recovers while browser stays online', async ({ browser }) => {
  const seed = Date.now()
  const titles = Array.from({ length: 7 }, (_, i) => `TCP ${String(i).padStart(2, '0')} ${seed}`)
  const target = new URL(process.env.E2E_BASE_URL || 'http://127.0.0.1:18081')
  const proxy = createTcpFaultProxy(target.hostname, Number(target.port))
  const proxyPort = await proxy.listen()
  const actorContext = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
  const actor = await actorContext.newPage()
  await actor.addInitScript(({ proxyPort }) => {
    const NativeWebSocket = window.WebSocket
    window.WebSocket = class extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        const proxied = new URL(String(url))
        if (proxied.pathname === '/ws') proxied.port = String(proxyPort)
        if (protocols === undefined) super(proxied.toString())
        else super(proxied.toString(), protocols)
      }
    }
  }, { proxyPort })
  const wire = recordWire(actor)
  let observer: Awaited<ReturnType<typeof openClient>> | undefined
  try {
    await login(actor)
    const ids = await uploadFixtures(actor, titles)
    const room = `TCP Room ${seed}`
    await enterRoom(actor, room, true)
    observer = await openClient(browser, room)
    for (const title of titles.slice(0, 6)) await enqueueById(actor, ids.get(title)!)
    await pausePlayback(actor)
    await expect.poll(() => domQueue(actor).then(rows => rows.length), { timeout: 30_000 }).toBe(5)
    const before = normalize(await domQueue(actor), titles)
    const previousReorders = wire.sent.filter(event => event.type === 'queue.reorder').length
    const nextButton = actor.getByRole('button', { name: /^(Next|下一首)$/ })

    const faultStartedAt = Date.now()
    proxy.setMode('drop-server-to-client')
    await expect.poll(() => proxy.snapshot().droppedServerBytes > 0, { timeout: 30_000 }).toBe(true)
    await expect.poll(async () => actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { connected: boolean }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('realtime-connection')!.connected
    }), { timeout: 30_000, intervals: [250] }).toBe(false)
    const detectionMs = Date.now() - faultStartedAt
    expect(await actor.evaluate(() => navigator.onLine)).toBe(true)
    await expect(nextButton).toBeDisabled()
    await realMouseDrag(actor, 2, 0)
    expect(wire.sent.filter(event => event.type === 'queue.reorder')).toHaveLength(previousReorders)
    expect(normalize(await domQueue(actor), titles)).toEqual(before)

    const addedTitle = titles[6]!
    await enqueueById(observer.page, ids.get(addedTitle)!)
    await expect.poll(async () => normalize(await domQueue(observer!.page), titles)).toContain(addedTitle)
    const expected = normalize(await domQueue(observer.page), titles)
    proxy.setMode('pass')
    const restoredAt = Date.now()
    await expect.poll(async () => actor.evaluate(() => {
      const app = (document.querySelector('#app') as unknown as { __vue_app__: { config: { globalProperties: { $pinia: { _s: Map<string, { connected: boolean }> } } } } }).__vue_app__
      return app.config.globalProperties.$pinia._s.get('realtime-connection')!.connected
    }), { timeout: 5_000 }).toBe(true)
    await expect.poll(async () => normalize(await domQueue(actor), titles), { timeout: 5_000 }).toEqual(expected)
    const recoveryMs = Date.now() - restoredAt
    expect(recoveryMs).toBeLessThanOrEqual(5_000)
    expect(await actor.evaluate(() => navigator.onLine)).toBe(true)
    expect(proxy.snapshot().droppedServerBytes).toBeGreaterThan(0)
    console.log(`@@RELIABILITY_TCP ${JSON.stringify({ scenario: 'silent-half-open-server-to-client', detectionMs, recoveryMs, droppedServerBytes: proxy.snapshot().droppedServerBytes, online: true })}`)
  } finally {
    await observer?.context.close().catch(() => undefined)
    await actorContext.close().catch(() => undefined)
    await proxy.close()
  }
})
