import client from './client';

const needsToken = (platform) => platform === 'navidrome' || String(platform || '').startsWith('subsonic-');

export const musicApi = {
    // 获取当前用户可用平台
    getPlatforms: (sessionToken, roomId) => client.get('/api/platforms', { params: { token: sessionToken, roomId } }),

    // 搜索歌曲
    search: (platform, keyword, sessionToken, offset = 0, limit = 20, roomId) => client.get(
        `/api/search/${platform}/${keyword}`,
        {
            params: {
                offset,
                limit,
                roomId,
                ...(needsToken(platform) ? { token: sessionToken } : {})
            }
        }
    ),

    // 搜索平台专辑
    searchAlbums: (platform, keyword, sessionToken, roomId) => client.get(`/api/album/search/${platform}`, {
        params: {
            keyword,
            roomId,
            ...(needsToken(platform) ? { token: sessionToken } : {})
        }
    }),

    // 获取平台专辑歌曲
    getAlbumSongs: (platform, albumId, sessionToken, roomId) => client.get(`/api/album/songs/${platform}/${albumId}`, {
        params: {
            roomId,
            ...(needsToken(platform) ? { token: sessionToken } : {})
        }
    }),

    // 获取歌词
    getLyric: (platform, songId) => client.get(`/api/music/lyric/${platform}/${songId}`),
    getLyricDetail: (platform, songId) => client.get(`/api/music/lyric-detail/${platform}/${songId}`),

    // 获取用户歌单
    getUserPlaylists: (platform, userId) => client.get(`/api/user/playlists/${platform}/${userId}`),

    // 获取歌单详情 (分页)
    getPlaylistSongs: (platform, playlistId, offset, limit) =>
        client.get(`/api/playlist/songs/${platform}/${playlistId}`, { params: { offset, limit } }),

    // 搜索用户
    searchUser: (platform, keyword) => client.get(`/api/user/search/${platform}/${keyword}`),

    extractCoverColor: (url) => client.get('/api/theme/extract-cover-color', { params: { url } })
};
