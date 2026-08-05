export const withPlaybackToken = (music) => {
  const url = music?.url || '';
  return url;
};

export const withNavidromeResourceToken = (url) => {
  if (!url || typeof url !== 'string') return '';
  return url;
};
