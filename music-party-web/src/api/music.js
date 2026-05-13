import client from './client';

export const musicApi = {
    // 获取当前用户可用平台
    getPlatforms: (token) => client.get('/api/platforms', { params: { token } }),

    // 搜索歌曲
    search: (platform, keyword, token) => client.get(
        `/api/search/${platform}/${keyword}`,
        { params: platform === 'navidrome' ? { token } : {} }
    ),

    // 搜索网易云专辑
    searchNeteaseAlbums: (keyword) => client.get('/api/album/search/netease', { params: { keyword } }),

    // 获取网易云专辑歌曲
    getNeteaseAlbumSongs: (albumId) => client.get(`/api/album/songs/netease/${albumId}`),

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
