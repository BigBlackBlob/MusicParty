#!/usr/bin/env node
import WebSocket from '../music-party-web/node_modules/ws/wrapper.mjs';
import { performance } from 'node:perf_hooks';

const count = Number(process.argv[2] ?? 10);
const baseUrl = process.argv[3] ?? 'http://127.0.0.1:8080';
const origin = process.argv[4] ?? 'http://127.0.0.1:15173';
const sendResync = process.argv[5] !== '--no-resync';
const perMessageDeflate = process.argv[6] !== '--no-deflate';
const adminUsername = process.env.BOOTSTRAP_ADMIN_USERNAME ?? 'perf-admin';
const adminPassword = process.env.BOOTSTRAP_ADMIN_PASSWORD ?? 'PerfHarness-Local-2026';

const cookiesFrom = response => {
  const values = response.headers.getSetCookie?.() ?? [response.headers.get('set-cookie')].filter(Boolean);
  const jar = new Map();
  values.forEach(value => {
    const pair = value.split(';', 1)[0];
    const separator = pair.indexOf('=');
    if (separator > 0) jar.set(pair.slice(0, separator), pair.slice(separator + 1));
  });
  return jar;
};
const cookieHeader = jar => [...jar].map(([name, value]) => `${name}=${value}`).join('; ');
const percentile = (values, quantile) => {
  if (values.length === 0) return null;
  const sorted = [...values].sort((left, right) => left - right);
  return Number(sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * quantile) - 1)].toFixed(2));
};

const adminResponse = await fetch(`${baseUrl}/api/account/login`, {
  method: 'POST', headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ username: adminUsername, password: adminPassword })
});
if (!adminResponse.ok) throw new Error(`admin login: HTTP ${adminResponse.status}`);
const adminCookies = cookiesFrom(adminResponse);
const csrf = adminCookies.get('MP_CSRF');
if (!adminCookies.get('MP_SESSION') || !csrf) throw new Error('admin login: missing session or CSRF cookie');

const createSession = async index => {
  const inviteResponse = await fetch(`${baseUrl}/api/rooms/lounge/invites`, {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      Cookie: cookieHeader(adminCookies),
      'X-CSRF-Token': csrf
    },
    body: JSON.stringify({ label: `load-${index}` })
  });
  if (!inviteResponse.ok) throw new Error(`invite ${index}: HTTP ${inviteResponse.status}`);
  const invite = await inviteResponse.json();
  const response = await fetch(`${baseUrl}/api/invites/redeem`, {
    method: 'POST', headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ secret: invite.secret, displayName: `perf-${index}` })
  });
  if (!response.ok) throw new Error(`redeem ${index}: HTTP ${response.status}`);
  const memberCookies = cookiesFrom(response);
  if (!memberCookies.get('MP_SESSION')) throw new Error(`redeem ${index}: missing session cookie`);
  return cookieHeader(memberCookies);
};

const sessions = [];
const setupConcurrency = Number(process.env.WS_SETUP_CONCURRENCY ?? 20);
for (let offset = 0; offset < count; offset += setupConcurrency) {
  const indexes = Array.from({ length: Math.min(setupConcurrency, count - offset) }, (_, index) => offset + index);
  sessions.push(...await Promise.all(indexes.map(createSession)));
}

const result = { requested: count, opened: 0, messages: 0, errors: 0, closeCodes: {}, closeReasons: {} };
const pingStartedAt = new Map();
const pongLatenciesMs = [];
const sockets = sessions.map((cookie, index) => new WebSocket(
  baseUrl.replace(/^http/, 'ws') + '/ws?room-id=lounge', { headers: { Cookie: cookie }, origin, perMessageDeflate }
));
sockets.forEach((socket, index) => {
  socket.on('open', () => {
    result.opened++;
    if (sendResync) socket.send(JSON.stringify({ type: 'player.resync', requestId: `perf-${index}`, payload: {} }));
    const pingId = `baseline-ping-${index}`;
    pingStartedAt.set(pingId, performance.now());
    socket.send(JSON.stringify({
      type: 'sync.ping',
      requestId: pingId,
      payload: { pingId, clientSendTime: Date.now() }
    }));
  });
  socket.on('message', data => {
    result.messages++;
    try {
      const message = JSON.parse(data.toString());
      if (message.type !== 'sync.pong') return;
      const pingId = message.payload?.pingId;
      const startedAt = pingStartedAt.get(pingId);
      if (startedAt == null) return;
      pongLatenciesMs.push(performance.now() - startedAt);
      pingStartedAt.delete(pingId);
    } catch {
      // Non-JSON messages remain counted; the compatibility harness reports them elsewhere.
    }
  });
  socket.on('error', () => result.errors++);
  socket.on('close', (code, reason) => {
    result.closeCodes[code] = (result.closeCodes[code] ?? 0) + 1;
    const label = reason?.toString() || '(none)';
    result.closeReasons[label] = (result.closeReasons[label] ?? 0) + 1;
  });
});
await new Promise(resolve => setTimeout(resolve, 5000));
result.open = sockets.filter(socket => socket.readyState === WebSocket.OPEN).length;
result.syncPong = {
  received: pongLatenciesMs.length,
  p50Ms: percentile(pongLatenciesMs, 0.5),
  p95Ms: percentile(pongLatenciesMs, 0.95),
  maxMs: pongLatenciesMs.length === 0 ? null : Number(Math.max(...pongLatenciesMs).toFixed(2))
};
sockets.forEach(socket => socket.close());
await new Promise(resolve => setTimeout(resolve, 300));
console.log(JSON.stringify(result));
