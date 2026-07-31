#!/usr/bin/env node

import { performance } from 'node:perf_hooks';
import WebSocket from '../../music-party-web/node_modules/ws/wrapper.mjs';

const options = Object.fromEntries(process.argv.slice(2).map(value => {
  const [key, ...rest] = value.replace(/^--/, '').split('=');
  return [key, rest.join('=') || 'true'];
}));
const baseUrl = options.baseUrl ?? 'http://127.0.0.1:18081';
const connections = Number(options.connections ?? 10);
const durationMs = Number(options.durationMs ?? 30_000);
const recoveryMs = Number(options.recoveryMs ?? 5_000);
const queueSize = Number(options.queueSize ?? 0);
const rooms = Number(options.rooms ?? 0);
const username = options.username ?? 'stage8-admin';
const password = options.password ?? 'Stage8-Password-2026!';
const queuePlatform = options.queuePlatform ?? 'netease';
const queueMusicId = options.queueMusicId ?? '28816031';

const sleep = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const percentile = (values, p) => {
  if (!values.length) return null;
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.max(0, Math.ceil(sorted.length * p) - 1)];
};

const login = await fetch(`${baseUrl}/api/account/login`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ username, password })
});
if (!login.ok) throw new Error(`login failed: HTTP ${login.status} ${await login.text()}`);
const setCookies = login.headers.getSetCookie?.() ?? [login.headers.get('set-cookie')].filter(Boolean);
const cookie = setCookies.map(value => value.split(';', 1)[0]).join('; ');
if (!cookie.includes('MP_SESSION=')) throw new Error('login did not return MP_SESSION');

const wsBase = baseUrl.replace(/^http/, 'ws');
function connect(roomId = 'lounge') {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(`${wsBase}/ws?room-id=${encodeURIComponent(roomId)}`, {
      headers: { Cookie: cookie },
      origin: baseUrl,
      perMessageDeflate: false
    });
    socket.state = { queueVersion: 0, messages: 0 };
    socket.on('message', raw => {
      socket.state.messages++;
      try {
        const message = JSON.parse(raw);
        const version = Number(message.payload?.queueVersion ?? 0);
        if (version > socket.state.queueVersion) socket.state.queueVersion = version;
      } catch {}
    });
    socket.once('open', () => resolve(socket));
    socket.once('error', reject);
  });
}

function waitForMessage(socket, predicate, timeoutMs = 10_000) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      socket.off('message', onMessage);
      reject(new Error(`message timeout after ${timeoutMs}ms`));
    }, timeoutMs);
    const onMessage = raw => {
      try {
        const message = JSON.parse(raw);
        if (!predicate(message)) return;
        clearTimeout(timeout);
        socket.off('message', onMessage);
        resolve(message);
      } catch {}
    };
    socket.on('message', onMessage);
  });
}

async function prometheusMetrics() {
  const response = await fetch(`${baseUrl}/actuator/prometheus`);
  if (!response.ok) return {};
  const text = await response.text();
  const read = name => {
    const match = text.match(new RegExp(`^${name}\\s+([0-9.eE+-]+)$`, 'm'));
    return match ? Number(match[1]) : null;
  };
  return {
    goroutines: read('go_goroutines'),
    rssBytes: read('process_resident_memory_bytes'),
    heapBytes: read('go_memstats_heap_alloc_bytes')
  };
}

const startedAt = performance.now();
const before = await prometheusMetrics();
const sockets = await Promise.all(Array.from({ length: connections }, () => connect()));
const openedMs = performance.now() - startedAt;
await sleep(250);
const leader = sockets.at(-1);
const ackLatencies = [];

for (let index = 0; index < queueSize; index++) {
  const mutationId = `stage8-${Date.now()}-${index}`;
  const waiting = waitForMessage(leader, message => message.type === 'enqueue.ack' && message.payload?.mutationId === mutationId);
  const sentAt = performance.now();
  leader.send(JSON.stringify({
    type: 'enqueue',
    payload: { platform: queuePlatform, musicId: queueMusicId, mutationId }
  }));
  await waiting;
  ackLatencies.push(performance.now() - sentAt);
}

let convergenceMs = null;
if (queueSize > 0) {
  const targetVersion = Math.max(...sockets.map(socket => socket.state.queueVersion));
  const convergenceStarted = performance.now();
  while (performance.now() - convergenceStarted < 2_000) {
    if (sockets.every(socket => socket.state.queueVersion >= targetVersion)) break;
    await sleep(10);
  }
  convergenceMs = performance.now() - convergenceStarted;
}

const roomSockets = [];
for (let index = 0; index < rooms; index++) {
  const name = `Stage 8 Room ${Date.now()} ${index}`;
  const waiting = waitForMessage(leader, message => message.type === 'rooms.created' && message.payload?.name === name);
  leader.send(JSON.stringify({ type: 'rooms.create', payload: { name, isPrivate: false } }));
  const created = await waiting;
  roomSockets.push(await connect(created.payload.roomId));
}

const interval = setInterval(() => {
  for (const socket of [leader, ...roomSockets]) {
    if (socket.readyState === WebSocket.OPEN) socket.send(JSON.stringify({ type: 'control.toggle-shuffle', payload: {} }));
  }
}, 1_000);
await sleep(durationMs);
clearInterval(interval);

const during = await prometheusMetrics();
for (const socket of [...roomSockets, ...sockets]) socket.close();
await sleep(recoveryMs);
const after = await prometheusMetrics();

const report = {
  baseUrl,
  connections,
  rooms,
  queueSize,
  durationMs,
  recoveryMs,
  openedMs: Math.round(openedMs),
  messages: sockets.reduce((total, socket) => total + socket.state.messages, 0),
  ackMs: {
    count: ackLatencies.length,
    p50: percentile(ackLatencies, 0.5),
    p95: percentile(ackLatencies, 0.95),
    max: ackLatencies.length ? Math.max(...ackLatencies) : null
  },
  convergenceMs,
  metrics: { before, during, after }
};
console.log(JSON.stringify(report, null, 2));
if (ackLatencies.length && report.ackMs.p95 >= 250) process.exitCode = 2;
if (convergenceMs !== null && convergenceMs >= 2_000) process.exitCode = 3;
