import http from 'node:http';

const backendUrl = new URL(process.env.PREVIEW_BACKEND_URL || 'http://127.0.0.1:18083');
const port = Number(process.env.PREVIEW_API_PORT || 18082);

const platforms = [
  { id: 'netease', label: 'NetEase Demo', supportsAlbumSearch: true, subsonic: false },
  { id: 'bilibili', label: 'Bilibili Demo', supportsAlbumSearch: false, subsonic: false },
  { id: 'local', label: 'Local Demo', supportsAlbumSearch: false, subsonic: false }
];

const songs = [
  { id: 'demo-n1', name: 'Midnight City', artists: ['M83'], duration: 241000, platform: 'netease', coverUrl: '' },
  { id: 'demo-n2', name: 'Sunset Lover', artists: ['Petit Biscuit'], duration: 237000, platform: 'netease', coverUrl: '' },
  { id: 'demo-n3', name: 'A Walk', artists: ['Tycho'], duration: 315000, platform: 'netease', coverUrl: '' },
  { id: 'demo-n4', name: 'Nightcall', artists: ['Kavinsky'], duration: 258000, platform: 'netease', coverUrl: '' },
  { id: 'demo-b1', name: 'Blue Hour', artists: ['Preview Artist'], duration: 198000, platform: 'bilibili', coverUrl: '' },
  { id: 'demo-b2', name: 'Afterglow', artists: ['Preview Artist'], duration: 226000, platform: 'bilibili', coverUrl: '' },
  { id: 'demo-l1', name: 'Local Session 01', artists: ['MusicParty Demo'], duration: 204000, platform: 'local', coverUrl: '' },
  { id: 'demo-l2', name: 'Local Session 02', artists: ['MusicParty Demo'], duration: 252000, platform: 'local', coverUrl: '' }
];

const albums = [
  { id: 'demo-album-01', name: 'Night Drive', artist: 'M83', artistName: 'M83', trackCount: 8, coverUrl: '' },
  { id: 'demo-album-02', name: 'Daydream', artist: 'Tycho', artistName: 'Tycho', trackCount: 10, coverUrl: '' },
  { id: 'demo-album-03', name: 'Soft Focus', artist: 'Petit Biscuit', artistName: 'Petit Biscuit', trackCount: 9, coverUrl: '' }
];

function sendJson(response, data) {
  response.writeHead(200, {
    'content-type': 'application/json; charset=utf-8',
    'cache-control': 'no-store'
  });
  response.end(JSON.stringify(data));
}

function page(items, url) {
  const offset = Math.max(0, Number(url.searchParams.get('offset') || 0));
  const limit = Math.min(100, Math.max(1, Number(url.searchParams.get('limit') || 20)));
  return items.slice(offset, offset + limit);
}

function proxyToBackend(request, response) {
  const target = new URL(request.url || '/', backendUrl);
  const upstream = http.request(target, {
    method: request.method,
    headers: { ...request.headers, host: backendUrl.host }
  }, (upstreamResponse) => {
    response.writeHead(upstreamResponse.statusCode || 502, upstreamResponse.headers);
    upstreamResponse.pipe(response);
  });

  upstream.on('error', (error) => {
    response.writeHead(502, { 'content-type': 'application/json; charset=utf-8' });
    response.end(JSON.stringify({ code: 'PREVIEW_BACKEND_UNAVAILABLE', message: error.message }));
  });
  request.pipe(upstream);
}

const server = http.createServer((request, response) => {
  const url = new URL(request.url || '/', 'http://127.0.0.1');
  if (request.method === 'GET' && url.pathname === '/api/platforms') return sendJson(response, platforms);

  const searchMatch = url.pathname.match(/^\/api\/search\/([^/]+)\/(.*)$/);
  if (request.method === 'GET' && searchMatch) {
    const platform = decodeURIComponent(searchMatch[1]);
    const available = songs.filter((song) => song.platform === platform);
    const keyword = decodeURIComponent(searchMatch[2]).trim().toLowerCase();
    const filtered = keyword
      ? available.filter((song) => `${song.name} ${song.artists.join(' ')}`.toLowerCase().includes(keyword))
      : available;
    return sendJson(response, page(filtered.length ? filtered : available, url));
  }

  const albumSearchMatch = url.pathname.match(/^\/api\/album\/search\/([^/]+)$/);
  if (request.method === 'GET' && albumSearchMatch) {
    const keyword = (url.searchParams.get('keyword') || '').trim().toLowerCase();
    const filtered = keyword
      ? albums.filter((album) => `${album.name} ${album.artist}`.toLowerCase().includes(keyword))
      : albums;
    return sendJson(response, filtered.length ? filtered : albums);
  }

  const albumSongsMatch = url.pathname.match(/^\/api\/album\/songs\/([^/]+)\/([^/]+)$/);
  if (request.method === 'GET' && albumSongsMatch) {
    const albumIndex = Math.max(0, albums.findIndex((album) => album.id === decodeURIComponent(albumSongsMatch[2])));
    const first = songs.filter((song) => song.platform === 'netease').slice(albumIndex, albumIndex + 3);
    return sendJson(response, first);
  }

  proxyToBackend(request, response);
});

server.listen(port, '127.0.0.1', () => {
  console.log(`Preview API listening at http://127.0.0.1:${port}; unmatched REST requests proxy to ${backendUrl}`);
});
