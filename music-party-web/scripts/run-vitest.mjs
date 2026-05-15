import { mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';

const rootDir = dirname(dirname(fileURLToPath(import.meta.url)));
const tempDir = resolve(rootDir, '.tmp');
const vitestBin = resolve(rootDir, 'node_modules', 'vitest', 'vitest.mjs');

mkdirSync(tempDir, { recursive: true });

const child = spawn(process.execPath, [vitestBin, ...process.argv.slice(2)], {
  cwd: rootDir,
  stdio: 'inherit',
  env: {
    ...process.env,
    TMPDIR: tempDir,
    TEMP: tempDir,
    TMP: tempDir
  }
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }

  process.exit(code ?? 1);
});
