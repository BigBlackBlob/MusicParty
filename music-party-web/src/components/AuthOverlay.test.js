import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const source = readFileSync(resolve(process.cwd(), 'src/components/AuthOverlay.vue'), 'utf8');

describe('AuthOverlay invitation entry', () => {
  it('keeps invitation membership and platform-admin login separate', () => {
    expect(source).toContain('redeemInvite');
    expect(source).toContain('secretFromLocation');
    expect(source).not.toContain('registerAccount');
    expect(source).toContain('loginAccount');
    expect(source).toContain('平台管理员登录');
  });
});
