import { describe, expect, it } from 'vitest';
import { buildInviteUrl, getInviteStatus } from './invites';

describe('invite helpers', () => {
    it('prioritizes used over revoked', () => {
        expect(getInviteStatus({ usedAt: 10, revokedAt: 11, permanent: true })).toBe('used');
    });

    it('recognizes permanent and expired invites', () => {
        expect(getInviteStatus({ permanent: true, expiresAt: 1 }, 100)).toBe('permanent');
        expect(getInviteStatus({ expiresAt: 1 }, 100)).toBe('expired');
    });

    it('builds an encoded invite URL from the current origin', () => {
        expect(buildInviteUrl('a/b?c', 'https://music.example')).toBe('https://music.example/join/a%2Fb%3Fc');
    });
});
