import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = readFileSync(resolve(process.cwd(), 'src/composables/useNowPlayingViewModel.ts'), 'utf8');

describe('Now playing seek permissions', () => {
  it('uses room capability instead of a role or display-name alias for seek permission', () => {
    expect(source).toContain('user.capabilities.canManageCurrentRoom');
    expect(source).not.toContain('AUTO_DJ');
    expect(source).not.toContain("currentUser.name");
  });
});
