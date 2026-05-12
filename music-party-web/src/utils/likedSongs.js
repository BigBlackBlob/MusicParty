const getArtistsText = (song) => {
    if (Array.isArray(song?.artists) && song.artists.length) {
        return song.artists.join(' / ');
    }
    return 'Unknown Artist';
};

const getSongTitle = (song) => song?.name || 'Unknown Title';

export const getSongExternalUrl = (song) => {
    if (!song?.platform || !song?.id) return '';
    if (song.platform === 'bilibili') {
        return `https://www.bilibili.com/video/${song.id}`;
    }
    if (song.platform === 'netease') {
        return `https://music.163.com/#/song?id=${song.id}`;
    }
    return '';
};

export const formatLikedSongLine = (song) => {
    const base = `${getArtistsText(song)} - ${getSongTitle(song)}`;
    const externalUrl = getSongExternalUrl(song);
    return externalUrl ? `${base} - ${externalUrl}` : base;
};

export const createLikedSongsText = (songs) => {
    if (!Array.isArray(songs)) return '';
    return songs.map(formatLikedSongLine).join('\n');
};

export const createLikedSongsFilename = (date = new Date()) => (
    `musicparty-liked-songs-${date.toISOString().slice(0, 10)}.txt`
);
