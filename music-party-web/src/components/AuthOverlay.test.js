import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = readFileSync(resolve(process.cwd(), 'src/components/AuthOverlay.vue'), 'utf8');

describe('AuthOverlay registration entry', () => {
  it('exposes a non-bootstrap registration mode for regular visitors', () => {
    expect(source).toContain("authMode = ref('login')");
    expect(source).toContain('auth-mode-toggle');
    expect(source).toContain("authMode.value = authMode.value === 'login' ? 'register' : 'login'");
    expect(source).toContain("authMode.value === 'register'");
    expect(source).toContain('authApi.registerAccount');
  });
});
