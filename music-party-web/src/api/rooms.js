import client from './client';

export const roomApi = {
    list: (sessionToken) => client.get('/api/rooms', { params: sessionToken ? { sessionToken } : {} }),
    verify: (roomId, password, sessionToken) => client.post(`/api/rooms/${roomId}/verify`, { password, sessionToken }),
    update: (roomId, payload) => client.put(`/api/rooms/${roomId}`, payload)
};
