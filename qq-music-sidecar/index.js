import { createServer } from 'node:http';
import { getPlayUrl, search } from '@sansenjian/qq-music-api/sdk';

// The SDK's debug logs can include the cookie and its derived authentication token.
process.env.DEBUG = 'false';

const pageSize = 50;
const port = Number(process.env.QQ_SIDECAR_PORT || '3100');

function json(response, status, body) {
  response.writeHead(status, { 'Content-Type': 'application/json' });
  response.end(JSON.stringify(body));
}

async function searchPage(keyword, page) {
  const result = await search({ key: keyword, limit: pageSize, page });
  const songs = result.body?.response?.data?.song?.list;
  if (result.status !== 200 || !Array.isArray(songs)) {
    throw new Error('QQ Music search failed');
  }
  return songs;
}

async function searchTracks(response, params) {
  const keyword = params.get('keyword')?.trim();
  const offset = Number(params.get('offset') ?? '0');
  const requestedLimit = Number(params.get('limit') ?? '20');
  if (!keyword || !Number.isSafeInteger(offset) || offset < 0 ||
      !Number.isSafeInteger(requestedLimit) || requestedLimit < 1) {
    return json(response, 400, { error: 'invalid_search_parameters' });
  }

  const limit = Math.min(requestedLimit, pageSize);
  const page = Math.floor(offset / pageSize) + 1;
  const start = offset % pageSize;
  const songs = await searchPage(keyword, page);
  const window = songs.slice(start, start + limit);
  // An offset-based request can span two of the SDK's fixed-size pages.
  if (songs.length === pageSize && window.length < limit) {
    const next = await searchPage(keyword, page + 1);
    window.push(...next.slice(0, limit - window.length));
  }
  const tracks = window.filter(song => song?.songmid).map(song => ({
    id: song.songmid,
    name: song.songname ?? '',
    artists: Array.isArray(song.singer) ? song.singer.map(singer => singer.name).filter(Boolean) : [],
    duration: (song.interval ?? 0) * 1000,
    coverUrl: song.albummid
      ? `https://y.gtimg.cn/music/photo_new/T002R300x300M000${encodeURIComponent(song.albummid)}.jpg`
      : '',
  }));
  return json(response, 200, tracks);
}

async function resolveTrack(request, response, params) {
  const id = params.get('id');
  if (!id || !/^[a-z0-9]+$/i.test(id)) {
    return json(response, 400, { error: 'invalid_song_id' });
  }
  const cookie = request.headers['x-qq-cookie'];
  const result = await getPlayUrl({ songmid: id, cookie });
  if (result.status !== 200) {
    throw new Error('QQ Music playback resolution failed');
  }
  const url = result.body?.data?.playUrl?.[id]?.url;
  if (!url) {
    return json(response, 422, {
      error: 'no_playable_url',
      message: 'Track is unavailable or requires purchase',
    });
  }
  // QQ may advertise an HTTP CDN address. Keep the signed path and query intact.
  const secureUrl = url.replace(/^http:/i, 'https:');
  if (new URL(secureUrl).protocol !== 'https:') {
    throw new Error('QQ Music returned an invalid playback URL');
  }
  return json(response, 200, { url: secureUrl, expires: 0 });
}

createServer(async (request, response) => {
  try {
    const { pathname, searchParams } = new URL(request.url, 'http://127.0.0.1');
    if (request.method === 'GET') {
      switch (pathname) {
        case '/health': return json(response, 200, { ok: true });
        case '/search': return await searchTracks(response, searchParams);
        case '/resolve': return await resolveTrack(request, response, searchParams);
      }
    }
    return json(response, 404, { error: 'not_found' });
  } catch {
    return json(response, 502, { error: 'qq_upstream_error' });
  }
}).listen(port, '127.0.0.1');
