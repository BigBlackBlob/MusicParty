export const withPlaybackToken = (music, sessionToken) => {
  const url = music?.url || '';
  if (!url || music?.platform !== 'navidrome') return url;
  if (!sessionToken) return '';
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}token=${encodeURIComponent(sessionToken)}`;
};

export const withNavidromeResourceToken = (url, sessionToken) => {
  if (!url || typeof url !== 'string') return '';
  if (!url.startsWith('/api/navidrome/')) return url;
  if (!sessionToken) return '';
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}token=${encodeURIComponent(sessionToken)}`;
};
