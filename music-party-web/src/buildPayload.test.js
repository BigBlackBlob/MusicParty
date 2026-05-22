import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = readFileSync(resolve(process.cwd(), 'vite.config.js'), 'utf8');

describe('frontend payload chunking', () => {
  it('splits large third-party dependencies out of the entry chunk', () => {
    expect(source).toContain('manualChunks');
    expect(source).toContain('vendor-vue');
    expect(source).toContain('vendor-ui');
    expect(source).toContain('vendor-network');
    expect(source).toContain('vendor-dnd');
  });
});
