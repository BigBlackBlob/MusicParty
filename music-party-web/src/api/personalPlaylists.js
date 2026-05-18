import client from './client';

const base = '/api/me/playlists';
const withToken = (sessionToken, params = {}) => ({ params: { sessionToken, ...params } });

export const personalPlaylistsApi = {
  list(sessionToken) {
    return client.get(base, withToken(sessionToken));
  },
  create(sessionToken, name) {
    return client.post(base, { name }, withToken(sessionToken));
  },
  rename(sessionToken, playlistId, name) {
    return client.patch(`${base}/${encodeURIComponent(playlistId)}`, { name }, withToken(sessionToken));
  },
  remove(sessionToken, playlistId) {
    return client.delete(`${base}/${encodeURIComponent(playlistId)}`, withToken(sessionToken));
  },
  tracks(sessionToken, playlistId, offset = 0, limit = 100) {
    return client.get(`${base}/${encodeURIComponent(playlistId)}/tracks`, withToken(sessionToken, { offset, limit }));
  },
  addTracks(sessionToken, playlistId, musics) {
    return client.post(`${base}/${encodeURIComponent(playlistId)}/tracks/batch`, { musics }, withToken(sessionToken));
  },
  removeTrack(sessionToken, playlistId, trackId) {
    return client.delete(`${base}/${encodeURIComponent(playlistId)}/tracks/${encodeURIComponent(trackId)}`, withToken(sessionToken));
  },
  reorder(sessionToken, playlistId, trackIds) {
    return client.post(`${base}/${encodeURIComponent(playlistId)}/tracks/reorder`, { trackIds }, withToken(sessionToken));
  },
  importNetease(sessionToken, playlistId, externalPlaylistId) {
    return client.post(`${base}/${encodeURIComponent(playlistId)}/import/netease`, { playlistId: externalPlaylistId }, withToken(sessionToken));
  },
  enqueue(sessionToken, playlistId) {
    return client.post(`${base}/${encodeURIComponent(playlistId)}/enqueue`, {}, withToken(sessionToken));
  }
};
