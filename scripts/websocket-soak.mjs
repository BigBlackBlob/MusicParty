#!/usr/bin/env node
import WebSocket from '../music-party-web/node_modules/ws/wrapper.mjs';

const count = Number(process.argv[2] ?? 300);
const durationMs = Number(process.argv[3] ?? 3_600_000);
const baseUrl = process.argv[4] ?? 'http://127.0.0.1:18081';
const sampleEveryMs = 30_000;
const password = 'PerfHarness-Local-2026';
const startedAt = Date.now();
const samples = [];
const result = { requested: count, opened: 0, closed: 0, errors: 0, messagesReceived: 0, slowClient: 0, samples };

const register = await fetch(`${baseUrl}/api/account/register`, {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ username: `soak-${startedAt}`, password })
});
if (!register.ok) throw new Error(`register: HTTP ${register.status}`);
const rawCookie = register.headers.getSetCookie?.()[0] ?? register.headers.get('set-cookie');
if (!rawCookie) throw new Error('register: missing session cookie');
const cookie = rawCookie.split(';', 1)[0];
const loungeUrl = `${baseUrl.replace(/^http/, 'ws')}/ws?room-id=lounge`;
const setup = new WebSocket(loungeUrl, { headers: { Cookie: cookie }, origin: baseUrl, perMessageDeflate: false });
await new Promise((resolve, reject) => {
  setup.once('open', resolve);
  setup.once('error', reject);
});
await new Promise(resolve => setTimeout(resolve, 250));
const roomId = await new Promise((resolve, reject) => {
  const timeout = setTimeout(() => reject(new Error('room creation timed out')), 8_000);
  setup.on('message', raw => {
    try {
      const message = JSON.parse(raw);
      if (message.type === 'rooms.created' && (message.payload?.id ?? message.payload?.roomId)) {
        clearTimeout(timeout);
        resolve(message.payload.id ?? message.payload.roomId);
      }
    } catch {}
  });
  setup.send(JSON.stringify({ type: 'rooms.create', payload: { name: `WS soak ${startedAt}`, isPrivate: false } }));
});
setup.close();
const wsUrl = `${baseUrl.replace(/^http/, 'ws')}/ws?room-id=${encodeURIComponent(roomId)}`;
const connect = () => new Promise((resolve, reject) => {
  const socket = new WebSocket(wsUrl, { headers: { Cookie: cookie }, origin: baseUrl, perMessageDeflate: false });
  socket.once('open', () => { result.opened++; resolve(socket); });
  socket.once('error', reject);
  socket.on('error', () => { result.errors++; });
  socket.on('close', () => { result.closed++; });
});
const passiveSockets = await Promise.all(Array.from({ length: count - 1 }, connect));
const leader = await connect();
await new Promise(resolve => setTimeout(resolve, 250));
const sockets = [...passiveSockets, leader];
const slowSocket = sockets[0];
slowSocket._socket?.pause();
for (const [index, socket] of sockets.entries()) {
  if (index !== 0) socket.on('message', () => { result.messagesReceived++; });
  else result.slowClient = index;
}

const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const percentile = (values, p) => values.length ? values[Math.max(0, Math.ceil(values.length * p) - 1)] : null;
async function metric(name, tag = '') {
  const suffix = tag ? `?tag=${encodeURIComponent(tag)}` : '';
  const response = await fetch(`${baseUrl}/actuator/metrics/${name}${suffix}`);
  if (!response.ok) return null;
  const data = await response.json();
  return data.measurements?.[0]?.value ?? null;
}

await sleep(5_000);
const tick = async () => {
  const live = sockets.filter(socket => socket.readyState === WebSocket.OPEN).length;
  const sample = { elapsedMs: Date.now() - startedAt, live,
    heapBytes: await metric('jvm.memory.used', 'area:heap'),
    gcCount: await metric('jvm.gc.pause', 'action:end of major GC'),
    dbWriteQueued: await metric('executor.queued', 'name:musicparty.executor') };
  samples.push(sample);
  process.stderr.write(`sample ${JSON.stringify(sample)}\n`);
};
await tick();
const control = setInterval(() => {
  const leader = sockets.at(-1);
  if (leader?.readyState === WebSocket.OPEN) leader.send(JSON.stringify({ type: 'control.toggle-shuffle', payload: {} }));
}, 5_000);
const sampler = setInterval(() => { tick().catch(() => {}); }, sampleEveryMs);
await sleep(durationMs);
clearInterval(control);
clearInterval(sampler);
await tick();
const liveBeforeClose = sockets.filter(socket => socket.readyState === WebSocket.OPEN).length;
slowSocket._socket?.resume();
sockets.forEach(socket => socket.close());
await sleep(500);
const heapValues = samples.map(sample => sample.heapBytes).filter(Number.isFinite).sort((a, b) => a - b);
console.log(JSON.stringify({ ...result, durationMs, liveBeforeClose,
  heapBytes: { min: heapValues[0] ?? null, p50: percentile(heapValues, .5), max: heapValues.at(-1) ?? null,
    first: samples[0]?.heapBytes ?? null, last: samples.at(-1)?.heapBytes ?? null } }));
