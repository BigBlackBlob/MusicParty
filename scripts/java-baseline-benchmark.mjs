#!/usr/bin/env node

import { spawn, execFile } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { performance } from 'node:perf_hooks';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);
const port = Number(process.env.BASELINE_PORT ?? 18080);
const healthSamples = Number(process.env.BASELINE_HTTP_SAMPLES ?? 200);
const websocketCounts = (process.env.BASELINE_WS_COUNTS ?? '10,100')
  .split(',')
  .map(value => Number(value.trim()))
  .filter(value => Number.isInteger(value) && value > 0);
const keepRuntime = process.argv.includes('--keep-runtime');
const jar = resolve(process.env.BASELINE_JAR ?? 'target/MusicParty-0.0.1-SNAPSHOT.jar');
const origin = process.env.BASELINE_ORIGIN ?? 'http://127.0.0.1:15173';
const baseUrl = `http://127.0.0.1:${port}`;
const runtimeRoot = await mkdtemp(join(tmpdir(), 'musicparty-java-baseline-'));
const stdout = [];
const stderr = [];

const child = spawn('java', ['-jar', jar], {
  cwd: resolve('.'),
  windowsHide: true,
  env: {
    ...process.env,
    SERVER_PORT: String(port),
    DB_PATH: join(runtimeRoot, 'musicparty.db'),
    BOOTSTRAP_ADMIN_USERNAME: process.env.BOOTSTRAP_ADMIN_USERNAME ?? 'perf-admin',
    BOOTSTRAP_ADMIN_PASSWORD: process.env.BOOTSTRAP_ADMIN_PASSWORD ?? 'PerfHarness-Local-2026',
    AUTH_SECURE_COOKIES: 'false',
    BASE_URL: baseUrl,
    ALLOWED_ORIGINS: origin,
    LOCAL_LIBRARY_PATH: join(runtimeRoot, 'local-library'),
    MUSICPARTY_PROFILE_DIR: join(runtimeRoot, 'desktop-profile'),
    SQUIDIFY_ENABLED: 'false',
    YOUTUBE_ENABLED: 'false'
  },
  stdio: ['ignore', 'pipe', 'pipe']
});

child.stdout.on('data', chunk => stdout.push(chunk.toString()));
child.stderr.on('data', chunk => stderr.push(chunk.toString()));

const delay = milliseconds => new Promise(resolveDelay => setTimeout(resolveDelay, milliseconds));
const percentile = (values, quantile) => {
  const sorted = [...values].sort((left, right) => left - right);
  return Number(sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * quantile) - 1)].toFixed(2));
};

const startedAt = performance.now();
let startupMs;
try {
  const deadline = startedAt + 60_000;
  while (performance.now() < deadline) {
    if (child.exitCode != null) throw new Error(`Java exited during startup with code ${child.exitCode}`);
    try {
      const response = await fetch(`${baseUrl}/actuator/health`);
      if (response.ok) {
        startupMs = performance.now() - startedAt;
        break;
      }
    } catch {
      // Expected until the listening socket is ready.
    }
    await delay(100);
  }
  if (startupMs == null) throw new Error('Java health endpoint did not become ready within 60 seconds');

  for (let index = 0; index < 10; index++) await fetch(`${baseUrl}/actuator/health`);
  const latencies = [];
  for (let index = 0; index < healthSamples; index++) {
    const requestStartedAt = performance.now();
    const response = await fetch(`${baseUrl}/actuator/health`);
    if (!response.ok) throw new Error(`Health benchmark returned HTTP ${response.status}`);
    await response.arrayBuffer();
    latencies.push(performance.now() - requestStartedAt);
  }

  let rssBytes = null;
  if (process.platform === 'win32') {
    const { stdout: rssOutput } = await execFileAsync('powershell.exe', [
      '-NoProfile', '-Command', `(Get-Process -Id ${child.pid}).WorkingSet64`
    ]);
    rssBytes = Number(rssOutput.trim());
  } else {
    const { stdout: rssOutput } = await execFileAsync('ps', ['-o', 'rss=', '-p', String(child.pid)]);
    rssBytes = Number(rssOutput.trim()) * 1024;
  }

  const websocket = [];
  for (const count of websocketCounts) {
    const { stdout: loadOutput } = await execFileAsync(process.execPath, [
      resolve('scripts/websocket-auth-load.mjs'),
      String(count),
      baseUrl,
      origin
    ], {
      cwd: resolve('.'),
      env: {
        ...process.env,
        BOOTSTRAP_ADMIN_USERNAME: process.env.BOOTSTRAP_ADMIN_USERNAME ?? 'perf-admin',
        BOOTSTRAP_ADMIN_PASSWORD: process.env.BOOTSTRAP_ADMIN_PASSWORD ?? 'PerfHarness-Local-2026'
      },
      maxBuffer: 1024 * 1024
    });
    websocket.push(JSON.parse(loadOutput.trim()));
  }

  console.log(JSON.stringify({
    implementation: 'java',
    pid: child.pid,
    runtimeRoot,
    startupMs: Number(startupMs.toFixed(2)),
    rssBytes,
    health: {
      samples: latencies.length,
      p50Ms: percentile(latencies, 0.5),
      p95Ms: percentile(latencies, 0.95),
      maxMs: Number(Math.max(...latencies).toFixed(2))
    },
    websocket
  }, null, 2));
} catch (error) {
  const recentOutput = [...stdout, ...stderr].join('').split(/\r?\n/).slice(-40).join('\n');
  console.error(`${error.stack ?? error}\n${recentOutput}`);
  process.exitCode = 1;
} finally {
  child.kill();
  await Promise.race([
    new Promise(resolveExit => child.once('exit', resolveExit)),
    delay(5_000)
  ]);
  if (!keepRuntime) await rm(runtimeRoot, { recursive: true, force: true });
}
