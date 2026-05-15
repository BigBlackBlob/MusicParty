import { defineConfig } from 'vitest/config';
import { mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const rootDir = dirname(fileURLToPath(import.meta.url));
const testTempDir = resolve(rootDir, '.tmp');

mkdirSync(testTempDir, { recursive: true });
process.env.TMPDIR = testTempDir;
process.env.TEMP = testTempDir;
process.env.TMP = testTempDir;

export default defineConfig({
  cacheDir: './.vite-test',
  test: {
    environment: 'happy-dom',
    globals: false
  }
});
