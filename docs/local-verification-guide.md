# Local verification

## Development stack

Requirements: Go, Node.js 22, pnpm 11.10.0 and, for optional media features, FFmpeg. Put local secrets in `.env.local`; `cookies.json` is supported only as a compatibility input and is ignored by Git.

```bash
./start-dev.sh
./start-dev.sh --start-netease-api
./start-dev.sh --navidrome-local
./start-dev.sh --backend-only --skip-browser
./start-dev.sh --frontend-only --env-file .env.local
```

Defaults are Go API `127.0.0.1:18081`, Vite `127.0.0.1:5173` and optional Netease API `127.0.0.1:3000`. Logs, PID files and the disposable development database live under ignored `.dev-logs/`. Ctrl+C stops only child processes created by the launcher.

On Windows, the wrapper first validates and stops only PIDs recorded for this repository:

```powershell
.\scripts\fresh-start.ps1 -StartNeteaseApi
.\scripts\fresh-start.ps1 -BackendOnly -SkipBrowser
```

## CI-aligned checks

Backend:

```bash
cd backend-go
./ci/verify.sh
```

Frontend:

```bash
cd music-party-web
pnpm install --frozen-lockfile
pnpm typecheck
pnpm lint
pnpm test:run
pnpm build
```

Container and browser acceptance:

```bash
./local-verification.sh
```

The verification script builds the root Dockerfile, creates a uniquely named temporary container and SQLite directory, checks health/readiness, runs Playwright, and cleans up only resources created by that invocation. It does not create `.env.test` or touch a daily-use Compose project.
