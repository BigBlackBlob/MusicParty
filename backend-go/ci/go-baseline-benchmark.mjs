#!/usr/bin/env node

import { spawn, execFile } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { performance } from 'node:perf_hooks';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const repositoryRoot = resolve('..');
const port = Number(process.env.BASELINE_PORT ?? 18082);
const healthSamples = Number(process.env.BASELINE_HTTP_SAMPLES ?? 200);
const websocketCounts = (process.env.BASELINE_WS_COUNTS ?? '10,100,300').split(',').map(Number).filter(value => value > 0);
const executable = resolve(process.env.BASELINE_GO_EXE ?? 'acceptance/stage8/musicparty-benchmark.exe');
const origin = process.env.BASELINE_ORIGIN ?? 'http://127.0.0.1:15173';
const baseUrl = `http://127.0.0.1:${port}`;
const runtimeRoot = await mkdtemp(join(tmpdir(), 'musicparty-go-baseline-'));
const stdout = [];
const stderr = [];

const child = spawn(executable, [], {
  cwd: runtimeRoot,
  windowsHide: true,
  env: {
    ...process.env,
    SERVER_PORT: String(port),
    APP_MODE: 'server',
    BASE_URL: baseUrl,
    ALLOWED_ORIGINS: `${baseUrl},${origin}`,
    DB_ENABLED: 'true',
    DB_INIT_SCHEMA: 'true',
    DB_PATH: join(runtimeRoot, 'musicparty.db'),
    LOCAL_LIBRARY_PATH: join(runtimeRoot, 'local-library'),
    CACHE_PATH: join(runtimeRoot, 'cache'),
    BOOTSTRAP_ADMIN_USERNAME: 'perf-admin',
    BOOTSTRAP_ADMIN_PASSWORD: 'PerfHarness-Local-2026',
    AUTH_SECURE_COOKIES: 'false',
    AUTH_RATE_LIMIT_ENABLED: 'false',
    ROOM_ACCESS_TOKEN_SECRET: 'benchmark-room-access-secret-32-bytes',
    NETEASE_API_URL: 'http://127.0.0.1:9',
    YOUTUBE_ENABLED: 'false',
    NAVIDROME_ENABLED: 'false',
    SQUIDIFY_ENABLED: 'false',
    LOG_LEVEL: 'WARN'
  },
  stdio: ['ignore', 'pipe', 'pipe']
});
child.stdout.on('data', chunk => stdout.push(chunk.toString()));
child.stderr.on('data', chunk => stderr.push(chunk.toString()));

const delay = milliseconds => new Promise(resolveDelay => setTimeout(resolveDelay, milliseconds));
const percentile = (values, quantile) => {
  const sorted = [...values].sort((a, b) => a - b);
  return Number(sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * quantile) - 1)].toFixed(2));
};

const startedAt = performance.now();
try {
  let startupMs;
  const deadline = startedAt + 60_000;
  while (performance.now() < deadline) {
    if (child.exitCode != null) throw new Error(`Go exited during startup with code ${child.exitCode}`);
    try {
      const response = await fetch(`${baseUrl}/actuator/health`);
      if (response.ok) {
        startupMs = performance.now() - startedAt;
        break;
      }
    } catch {}
    await delay(50);
  }
  if (startupMs == null) throw new Error('Go health endpoint did not become ready within 60 seconds');

  for (let index = 0; index < 10; index++) await fetch(`${baseUrl}/actuator/health`);
  const latencies = [];
  for (let index = 0; index < healthSamples; index++) {
    const requestStartedAt = performance.now();
    const response = await fetch(`${baseUrl}/actuator/health`);
    if (!response.ok) throw new Error(`Health benchmark returned HTTP ${response.status}`);
    await response.arrayBuffer();
    latencies.push(performance.now() - requestStartedAt);
  }

  const { stdout: rssOutput } = await execFileAsync('powershell.exe', ['-NoProfile', '-Command', `(Get-Process -Id ${child.pid}).WorkingSet64`]);
  const websocket = [];
  for (const count of websocketCounts) {
    const { stdout: loadOutput } = await execFileAsync(process.execPath, [
      resolve(repositoryRoot, 'scripts/websocket-auth-load.mjs'), String(count), baseUrl, origin
    ], {
      cwd: repositoryRoot,
      env: { ...process.env, BOOTSTRAP_ADMIN_USERNAME: 'perf-admin', BOOTSTRAP_ADMIN_PASSWORD: 'PerfHarness-Local-2026' },
      maxBuffer: 1024 * 1024
    });
    websocket.push(JSON.parse(loadOutput.trim()));
  }

  console.log(JSON.stringify({
    implementation: 'go',
    startupMs: Number(startupMs.toFixed(2)),
    rssBytes: Number(rssOutput.trim()),
    health: { samples: latencies.length, p50Ms: percentile(latencies, 0.5), p95Ms: percentile(latencies, 0.95), maxMs: Number(Math.max(...latencies).toFixed(2)) },
    websocket
  }, null, 2));
} catch (error) {
  console.error(`${error.stack ?? error}\n${[...stdout, ...stderr].join('').split(/\r?\n/).slice(-40).join('\n')}`);
  process.exitCode = 1;
} finally {
  child.kill();
  await Promise.race([new Promise(resolveExit => child.once('exit', resolveExit)), delay(5_000)]);
  await rm(runtimeRoot, { recursive: true, force: true });
}
