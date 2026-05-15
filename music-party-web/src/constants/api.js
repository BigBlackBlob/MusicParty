// WebSocket 目的地
export const WS_DEST = {
    // 发送指令 (Publish)
    CHAT_SEND: '/app/chat',
    PUBLIC_CHAT_SEND: '/app/public-chat',
    CHAT_HISTORY_FETCH: '/app/chat/history/fetch',
    PUBLIC_CHAT_HISTORY_FETCH: '/app/public-chat/history/fetch',
    PLAYER_NEXT: '/app/control/next',
    PLAYER_PAUSE: '/app/control/toggle-pause',
    PLAYER_SHUFFLE: '/app/control/toggle-shuffle',
    PLAYER_SEEK: '/app/control/seek',
    PLAYER_LIKE: '/app/control/like',
    ENQUEUE: '/app/enqueue',
    ENQUEUE_PLAYLIST: '/app/enqueue/playlist',
    ENQUEUE_ALBUM: '/app/enqueue/album',
    QUEUE_TOP: '/app/queue/top',
    QUEUE_REMOVE: '/app/queue/remove',
    QUEUE_BATCH_TOP: '/app/queue/batch-top',
    QUEUE_BATCH_REMOVE: '/app/queue/batch-remove',
    QUEUE_REORDER: '/app/queue/reorder',
    USER_BIND: '/app/user/bind',
    USER_RENAME: '/app/user/rename',
    ROOM_CREATE: '/app/rooms/create',
    ROOM_DELETE: '/app/rooms/delete',
    SYNC_PING: '/app/sync/ping',
    RESYNC: '/app/player/resync',

    // 订阅频道 (Subscribe)
    TOPIC_EVENTS: '/topic/player/events',
    TOPIC_STATE: '/topic/player/state',
    TOPIC_QUEUE: '/topic/player/queue',
    TOPIC_USERS: '/topic/users/online',
    TOPIC_CHAT: '/topic/chat',
    TOPIC_PUBLIC_CHAT: '/topic/public/chat',
    TOPIC_ROOMS_LIST: '/topic/rooms/list',

    // 个人频道
    USER_ME: '/app/user/me',
    USER_ME_UPDATE: '/user/queue/me',
    APP_CHAT_HISTORY: '/app/chat/history',
    USER_STATE: '/user/queue/player/state',
    USER_SYNC_PONG: '/user/queue/sync/pong',
    USER_CHAT_HISTORY: '/user/queue/chat/history',
    USER_PUBLIC_CHAT_HISTORY: '/user/queue/public-chat/history',
    USER_EVENTS: '/user/queue/events',
    USER_PRIVATE_CHAT: '/user/queue/chat/private',
    USER_ROOM_CREATED: '/user/queue/rooms/created'
};

export const roomTopic = (roomId, suffix) => `/topic/rooms/${roomId || 'lounge'}${suffix}`;
