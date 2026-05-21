import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = readFileSync(resolve(process.cwd(), 'src/composables/useNowPlayingViewModel.js'), 'utf8');

describe('Now playing seek permissions', () => {
  it('uses account role instead of ADMIN display name for admin seek permission', () => {
    expect(source).toContain('user.isAdmin');
    expect(source).not.toContain("user.currentUser.name === 'ADMIN'");
  });
});
