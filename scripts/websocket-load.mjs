#!/usr/bin/env node

import { performance } from 'node:perf_hooks';

const options = Object.fromEntries(process.argv.slice(2).map(argument => {
  const [key, value = 'true'] = argument.replace(/^--/, '').split('=', 2);
  return [key, value];
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
const clients = Array.from({ length: connections }, (_, index) => {
  const socket = new WebSocket(url);
  socket.addEventListener('open', () => {
    opened += 1;
    socket.send(JSON.stringify({ type: 'player.resync', requestId: `baseline-${index}`, payload: {} }));
  });
  if (index >= slowClients) socket.addEventListener('message', () => { received += 1; });
  socket.addEventListener('close', () => { closed += 1; });
  return socket;
});

await new Promise(resolve => setTimeout(resolve, durationMs));
clients.forEach(socket => socket.close());
await new Promise(resolve => setTimeout(resolve, 500));

console.log(JSON.stringify({
  url,
  connections,
  slowClients,
  opened,
  received,
  closed,
  durationMs: Math.round(performance.now() - startedAt)
}, null, 2));
