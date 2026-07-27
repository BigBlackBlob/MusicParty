#!/usr/bin/env node
import WebSocket from '../music-party-web/node_modules/ws/wrapper.mjs';

process.on('unhandledRejection', error => {
  console.error(error.stack || error.message);
  process.exit(1);
});

const rooms = Number(process.argv[2] ?? 20);
const operationsPerRoom = Number(process.argv[3] ?? 25);
const baseUrl = process.argv[4] ?? 'http://127.0.0.1:18081';
const origin = process.argv[5] ?? baseUrl;
const deadlineMs = 8_000;
const runId = Date.now();
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const percentile = (samples, p) => samples[Math.max(0, Math.ceil(samples.length * p) - 1)];

function open(cookie, roomId = 'lounge') {
  const ws = new WebSocket(`${baseUrl.replace(/^http/, 'ws')}/ws?room-id=${encodeURIComponent(roomId)}`, {
    headers: { Cookie: cookie }, origin, perMessageDeflate: false
  });
  const messages = [];
  ws.on('message', raw => { try { messages.push(JSON.parse(raw)); } catch {} });
  return new Promise((resolve, reject) => {
    ws.once('open', async () => { await sleep(250); resolve({ ws, messages }); });
    ws.once('error', reject);
  });
}

async function nextMessage(client, predicate, label) {
  const started = Date.now();
  for (;;) {
    const index = client.messages.findIndex(predicate);
    if (index >= 0) return client.messages.splice(index, 1)[0];
    if (Date.now() - started > deadlineMs) throw new Error(`timeout waiting for ${label}; received=${client.messages.map(message => message.type).join(',')}; events=${client.messages.filter(message => message.type === 'player.events').map(message => `${message.payload?.action}:${message.payload?.message}`).join(',')}`);
    await sleep(5);
  }
}

function send(client, type, payload) { client.ws.send(JSON.stringify({ type, payload })); }

async function register(index) {
  const response = await fetch(`${baseUrl}/api/account/register`, {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ username: `queue-perf-${Date.now()}-${index}`, password: 'PerfHarness-Local-2026' })
  });
  if (!response.ok) throw new Error(`register ${index}: HTTP ${response.status}`);
  const raw = response.headers.getSetCookie?.()[0] ?? response.headers.get('set-cookie');
  if (!raw) throw new Error(`register ${index}: missing cookie`);
  return raw.split(';', 1)[0];
}

async function prepare(index) {
  const cookie = await register(index);
  const lounge = await open(cookie);
  send(lounge, 'rooms.create', { name: `Queue perf ${runId}-${index}`, isPrivate: false });
  const created = await nextMessage(lounge, message => message.type === 'rooms.created', 'room creation');
  const roomId = created.payload?.id ?? created.payload?.roomId;
  if (!roomId) throw new Error(`room ${index}: missing id`);
  lounge.ws.close();
  const client = await open(cookie, roomId);
  send(client, 'enqueue', { platform: 'fixture', musicId: `fixture-a-${index}` });
  const firstAppend = await nextMessage(client, message => message.type === 'queue.patch' && message.payload?.operation === 'append', 'first fixture queue append');
  send(client, 'enqueue', { platform: 'fixture', musicId: `fixture-b-${index}` });
  const secondAppend = await nextMessage(client, message => message.type === 'queue.patch' && message.payload?.operation === 'append', 'second fixture queue append');
  send(client, 'enqueue', { platform: 'fixture', musicId: `fixture-c-${index}` });
  const thirdAppend = await nextMessage(client, message => message.type === 'queue.patch' && message.payload?.operation === 'append', 'third fixture queue append');
  const queue = [
    ...(Array.isArray(secondAppend.payload?.items) ? secondAppend.payload.items : [])
    ,...(Array.isArray(thirdAppend.payload?.items) ? thirdAppend.payload.items : [])
  ];
  if (queue.length < 2 || queue.some(item => !item?.queueId)) throw new Error(`room ${index}: fixture patches missing queue IDs`);
  return { client, queue };
}

const prepared = [];
for (let index = 0; index < rooms; index++) {
  prepared.push(await prepare(index));
  console.error(`prepared room ${index + 1}/${rooms}`);
}
const samples = [];
const nacks = [];
const failures = [];
await Promise.all(prepared.map(async ({ client, queue }, roomIndex) => {
  let first = queue[0].queueId;
  let second = queue[1].queueId;
  for (let operation = 0; operation < operationsPerRoom; operation++) {
    const mutationId = `perf-${roomIndex}-${operation}`;
    const started = performance.now();
    send(client, 'queue.reorder', { oldIndex: 0, newIndex: 1, queueId: first, targetQueueId: second, position: 'after', mutationId });
    try {
      const response = await nextMessage(client, message =>
        (message.type === 'queue.reorder.ack' || message.type === 'queue.reorder.nack') && message.payload?.mutationId === mutationId,
        `ACK ${mutationId}`);
      const elapsed = performance.now() - started;
      if (response.type === 'queue.reorder.ack') {
        samples.push(elapsed);
        [first, second] = [second, first];
      } else nacks.push({ mutationId, reason: response.payload?.reason ?? 'unknown', elapsed });
    } catch (error) { failures.push({ mutationId, error: error.message }); }
    await sleep(600);
  }
}));
prepared.forEach(({ client }) => client.ws.close());
samples.sort((a, b) => a - b);
console.log(JSON.stringify({ rooms, requested: rooms * operationsPerRoom, acked: samples.length, nacks, failures,
  latencyMs: samples.length ? { min: samples[0], p50: percentile(samples, .50), p95: percentile(samples, .95), p99: percentile(samples, .99), max: samples.at(-1) } : null }));
