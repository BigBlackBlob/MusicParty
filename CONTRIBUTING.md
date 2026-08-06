# Contributing to MusicParty

MusicParty is a Go-only backend with a Vue 3 frontend. Prefer small, typed changes and keep runtime data, credentials and generated binaries out of Git.

## Toolchain

- Go version declared by `backend-go/go.mod`
- Node.js 22
- pnpm 11.10.0
- Docker Desktop or Docker Engine for container verification
- FFmpeg for media and browser E2E scenarios

## Local development

```bash
./start-dev.sh
./start-dev.sh --start-netease-api
```

Use `.env.local` for local secrets. The launcher writes disposable state and PID files under `.dev-logs/` and stops only processes it started. See `docs/local-verification-guide.md`.

## Required checks

```bash
cd backend-go
./ci/verify.sh

cd ../music-party-web
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm test:run
pnpm build
```

Use `./local-verification.sh` when a change affects Docker packaging, startup, HTTP/WebSocket integration or browser behavior. Tests should protect a meaningful contract or failure mode; do not add ceremonial coverage for deleted functionality.

## Contracts and database changes

- Go owns HTTP, WebSocket and environment contracts. Regenerate them with `go run ./cmd/contractgen -repo ..` from `backend-go/` and commit the generated diff.
- Do not hand-edit generated frontend contracts.
- Go owns future SQLite migrations. Every schema change must be versioned and tested from the frozen or prior supported schema.
- Never use a production or personal runtime database as a fixture.

## Frontend changes

- Keep TypeScript strict and do not introduce business `any`.
- HTTP resources belong in TanStack Vue Query; Pinia should not duplicate them.
- Components must not construct transport URLs or WebSocket message destinations directly.
- Do not update visual baselines automatically. Review intentional UI changes at the supported desktop and mobile viewports.

## Commits and pull requests

Use concise conventional commits where practical, for example `fix(web): resync presence after reconnect`. Rebase a PR branch on the latest `NRT-Base`, create a normal (non-draft) PR, and describe the problem before the solution. Never commit `.env` files, cookies, SQLite runtime databases, media cache files or build executables.
