#!/usr/bin/env node

import { performance } from 'node:perf_hooks';

const options = Object.fromEntries(process.argv.slice(2).map(argument => {
  const normalized = argument.replace(/^--/, '');
  const separator = normalized.indexOf('=');
  return separator < 0
    ? [normalized, 'true']
    : [normalized.slice(0, separator), normalized.slice(separator + 1)];
}));
const url = options.url ?? 'ws://localhost:8080/ws?room-id=lounge';
const connections = Number(options.connections ?? 10);
const slowClients = Number(options.slow ?? 1);
const durationMs = Number(options.durationMs ?? 30_000);

if (typeof WebSocket === 'undefined') {
  throw new Error('This script requires Node.js 22+ with the built-in WebSocket client.');
}

const startedAt = performance.now();
let opened = 0;
let received = 0;
let closed = 0;
let closedBeforeFinish = 0;
const closeCodes = {};
let stopping = false;
const clients = Array.from({ length: connections }, (_, index) => {
  const socket = new WebSocket(url);
  socket.addEventListener('open', () => {
    opened += 1;
    socket.send(JSON.stringify({ type: 'player.resync', requestId: `baseline-${index}`, payload: {} }));
  });
  if (index >= slowClients) socket.addEventListener('message', () => { received += 1; });
  socket.addEventListener('close', event => {
    closed += 1;
    if (!stopping) closedBeforeFinish += 1;
    closeCodes[event.code] = (closeCodes[event.code] ?? 0) + 1;
  });
  return socket;
});

await new Promise(resolve => setTimeout(resolve, durationMs));
stopping = true;
clients.forEach(socket => socket.close());
await new Promise(resolve => setTimeout(resolve, 500));

console.log(JSON.stringify({
  url,
  connections,
  slowClients,
  opened,
  received,
  closed,
  closedBeforeFinish,
  closeCodes,
  durationMs: Math.round(performance.now() - startedAt)
}, null, 2));
