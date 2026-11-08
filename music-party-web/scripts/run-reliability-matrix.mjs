import { spawn, spawnSync } from 'node:child_process'
import { createServer } from 'node:net'
import { createWriteStream } from 'node:fs'
import { mkdir, writeFile } from 'node:fs/promises'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const repoRoot = resolve(webRoot, '..')
const backendRoot = join(repoRoot, 'backend-go')
const runId = new Date().toISOString().replaceAll(':', '-').replaceAll('.', '-')
const startedAt = new Date().toISOString()
const runDir = join(webRoot, 'test-results', 'reliability-matrix', runId)
const counts = readList('RELIABILITY_CLIENT_COUNTS', [2, 3, 5], value => value >= 2 && value <= 5)
const sizes = readList('RELIABILITY_QUEUE_SIZES', [3, 10, 30], value => [3, 10, 30].includes(value))
const repeats = readPositiveInteger('RELIABILITY_REPEATS', 30)
const grep = process.env.RELIABILITY_GREP || 'desktop queue real-mouse drag baseline'
const matrixRun = grep === 'desktop queue real-mouse drag baseline'
const repeatEach = readPositiveInteger('RELIABILITY_REPEAT_EACH', 1)
const username = process.env.E2E_ADMIN_USERNAME || 'e2e-admin'
const password = process.env.E2E_ADMIN_PASSWORD || 'E2E-Password-2026!'

function readList(name, fallback, isValid) {
  const values = (process.env[name] || fallback.join(',')).split(',').map(Number)
  if (!values.length || values.some(value => !Number.isInteger(value) || !isValid(value))) {
    throw new Error(`${name} must be a comma-separated list of supported positive integers`)
  }
  return [...new Set(values)]
}

function readPositiveInteger(name, fallback) {
  const value = Number(process.env[name] || fallback)
  if (!Number.isInteger(value) || value < 1) throw new Error(`${name} must be a positive integer`)
  return value
}

function run(command, args, cwd) {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit', windowsHide: true })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} failed with exit code ${result.status}`)
}

function runStreaming(command, args, cwd, env, outputPath) {
  return new Promise((resolveRun, rejectRun) => {
    const child = spawn(command, args, { cwd, env, stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true })
    const output = createWriteStream(outputPath)
    let captured = ''
    const capture = chunk => {
      const text = chunk.toString()
      captured += text
      process.stdout.write(text)
      output.write(chunk)
    }
    child.stdout.on('data', capture)
    child.stderr.on('data', chunk => {
      const text = chunk.toString()
      captured += text
      process.stderr.write(text)
      output.write(chunk)
    })
    child.once('error', error => rejectRun(error))
    child.once('close', (status, signal) => {
      output.end()
      resolveRun({ status: status ?? (signal ? 1 : 0), captured })
    })
  })
}

async function unusedLoopbackPort() {
  const listener = createServer()
  await new Promise((resolveListen, reject) => {
    listener.once('error', reject)
    listener.listen(0, '127.0.0.1', resolveListen)
  })
  const address = listener.address()
  if (!address || typeof address === 'string') throw new Error('Could not allocate a local test port')
  await new Promise((resolveClose, reject) => listener.close(error => error ? reject(error) : resolveClose()))
  return address.port
}

async function waitUntilReady(server, url, logPath) {
  const deadline = Date.now() + 90_000
  while (Date.now() < deadline) {
    if (server.exitCode !== null) throw new Error(`Isolated server exited early; see ${logPath}`)
    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(1_500) })
      if (response.ok) return
    } catch {
      await new Promise(resolveDelay => setTimeout(resolveDelay, 300))
    }
  }
  throw new Error(`Isolated server did not become ready; see ${logPath}`)
}

function quantile(values, percentile) {
  if (!values.length) return null
  return values[Math.max(0, Math.ceil(values.length * percentile) - 1)]
}

await mkdir(runDir, { recursive: true })
const port = await unusedLoopbackPort()
const baseUrl = `http://127.0.0.1:${port}`
const serverPath = join(runDir, process.platform === 'win32' ? 'musicparty.exe' : 'musicparty')
const serverLogPath = join(runDir, 'server.log')
const serverErrorPath = join(runDir, 'server-error.log')
const serverOut = createWriteStream(serverLogPath)
const serverErr = createWriteStream(serverErrorPath)
let server
let testResult
let thrown

try {
  console.log('Building frontend for the isolated reliability run')
  run(process.execPath, [join(webRoot, 'node_modules', 'vite', 'bin', 'vite.js'), 'build'], webRoot)
  console.log('Building Go server for the isolated reliability run')
  run('go', ['build', '-trimpath', '-o', serverPath, './cmd/musicparty'], backendRoot)

  const serverEnv = {
    ...process.env,
    SERVER_PORT: String(port),
    APP_MODE: 'server',
    BASE_URL: baseUrl,
    ALLOWED_ORIGINS: baseUrl,
    DB_ENABLED: 'true',
    DB_INIT_SCHEMA: 'true',
    DB_PATH: join(runDir, 'reliability.sqlite'),
    LOCAL_LIBRARY_PATH: join(runDir, 'local-library'),
    STATIC_PATH: join(webRoot, 'dist'),
    BOOTSTRAP_ADMIN_USERNAME: username,
    BOOTSTRAP_ADMIN_PASSWORD: password,
    AUTH_SECURE_COOKIES: 'false',
    AUTH_RATE_LIMIT_ENABLED: 'false',
    ROOM_ACCESS_TOKEN_SECRET: 'reliability-isolated-room-secret-32-bytes',
    NETEASE_API_URL: 'http://127.0.0.1:9',
    YOUTUBE_ENABLED: 'false',
    NAVIDROME_ENABLED: 'false',
    SQUIDIFY_ENABLED: 'false'
  }
  server = spawn(serverPath, [], { cwd: backendRoot, env: serverEnv, stdio: ['ignore', 'pipe', 'pipe'], windowsHide: true })
  server.stdout.pipe(serverOut)
  server.stderr.pipe(serverErr)
  console.log(`Waiting for isolated server on ${baseUrl}`)
  await waitUntilReady(server, `${baseUrl}/actuator/health/readiness`, serverErrorPath)
  console.log('Isolated server is ready')

  const cli = join(webRoot, 'node_modules', '@playwright', 'test', 'cli.js')
  const args = [cli, 'test', 'e2e/reliability-drag.spec.ts', '--grep', grep, '--reporter=list', '--output', join(runDir, 'playwright-artifacts')]
  if (repeatEach > 1) args.push(`--repeat-each=${repeatEach}`)
  console.log(matrixRun
    ? `Starting Playwright matrix: ${counts.length} client sizes x ${sizes.length} queue sizes x ${repeats} repetitions`
    : `Starting Playwright scenarios: ${grep} x ${repeatEach}`)
  testResult = await runStreaming(process.execPath, args, webRoot, {
      ...serverEnv,
      E2E_BASE_URL: baseUrl,
      RELIABILITY_PROBE: '1',
      DRAG_CLIENT_COUNTS: counts.join(','),
      DRAG_SIZES: sizes.join(','),
      DRAG_REPEATS: String(repeats),
      RELIABILITY_PROGRESS_FILE: join(runDir, 'progress.log'),
      DEBUG: process.env.RELIABILITY_DEBUG === '1' ? 'pw:api' : process.env.DEBUG
    }, join(runDir, 'playwright.log'))
} catch (error) {
  thrown = error
} finally {
  if (server && server.exitCode === null) {
    server.kill()
    await new Promise(resolveExit => {
      const timeout = setTimeout(resolveExit, 5_000)
      server.once('exit', () => { clearTimeout(timeout); resolveExit() })
    })
  }
  serverOut.end()
  serverErr.end()
}

const output = testResult?.captured || ''
const samples = [...output.matchAll(/^@@RELIABILITY_SAMPLE (.+)$/gm)].map(([, json]) => JSON.parse(json))
const integrity = [...output.matchAll(/^@@RELIABILITY_INTEGRITY (.+)$/gm)].map(([, json]) => JSON.parse(json))
const scenarioSamples = [...output.matchAll(/^@@RELIABILITY_SCENARIO (.+)$/gm)].map(([, json]) => JSON.parse(json))
const tcpSamples = [...output.matchAll(/^@@RELIABILITY_TCP (.+)$/gm)].map(([, json]) => JSON.parse(json))
const audioSamples = [...output.matchAll(/^@@RELIABILITY_AUDIO (.+)$/gm)].map(([, json]) => JSON.parse(json))
const scopeSamples = [...output.matchAll(/^@@RELIABILITY_SCOPE (.+)$/gm)].map(([, json]) => JSON.parse(json))
const groups = new Map()
for (const sample of samples) {
  const key = `${sample.clients} clients / ${sample.size} tracks / ${sample.scenario}`
  groups.set(key, [...(groups.get(key) ?? []), sample])
}
const summaries = [...groups].map(([group, entries]) => {
  const successful = entries.filter(sample => sample.ok).map(sample => sample.latencyMs).sort((a, b) => a - b)
  return {
    group,
    attempts: entries.length,
    successful: entries.filter(sample => sample.ok).length,
    successRate: entries.filter(sample => sample.ok).length / entries.length,
    latencyMs: { p50: quantile(successful, 0.5), p95: quantile(successful, 0.95), max: successful.at(-1) ?? null }
  }
})
const scenarioDefinitions = [
  { scenario: 'auto-scroll', testName: 'desktop queue auto-scroll drag converges', cases: counts.length },
  { scenario: 'concurrent-enqueue-remove', testName: 'desktop queue drag converges with concurrent enqueue and remove', cases: counts.length * sizes.length },
  { scenario: 'dropped-reorder-ack', testName: 'desktop queue recovers after a single dropped reorder ACK', cases: counts.length * sizes.length },
  { scenario: 'websocket-disconnect', testName: 'desktop queue recovers from a real WebSocket disconnect', cases: counts.length }
]
const selectedScenarios = scenarioDefinitions.filter(item => new RegExp(grep).test(item.testName))
const expectedScenarioSamples = selectedScenarios.reduce((sum, item) => sum + item.cases * repeatEach, 0)
const tcpRun = /TCP silent half-open/.test(grep)
const expectedTcpSamples = tcpRun ? repeatEach : 0
const audioRun = /local WAV playback alignment and websocket frame cost/.test(grep)
const expectedAudioSamples = audioRun ? repeatEach : 0
const scopeRun = /pending reorder does not cross room scope/.test(grep)
const expectedScopeSamples = scopeRun ? repeatEach : 0
const scenarioGroups = new Map()
for (const sample of scenarioSamples) {
  const key = `${sample.scenario} / ${sample.clients} clients / ${sample.size} tracks`
  scenarioGroups.set(key, [...(scenarioGroups.get(key) ?? []), sample])
}
const scenarioSummaries = [...scenarioGroups].map(([group, entries]) => {
  const latencies = entries.map(sample => sample.latencyMs).sort((a, b) => a - b)
  return {
    group,
    attempts: entries.length,
    latencyMs: { p50: quantile(latencies, 0.5), p95: quantile(latencies, 0.95), max: latencies.at(-1) ?? null },
    violations: entries.filter(sample => sample.latencyMs > (sample.scenario === 'websocket-disconnect' ? 5_000 : 2_000)
      || (sample.scenario === 'auto-scroll' && (sample.sent !== 1 || sample.ack !== 1 || !sample.scrolled))
      || (sample.scenario === 'concurrent-enqueue-remove' && (sample.added !== 1 || sample.removed !== 1 || sample.duplicates !== 0))
      || (sample.scenario === 'dropped-reorder-ack' && (sample.sent !== 2 || sample.droppedAcks !== 1 || sample.versionDelta !== 1))).length
  }
})
const scenarioSamplesCorrect = scenarioSamples.length === expectedScenarioSamples
  && scenarioSamples.every(sample => sample.latencyMs <= (sample.scenario === 'websocket-disconnect' ? 5_000 : 2_000)
    && (sample.scenario !== 'auto-scroll' || (sample.sent === 1 && sample.ack === 1 && sample.scrolled))
    && (sample.scenario !== 'concurrent-enqueue-remove' || (sample.added === 1 && sample.removed === 1 && sample.duplicates === 0))
    && (sample.scenario !== 'dropped-reorder-ack' || (sample.sent === 2 && sample.droppedAcks === 1 && sample.versionDelta === 1)))
const report = {
  runId,
  startedAt,
  baseUrl,
  isolated: true,
  gitHead: spawnSync('git', ['rev-parse', 'HEAD'], { cwd: repoRoot, encoding: 'utf8' }).stdout.trim(),
  matrix: { clients: counts, queueSizes: sizes, repeatsPerScenario: repeats, scenarios: ['empty-then-add', 'clear-then-readd', 'mounted-with-queue'] },
  scenarioSelector: matrixRun ? null : grep,
  repeatEach,
  expectedSamples: matrixRun ? counts.length * sizes.length * repeats * 3 : expectedScenarioSamples + expectedTcpSamples + expectedAudioSamples + expectedScopeSamples,
  capturedSamples: samples.length,
  expectedScenarioSamples,
  capturedScenarioSamples: scenarioSamples.length,
  expectedTcpSamples,
  capturedTcpSamples: tcpSamples.length,
  expectedAudioSamples,
  capturedAudioSamples: audioSamples.length,
  expectedScopeSamples,
  capturedScopeSamples: scopeSamples.length,
  correct: matrixRun
    ? samples.length === counts.length * sizes.length * repeats * 3
      && samples.every(sample => sample.ok && sample.latencyMs <= 2_000 && sample.sent === 1 && sample.ack === 'ack')
      && integrity.length === counts.length * sizes.length
      && integrity.every(item => item.matchesFreshSnapshot && item.duplicates === 0)
    : audioRun
      ? (testResult?.status === 0 && !thrown && audioSamples.length === expectedAudioSamples
        && audioSamples.every(sample => Number.isFinite(sample.mediaDriftMs) && sample.clientFrames > 0 && sample.clientEnvelopeBytes > 0))
    : scopeRun
      ? (testResult?.status === 0 && !thrown && scopeSamples.length === expectedScopeSamples
        && scopeSamples.every(sample => sample.droppedAck && sample.pendingAfterSwitch === 0 && !sample.replayedAcrossScope))
    : tcpRun
      ? (testResult?.status === 0 && !thrown && tcpSamples.length === expectedTcpSamples && tcpSamples.every(sample => sample.recoveryMs <= 5_000 && sample.droppedServerBytes > 0))
      : selectedScenarios.length > 0
      ? (testResult?.status === 0 && !thrown && scenarioSamplesCorrect)
      : (testResult?.status === 0 && !thrown),
  exitCode: thrown ? 1 : (testResult?.status ?? 1),
  summaries,
  scenarioSummaries,
  integrity,
  samples,
  scenarioSamples,
  tcpSamples,
  audioSamples,
  scopeSamples,
  serverDatabase: join(runDir, 'reliability.sqlite'),
  serverLog: serverLogPath,
  serverErrorLog: serverErrorPath
}
// An incomplete or invalid sample set must fail even if the test process exits 0.
if (!report.correct && report.exitCode === 0) report.exitCode = 1
await writeFile(join(runDir, 'report.json'), `${JSON.stringify(report, null, 2)}\n`)
console.log(`Reliability report: ${join(runDir, 'report.json')}`)
const capturedCount = matrixRun ? samples.length : scenarioSamples.length + tcpSamples.length + audioSamples.length + scopeSamples.length
console.log(`Samples: ${capturedCount}/${report.expectedSamples}; correctness=${report.correct}`)
if (thrown) throw thrown
if (report.exitCode !== 0) process.exitCode = report.exitCode
