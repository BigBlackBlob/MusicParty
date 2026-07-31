#!/usr/bin/env node

import WebSocket from '../../music-party-web/node_modules/ws/wrapper.mjs';

const baseUrl = process.argv[2] ?? 'http://127.0.0.1:18881';
const username = process.argv[3] ?? 'stage8-admin';
const password = process.argv[4] ?? 'Stage8-Password-2026!';
const musicId = process.argv[5] ?? '28816031';

const login = await fetch(`${baseUrl}/api/account/login`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ username, password })
});
if (!login.ok) throw new Error(`login failed: ${login.status} ${await login.text()}`);
await login.json();
const cookies = login.headers.getSetCookie?.() ?? [login.headers.get('set-cookie')].filter(Boolean);
const cookie = cookies.map(value => value.split(';', 1)[0]).join('; ');
const sessionToken = cookie.split('; ').find(value => value.startsWith('MP_SESSION='))?.slice('MP_SESSION='.length);
if (!sessionToken || !cookie.includes('MP_SESSION=')) throw new Error('login response is missing session state');

const socket = new WebSocket(`${baseUrl.replace(/^http/, 'ws')}/ws?room-id=lounge`, {
  headers: { Cookie: cookie },
  origin: baseUrl,
  perMessageDeflate: false
});
await new Promise((resolve, reject) => {
  socket.once('open', resolve);
  socket.once('error', reject);
});

const waitFor = (predicate, timeoutMs = 15_000) => new Promise((resolve, reject) => {
  const timeout = setTimeout(() => {
    socket.off('message', onMessage);
    reject(new Error(`WebSocket message timeout after ${timeoutMs}ms`));
  }, timeoutMs);
  const onMessage = raw => {
    const message = JSON.parse(raw);
    if (!predicate(message)) return;
    clearTimeout(timeout);
    socket.off('message', onMessage);
    resolve(message);
  };
  socket.on('message', onMessage);
});

const mutationId = `stage8-playback-${Date.now()}`;
const statePromise = waitFor(message => message.type === 'player.state' && message.payload?.nowPlaying?.music?.id === musicId);
const ackPromise = waitFor(message => message.type === 'enqueue.ack' && message.payload?.mutationId === mutationId);
socket.send(JSON.stringify({ type: 'enqueue', payload: { platform: 'netease', musicId, mutationId } }));
await ackPromise;
const state = await statePromise;
const music = state.payload.nowPlaying.music;
if (!music.name || music.name === musicId || !music.artists?.length || music.duration <= 0 || !music.url) {
  throw new Error(`incomplete resolved music: ${JSON.stringify(music)}`);
}

const streamUrl = new URL(music.url, baseUrl);
streamUrl.searchParams.set('token', sessionToken);
const stream = await fetch(streamUrl, { headers: { Range: 'bytes=0-65535' } });
if (![200, 206].includes(stream.status)) throw new Error(`stream failed: ${stream.status} ${await stream.text()}`);
const audio = new Uint8Array(await stream.arrayBuffer());
if (audio.length === 0) throw new Error('stream returned no audio bytes');

const requestedPosition = Math.min(30_000, Math.floor(music.duration / 2));
const seekStatePromise = waitFor(message => message.type === 'player.state'
  && message.payload?.nowPlaying?.music?.id === musicId
  && message.payload.nowPlaying.currentPosition >= requestedPosition);
socket.send(JSON.stringify({ type: 'control.seek', payload: { positionMs: requestedPosition } }));
const seekState = await seekStatePromise;
socket.close();

console.log(JSON.stringify({
  music,
  stream: {
    status: stream.status,
    contentType: stream.headers.get('content-type'),
    contentRange: stream.headers.get('content-range'),
    bytesRead: audio.length
  },
  seek: {
    requestedPosition,
    observedPosition: seekState.payload.nowPlaying.currentPosition,
    playEpoch: seekState.payload.nowPlaying.playEpoch
  }
}, null, 2));
