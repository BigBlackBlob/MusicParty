import client from './client';

export const roomApi = {
    list: () => client.get('/api/rooms'),
    verify: (roomId, password, sessionToken) => client.post(`/api/rooms/${roomId}/verify`, { password, sessionToken }),
    update: (roomId, payload) => client.put(`/api/rooms/${roomId}`, payload)
};
