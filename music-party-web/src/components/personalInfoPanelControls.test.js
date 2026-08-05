import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const readSource = (path) => readFileSync(resolve(process.cwd(), path), 'utf8');

describe('personal info account controls', () => {
  it('keeps the logout action styled as a visible button in both themes', () => {
    const source = readSource('src/components/PersonalInfoPanel.vue');

    expect(source).toContain("class=\"personal-info__secondary\"");
    expect(source).toContain("'personal-info__secondary--danger': confirmingLogout");
    expect(source).toContain('border-color: var(--border-default);');
    expect(source).toContain('.personal-info__actions > .personal-info__secondary');
    expect(source).toContain('background: var(--surface-2);');
    expect(source).toContain('color: var(--text-primary);');
    expect(source).toContain('.personal-info__secondary--danger');
  });
});
