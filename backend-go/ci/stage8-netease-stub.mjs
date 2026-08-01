#!/usr/bin/env node

import { createServer } from 'node:http';

const options = Object.fromEntries(process.argv.slice(2).map(value => {
  const [key, ...rest] = value.replace(/^--/, '').split('=');
  return [key, rest.join('=') || 'true'];
}));
const port = Number(options.port);
if (!Number.isInteger(port) || port < 1 || port > 65_535) {
  throw new Error('stage8-netease-stub requires a valid --port');
}

const json = (response, value) => {
  response.writeHead(200, { 'content-type': 'application/json; charset=utf-8' });
  response.end(JSON.stringify(value));
};

const server = createServer((request, response) => {
  const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
  if (url.pathname === '/health') {
    json(response, { status: 'UP' });
    return;
  }
  if (url.pathname === '/song/detail') {
    const id = url.searchParams.get('ids') ?? 'stage8-load-track';
    json(response, {
      songs: [{
        id,
        name: 'Stage 8 Load Track',
        dt: 240_000,
        ar: [{ name: 'Stage 8' }],
        al: { picUrl: '' }
      }]
    });
    return;
  }

  response.writeHead(404, { 'content-type': 'application/json; charset=utf-8' });
  response.end(JSON.stringify({ code: 404 }));
});

server.listen(port, '127.0.0.1');

const shutdown = () => server.close(() => process.exit(0));
process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);
