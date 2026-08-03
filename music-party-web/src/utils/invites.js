export const getInviteStatus = (invite, now = Date.now()) => {
    if (invite?.usedAt != null) return 'used';
    if (invite?.revokedAt != null) return 'revoked';
    if (invite?.permanent) return 'permanent';
    if (Number(invite?.expiresAt) < now) return 'expired';
    return 'available';
};

export const buildInviteUrl = (secret, origin = window.location.origin) => (
    new URL(`/join/${encodeURIComponent(secret)}`, origin).toString()
);
