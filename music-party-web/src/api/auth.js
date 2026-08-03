import client from './client';

const accountRequestOptions = {
    timeout: 60000
};

const accountTokenOptions = () => accountRequestOptions;

export const authApi = {
    getAccountStatus: () => client.get('/api/account/status'),
    loginAccount: (username, password) => client.post('/api/account/login', { username, password }, accountRequestOptions),
    getAccountMe: () => client.get('/api/account/me', accountTokenOptions()),
    updateAccountProfile: (displayName) => client.put('/api/account/profile', { displayName }, accountTokenOptions()),
    logoutAccount: () => client.post('/api/account/logout', null, accountTokenOptions()),
    inviteMetadata: (secret) => client.get(`/api/join/${encodeURIComponent(secret)}/metadata`),
    redeemInvite: (secret, displayName) => client.post('/api/invites/redeem', { secret, displayName }, accountRequestOptions),
    createInvite: (roomId, label = '') => client.post(`/api/rooms/${encodeURIComponent(roomId)}/invites`, { label }),
    listInvites: (roomId) => client.get(`/api/rooms/${encodeURIComponent(roomId)}/invites`),
    revokeInvite: (roomId, inviteId) => client.delete(`/api/rooms/${encodeURIComponent(roomId)}/invites/${encodeURIComponent(inviteId)}`),
    updatePlatformCredential: (platform, credential) => client.post('/api/admin/platform-credentials', { platform, credential }),
    listRoomMembers: (roomId) => client.get(`/api/rooms/${encodeURIComponent(roomId)}/members`),
    removeRoomMember: (roomId, publicId) => client.delete(`/api/rooms/${encodeURIComponent(roomId)}/members/${encodeURIComponent(publicId)}`),
    grantNavidrome: (_sessionToken, userName, roomId) => client.post('/api/admin/navidrome-access/grant', {
        userName, roomId
    }),
    revokeNavidrome: (_sessionToken, userName, roomId) => client.post('/api/admin/navidrome-access/revoke', {
        userName, roomId
    }),
    listSubsonicSources: (_sessionToken, roomId) => client.get('/api/admin/subsonic-sources', {
        params: { roomId }
    }),
    saveSubsonicSource: (_sessionToken, roomId, source) => client.post('/api/admin/subsonic-source', {
        roomId,
        ...source
    }),
    removeSubsonicSource: (_sessionToken, roomId, id) => client.post('/api/admin/subsonic-source/remove', {
        roomId,
        id
    }),
    testSubsonicSource: (_sessionToken, roomId, id) => client.post('/api/admin/subsonic-source/test', {
        roomId,
        id
    }),
    reorderSubsonicSource: (_sessionToken, roomId, id, sortOrder) => client.post('/api/admin/subsonic-source/order', {
        roomId,
        id,
        sortOrder
    }),
    listLocalTracks: () => client.get('/api/local/tracks'),
    uploadLocalTrack: (_sessionToken, file, metadata = {}) => {
        const formData = new FormData();
        formData.append('file', file);
        Object.entries(metadata).forEach(([key, value]) => {
            if (value !== undefined && value !== null && String(value).trim()) {
                formData.append(key, value);
            }
        });
        return client.post('/api/local/tracks/upload', formData, {
            headers: { 'Content-Type': 'multipart/form-data' },
            timeout: 120000
        });
    },
    updateLocalTrack: (_sessionToken, id, track) => client.patch(`/api/local/tracks/${id}`, track),
    deleteLocalTrack: (_sessionToken, id) => client.delete(`/api/local/tracks/${id}`),
    listLocalUploadAccess: () => client.get('/api/local/upload-access'),
    grantLocalUploadAccess: (_sessionToken, userName) => client.post('/api/local/upload-access/grant', { userName }),
    revokeLocalUploadAccess: (_sessionToken, userName) => client.post('/api/local/upload-access/revoke', { userName }),
};
