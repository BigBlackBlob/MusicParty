# MusicParty web frontend

This Vue 3 application supports the MusicParty Go backend only. HTTP and WebSocket boundaries are generated from the Go-owned contracts under `src/contracts/generated/`.

## Toolchain

- Node.js 22
- pnpm 11.10.0

## Development

From the repository root, the preferred launcher starts the Go API on port 18081 and Vite on port 5173:

```bash
./start-dev.sh
```

To run only the frontend against an existing backend:

```bash
./start-dev.sh --frontend-only
```

You can also work directly in this directory:

```bash
pnpm install --frozen-lockfile
VITE_BACKEND_URL=http://127.0.0.1:18081 pnpm dev
```

## Local UI preview with fixture music

The preview API supplies clearly labeled placeholder platforms, songs, and albums. Other REST requests pass through to a local Go backend; the frontend WebSocket connects directly to that backend.

Start a local Go backend with an isolated database on port `18083`, then run these commands in separate terminals:

```powershell
$env:PREVIEW_BACKEND_URL = 'http://127.0.0.1:18083'
$env:PREVIEW_API_PORT = '18082'
pnpm preview:api
```

```powershell
$env:VITE_BACKEND_URL = 'http://127.0.0.1:18083'
$env:VITE_API_URL = 'http://127.0.0.1:18082'
pnpm dev -- --host 127.0.0.1 --port 5174
```

## Checks

```bash
pnpm typecheck
pnpm lint
pnpm test:run
pnpm build
pnpm test:e2e
pnpm audit --audit-level=moderate --registry=https://registry.npmjs.org
```

Do not hand-edit generated contracts, introduce browser-readable authentication tokens, duplicate Vue Query resources in Pinia, or update visual snapshots without reviewing the rendered change.
