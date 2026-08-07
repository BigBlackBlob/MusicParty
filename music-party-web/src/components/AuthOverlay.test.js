import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = readFileSync(resolve(process.cwd(), 'src/components/AuthOverlay.vue'), 'utf8');

describe('AuthOverlay guest/admin entry', () => {
  it('offers direct guest entry and administrator login only', () => {
    expect(source).toContain('createGuestSession');
    expect(source).toContain('loginAdmin');
    expect(source).not.toContain('redeemInvite');
    expect(source).not.toContain('secretFromLocation');
    expect(source).toContain('loginAccount');
    expect(source).toContain('平台管理员登录');
  });
});
