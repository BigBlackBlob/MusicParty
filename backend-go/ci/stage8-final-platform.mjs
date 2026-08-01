#!/usr/bin/env node

import { readFile } from 'node:fs/promises';
import WebSocket from '../../music-party-web/node_modules/ws/wrapper.mjs';

const baseUrl = process.argv[2] ?? 'http://127.0.0.1:18881';
const mode = process.argv[3];
const modeArgument = process.argv[4];
const username = process.env.STAGE8_USERNAME ?? 'stage8-admin';
const password = process.env.STAGE8_PASSWORD ?? 'Stage8-Password-2026!';

if (!['bilibili', 'local'].includes(mode)) {
  throw new Error('usage: stage8-final-platform.mjs <base-url> <bilibili|local> <user-id|audio-file>');
}
if (!modeArgument) throw new Error(`${mode} requires its mode-specific argument`);

const checkedFetch = async (url, options = {}) => {
  const response = await fetch(url, { ...options, signal: options.signal ?? AbortSignal.timeout(120_000) });
  if (!response.ok) throw new Error(`${options.method ?? 'GET'} ${new URL(url, baseUrl).pathname} failed: ${response.status}`);
  return response;
};

const login = await checkedFetch(`${baseUrl}/api/account/login`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ username, password })
});
await login.json();
const cookies = login.headers.getSetCookie?.() ?? [login.headers.get('set-cookie')].filter(Boolean);
const cookie = cookies.map(value => value.split(';', 1)[0]).join('; ');
const sessionToken = cookie.split('; ').find(value => value.startsWith('MP_SESSION='))?.slice('MP_SESSION='.length);
const csrfToken = cookie.split('; ').find(value => value.startsWith('MP_CSRF='))?.slice('MP_CSRF='.length);
if (!sessionToken || !csrfToken) throw new Error('login response is missing session or CSRF state');

const openSocket = async roomId => {
  const socket = new WebSocket(`${baseUrl.replace(/^http/, 'ws')}/ws?room-id=${encodeURIComponent(roomId)}`, {
    headers: { Cookie: cookie },
    origin: baseUrl,
    perMessageDeflate: false
  });
  await new Promise((resolve, reject) => {
    socket.once('open', resolve);
    socket.once('error', reject);
  });
  return socket;
};

const waitFor = (socket, predicate, timeoutMs = 120_000) => new Promise((resolve, reject) => {
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

const enqueueAndVerify = async ({ platform, musicId, roomId }) => {
  const socket = await openSocket(roomId);
  try {
    const mutationId = `stage8-${platform}-${Date.now()}`;
    const statePromise = waitFor(socket, message => message.type === 'player.state'
      && message.payload?.nowPlaying?.music?.id === musicId);
    const ackPromise = waitFor(socket, message => (message.type === 'enqueue.ack' || message.type === 'enqueue.nack')
      && message.payload?.mutationId === mutationId);
    socket.send(JSON.stringify({ type: 'enqueue', payload: { platform, musicId, mutationId } }));
    const ack = await ackPromise;
    if (ack.type !== 'enqueue.ack') throw new Error(`${platform} enqueue rejected`);
    const state = await statePromise;
    const music = state.payload.nowPlaying.music;
    if (!music.name || music.name === musicId || !music.artists?.length || music.duration <= 0 || !music.url) {
      throw new Error(`${platform} resolved incomplete playable metadata`);
    }

    const streamUrl = new URL(music.url, baseUrl);
    streamUrl.searchParams.set('token', sessionToken);
    const stream = await checkedFetch(streamUrl, { headers: { Range: 'bytes=0-65535' } });
    const audio = new Uint8Array(await stream.arrayBuffer());
    const contentRange = stream.headers.get('content-range');
    if (stream.status !== 206 || !contentRange || audio.length === 0) {
      throw new Error(`${platform} stream did not satisfy Range: status=${stream.status} bytes=${audio.length}`);
    }

    const requestedPosition = Math.min(30_000, Math.floor(music.duration / 2));
    const seekPromise = waitFor(socket, message => message.type === 'player.state'
      && message.payload?.nowPlaying?.music?.id === musicId
      && message.payload.nowPlaying.currentPosition >= requestedPosition);
    socket.send(JSON.stringify({ type: 'control.seek', payload: { positionMs: requestedPosition } }));
    const seek = await seekPromise;

    return {
      music: { id: music.id, platform: music.platform, duration: music.duration, metadataComplete: true },
      stream: {
        status: stream.status,
        contentType: stream.headers.get('content-type'),
        contentRange,
        bytesRead: audio.length
      },
      seek: { requestedPosition, observedPosition: seek.payload.nowPlaying.currentPosition }
    };
  } finally {
    socket.close();
  }
};

if (mode === 'bilibili') {
  const keyword = process.env.STAGE8_BILIBILI_KEYWORD ?? '音乐';
  const search = await checkedFetch(`${baseUrl}/api/search/bilibili/${encodeURIComponent(keyword)}?offset=0&limit=10`);
  const results = await search.json();
  if (!Array.isArray(results) || results.length === 0) throw new Error('Bilibili real search returned no results');

  const playlists = await checkedFetch(`${baseUrl}/api/user/playlists/bilibili/${encodeURIComponent(modeArgument)}`);
  const playlistResults = await playlists.json();
  if (!Array.isArray(playlistResults)) throw new Error('Bilibili favorites response was not an array');

  let playback;
  let lastError;
  for (const candidate of results.slice(0, 5)) {
    try {
      playback = await enqueueAndVerify({ platform: 'bilibili', musicId: candidate.id, roomId: 'lounge' });
      break;
    } catch (error) {
      lastError = error;
    }
  }
  if (!playback) throw lastError ?? new Error('no Bilibili result was playable');
  console.log(JSON.stringify({
    platform: 'bilibili',
    search: { status: search.status, resultCount: results.length },
    favorites: { status: playlists.status, resultCount: playlistResults.length },
    playback
  }, null, 2));
}

if (mode === 'local') {
  const title = `Stage 8 Local ${Date.now()}`;
  const form = new FormData();
  form.set('title', title);
  form.set('artists', 'Stage 8');
  form.set('file', new Blob([await readFile(modeArgument)], { type: 'audio/wav' }), 'stage8-local.wav');
  const upload = await checkedFetch(`${baseUrl}/api/local/tracks/upload`, {
    method: 'POST',
    headers: { Cookie: cookie, 'X-CSRF-Token': csrfToken },
    body: form
  });
  const uploaded = await upload.json();
  const trackId = uploaded.track?.id;
  if (!trackId) throw new Error('local upload response is missing track id');

  let completed;
  const deadline = Date.now() + 120_000;
  while (Date.now() < deadline) {
    const response = await checkedFetch(`${baseUrl}/api/local/tracks?sessionToken=${encodeURIComponent(sessionToken)}`);
    const tracks = await response.json();
    completed = tracks.find(track => track.id === trackId);
    if (completed?.status === 'COMPLETED') break;
    if (completed?.status === 'FAILED') throw new Error('local FFmpeg transcode failed');
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  if (completed?.status !== 'COMPLETED') throw new Error('local FFmpeg transcode timed out');

  const search = await checkedFetch(`${baseUrl}/api/search/local/${encodeURIComponent(title)}?offset=0&limit=10&token=${encodeURIComponent(sessionToken)}`);
  const results = await search.json();
  if (!Array.isArray(results) || !results.some(track => track.id === trackId)) {
    throw new Error('completed local track was not searchable');
  }

  const playback = await enqueueAndVerify({ platform: 'local', musicId: trackId, roomId: 'lounge' });
  const deletion = await checkedFetch(`${baseUrl}/api/local/tracks/${encodeURIComponent(trackId)}?token=${encodeURIComponent(sessionToken)}`, {
    method: 'DELETE',
    headers: { Cookie: cookie, 'X-CSRF-Token': csrfToken }
  });
  await deletion.json();
  const afterDelete = await checkedFetch(`${baseUrl}/api/local/tracks?sessionToken=${encodeURIComponent(sessionToken)}`);
  const remaining = await afterDelete.json();
  const mediaAfterDelete = await fetch(`${baseUrl}/api/local/media/${encodeURIComponent(trackId)}?token=${encodeURIComponent(sessionToken)}`);
  if (remaining.some(track => track.id === trackId && track.status !== 'DELETED')) {
    throw new Error('local track remained active after deletion');
  }
  if (![404, 410].includes(mediaAfterDelete.status)) {
    throw new Error(`deleted local media remained readable: ${mediaAfterDelete.status}`);
  }

  console.log(JSON.stringify({
    platform: 'local',
    upload: { status: upload.status, duplicate: Boolean(uploaded.duplicate) },
    transcode: { status: completed.status, durationMs: completed.durationMs, oggCreated: Boolean(completed.oggPath) },
    search: { status: search.status, matched: true },
    playback,
    deletion: {
      status: deletion.status,
      activeRecordRemoved: !remaining.some(track => track.id === trackId && track.status !== 'DELETED'),
      mediaStatus: mediaAfterDelete.status
    }
  }, null, 2));
}
