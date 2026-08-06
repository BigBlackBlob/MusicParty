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
