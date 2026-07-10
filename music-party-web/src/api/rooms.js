import client from './client';

export const roomApi = {
    list: () => client.get('/api/rooms'),
    verify: (roomId, password) => client.post(`/api/rooms/${roomId}/verify`, { password }),
    update: (roomId, payload) => client.put(`/api/rooms/${roomId}`, payload),
    remove: (roomId, sessionToken) => client.delete(`/api/rooms/${roomId}`, { params: { sessionToken } })
};
