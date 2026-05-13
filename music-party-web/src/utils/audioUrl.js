export const withPlaybackToken = (music, userToken) => {
  const url = music?.url || '';
  if (!url || music?.platform !== 'navidrome') return url;
  if (!userToken) return '';
  const sep = url.includes('?') ? '&' : '?';
  return `${url}${sep}token=${encodeURIComponent(userToken)}`;
};
