import client from './client';

export const authApi = {
    getAccountStatus: () => client.get('/api/account/status'),
    registerAccount: (username, password) => client.post('/api/account/register', { username, password }),
    loginAccount: (username, password) => client.post('/api/account/login', { username, password }),
    getAccountMe: (sessionToken) => client.get('/api/account/me', {
        headers: { 'X-Session-Token': sessionToken }
    }),
    adminCommand: (sessionToken, command, roomId) => client.post('/api/admin/command', { sessionToken, command, roomId }),
    grantNavidrome: (sessionToken, userName, roomId) => client.post('/api/admin/navidrome-access/grant', {
        sessionToken,
        userName,
        roomId
    }),
    revokeNavidrome: (sessionToken, userName, roomId) => client.post('/api/admin/navidrome-access/revoke', {
        sessionToken,
        userName,
        roomId
    }),
    setStreamEnabled: (sessionToken, enabled, roomId) => client.post('/api/admin/command', {
        sessionToken,
        command: `//STREAM ${enabled ? 'ON' : 'OFF'}`,
        roomId
    }),
    clearQueue: (sessionToken, roomId) => client.post('/api/admin/command', { sessionToken, command: '//CLEAR QUEUE', roomId }),
    clearChat: (sessionToken, roomId) => client.post('/api/admin/command', { sessionToken, command: '//CLEAR CHAT', roomId }),
    listSubsonicSources: (sessionToken, roomId) => client.get('/api/admin/subsonic-sources', {
        params: { sessionToken, roomId }
    }),
    saveSubsonicSource: (sessionToken, roomId, source) => client.post('/api/admin/subsonic-source', {
        sessionToken,
        roomId,
        ...source
    }),
    removeSubsonicSource: (sessionToken, roomId, id) => client.post('/api/admin/subsonic-source/remove', {
        sessionToken,
        roomId,
        id
    }),
    testSubsonicSource: (sessionToken, roomId, id) => client.post('/api/admin/subsonic-source/test', {
        sessionToken,
        roomId,
        id
    }),
    reorderSubsonicSource: (sessionToken, roomId, id, sortOrder) => client.post('/api/admin/subsonic-source/order', {
        sessionToken,
        roomId,
        id,
        sortOrder
    }),
    listLocalTracks: (sessionToken) => client.get('/api/local/tracks', {
        params: { sessionToken }
    }),
    uploadLocalTrack: (sessionToken, file, metadata = {}) => {
        const formData = new FormData();
        formData.append('file', file);
        Object.entries(metadata).forEach(([key, value]) => {
            if (value !== undefined && value !== null && String(value).trim()) {
                formData.append(key, value);
            }
        });
        return client.post('/api/local/tracks/upload', formData, {
            params: { token: sessionToken },
            headers: { 'Content-Type': 'multipart/form-data' },
            timeout: 120000
        });
    },
    updateLocalTrack: (sessionToken, id, track) => client.patch(`/api/local/tracks/${id}`, {
        ...track,
        token: sessionToken
    }),
    deleteLocalTrack: (sessionToken, id) => client.delete(`/api/local/tracks/${id}`, {
        params: { token: sessionToken }
    }),
    listLocalUploadAccess: (sessionToken) => client.get('/api/local/upload-access', {
        params: { sessionToken }
    }),
    grantLocalUploadAccess: (sessionToken, userName) => client.post('/api/local/upload-access/grant', {
        sessionToken,
        userName
    }),
    revokeLocalUploadAccess: (sessionToken, userName) => client.post('/api/local/upload-access/revoke', {
        sessionToken,
        userName
    }),
};
