import client from './client';

const base = (roomId) => `/api/rooms/${encodeURIComponent(roomId || 'lounge')}/playlists`;

export const playlistsApi = {
  list(roomId) {
    return client.get(base(roomId));
  },
  create(roomId, name) {
    return client.post(base(roomId), { name });
  },
  rename(roomId, playlistId, name) {
    return client.patch(`${base(roomId)}/${encodeURIComponent(playlistId)}`, { name });
  },
  remove(roomId, playlistId) {
    return client.delete(`${base(roomId)}/${encodeURIComponent(playlistId)}`);
  },
  tracks(roomId, playlistId, offset = 0, limit = 100) {
    return client.get(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks`, { params: { offset, limit } });
  },
  addTrack(roomId, playlistId, music) {
    return client.post(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks`, { music });
  },
  removeTrack(roomId, playlistId, trackId) {
    return client.delete(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks/${encodeURIComponent(trackId)}`);
  },
  reorder(roomId, playlistId, trackIds) {
    return client.post(`${base(roomId)}/${encodeURIComponent(playlistId)}/tracks/reorder`, { trackIds });
  },
  importExternal(roomId, playlistId, platform, externalPlaylistId, token) {
    return client.post(`${base(roomId)}/${encodeURIComponent(playlistId)}/import`, { platform, playlistId: externalPlaylistId }, { params: { token } });
  }
};
