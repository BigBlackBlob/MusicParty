export const withPlaybackToken = (music, sessionToken) => {
  const url = music?.url || '';
  if (!url || !requiresResourceToken(music?.platform, url)) return url;
  if (!sessionToken) return '';
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}token=${encodeURIComponent(sessionToken)}`;
};

export const withNavidromeResourceToken = (url, sessionToken) => {
  if (!url || typeof url !== 'string') return '';
  if (!url.startsWith('/api/navidrome/') && !url.startsWith('/api/subsonic/')) return url;
  if (!sessionToken) return '';
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}token=${encodeURIComponent(sessionToken)}`;
};

export const isSubsonicPlatform = (platform) => platform === 'navidrome' || String(platform || '').startsWith('subsonic-');

const requiresResourceToken = (platform, url) => (
  isSubsonicPlatform(platform)
  || platform === 'netease'
  || String(url || '').startsWith('/api/navidrome/')
  || String(url || '').startsWith('/api/subsonic/')
  || String(url || '').startsWith('/api/netease/')
);
