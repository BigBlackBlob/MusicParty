import client from './client';

export const authApi = {
    // 检查房间状态 (是否初始化/有密码)
    getStatus: () => client.get('/api/auth/status'),
    // 验证密码
    verify: (password) => client.post('/api/auth/verify', { password }),
    // 初始化/设置密码
    setup: (password) => client.post('/api/auth/setup', { password }),

    // 管理员指令
    // 统一管理员指令接口
    adminCommand: (password, command, roomId) => client.post('/api/admin/command', { password, command, roomId }),
    grantNavidrome: (adminPassword, userName, roomId) => client.post('/api/admin/navidrome-access/grant', {
        adminPassword,
        userName,
        roomId
    }),
    revokeNavidrome: (adminPassword, userName, roomId) => client.post('/api/admin/navidrome-access/revoke', {
        adminPassword,
        userName,
        roomId
    }),
    setStreamEnabled: (password, enabled, roomId) => client.post('/api/admin/command', {
        password,
        command: `//STREAM ${enabled ? 'ON' : 'OFF'}`,
        roomId
    }),
    clearQueue: (password, roomId) => client.post('/api/admin/command', { password, command: '//CLEAR QUEUE', roomId }),
    clearChat: (password, roomId) => client.post('/api/admin/command', { password, command: '//CLEAR CHAT', roomId }),
    listSubsonicSources: (adminPassword, roomId) => client.get('/api/admin/subsonic-sources', {
        params: { adminPassword, roomId }
    }),
    saveSubsonicSource: (adminPassword, roomId, source) => client.post('/api/admin/subsonic-source', {
        adminPassword,
        roomId,
        ...source
    }),
    removeSubsonicSource: (adminPassword, roomId, id) => client.post('/api/admin/subsonic-source/remove', {
        adminPassword,
        roomId,
        id
    }),
    testSubsonicSource: (adminPassword, roomId, id) => client.post('/api/admin/subsonic-source/test', {
        adminPassword,
        roomId,
        id
    }),
    reorderSubsonicSource: (adminPassword, roomId, id, sortOrder) => client.post('/api/admin/subsonic-source/order', {
        adminPassword,
        roomId,
        id,
        sortOrder
    }),
};
