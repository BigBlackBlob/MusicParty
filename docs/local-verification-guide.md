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

On Windows, compare visual baselines in the default headless mode, then run desktop background playback acceptance in a visible browser against the same isolated server:

```powershell
$env:E2E_BASE_URL = 'http://127.0.0.1:18848'
cd music-party-web
pnpm test:e2e
pnpm test:e2e --headed --grep 'desktop playback clock'
```

The hidden-tab case launches a disposable Chromium profile and connects without Playwright's default focus emulation. It checks native tab visibility, advancing audio time, and lyric recovery. It skips headless and non-Windows runs; those runs do not satisfy desktop background playback acceptance. Its trace and temporary browser profile are kept under the ignored `test-results/` directory.
