import client from './client';

const base = (roomId) => `/api/rooms/${encodeURIComponent(roomId || 'lounge')}/playlists`;

const authParams = (auth) => ({
    sessionToken: auth?.sessionToken || '',
    roomAccessToken: auth?.roomAccessToken || ''
});

export const playlistsApi = {
  list(roomId) {
    return client.get(base(roomId));
  },
  create(roomId, name, auth = {}) {
    return client.post(base(roomId), { name }, { params: authParams(auth) });
  },
  rename(roomId, playlistId, name, auth = {}) {
    return client.patch(`${base(roomId)}/${encodeURIComponent(playlistId)}`, { name }, { params: authParams(auth) });
  },
  remove(roomId, playlistId, auth = {}) {
    return client.delete(`${base(roomId)}/${encodeURIComponent(playlistId)}`, { params: authParams(auth) });
  },
  tracks(roomId, playlistId, offset = 0, limit = 100) {
    return client.get(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks`, { params: { offset, limit } });
  },
  addTrack(roomId, playlistId, music, auth = {}) {
    return client.post(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks`, { music }, { params: authParams(auth) });
  },
  removeTrack(roomId, playlistId, trackId, auth = {}) {
    return client.delete(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks/${encodeURIComponent(trackId)}`, { params: authParams(auth) });
  },
  reorder(roomId, playlistId, trackIds, auth = {}) {
    return client.post(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks/reorder`, { trackIds }, { params: authParams(auth) });
  },
  importExternal(roomId, playlistId, platform, externalPlaylistId, token, auth = {}) {
    return client.post(
      `${base(roomId)}/${encodeURIComponent(playlistId)}/import`,
      { platform, playlistId: externalPlaylistId },
      { params: { token: token || '', ...authParams(auth) } }
    );
  },
  exportPlaylist(roomId, playlistId, format = 'txt') {
    return client.get(`${base(roomId)}/${encodeURIComponent(playlistId)}/export`, { params: { format } });
  }
};
