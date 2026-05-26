import { usePlayerStore } from '../stores/player';
import { useUserStore } from '../stores/user';
import { useChatStore } from '../stores/chat';
import { useRoomStore } from '../stores/room';
import { useRoomPlaylistsStore } from '../stores/roomPlaylists';
import { useToast } from '../composables/useToast';
import { socketService } from './socket';
import { WS_DEST } from "../constants/api.js";

/**
 * 处理游戏/播放器事件通知 (Toast)
 * 这里集中管理所有的业务通知文案
 */
function handleGameEvent(event) {
    const userStore = useUserStore();
    const chatStore = useChatStore();
    const { show, error } = useToast(); // This now uses the Pinia store wrapper
    const userName = event.userId === 'SYSTEM' ? '系统' : userStore.resolveName(event.userId);

    // 1. 处理特殊业务逻辑 (非 UI 展示)
    if (event.action === 'LIKE') {
        window.dispatchEvent(new CustomEvent('player:like', { detail: { userId: event.userId } }));
    }

    if (event.action === 'RESET') {
        chatStore.messages = []; // 清空聊天
    }

    if (event.action === 'PASSWORD_CHANGED') {
        error('房间密码已更改，请重新验证');
        setTimeout(() => {
            userStore.resetAuthentication();
            window.location.reload();
        }, 1500);
        return;
    }

    if (event.action === 'ROOM_DELETED') {
        const roomStore = useRoomStore();
        const playerStore = usePlayerStore();
        const chatStore = useChatStore();
        error(event.message || '房间已被删除，已返回 Lounge');
        roomStore.setCurrentRoom('lounge');
        playerStore.reconnectToCurrentRoom();
        chatStore.resetRoomMessages();
        return;
    }

    if (event.action === 'RENAME_FAILED' || (event.type === 'ERROR' && event.message && (event.message.includes('taken') || event.message.includes('占用')))) {
        error(event.message || '该名称已被占用，请更换。');
        userStore.showNameModal = true;
        return;
    }

    if (event.payload === 'room-playlists:update') {
        useRoomPlaylistsStore().loadPlaylists();
        return;
    }

    if (event.action === 'CONTROL_DENIED' && event.message && event.message.includes('设置昵称')) {
        userStore.showNameModal = true;
    }

    // 过滤掉用户进入/离开、以及歌曲开始播放的系统内部通知，避免弹窗干扰
    // 这些事件已经在 Chat Log 中展示，Toast 只展示关键交互
    if (event.action === 'USER_JOIN' || event.action === 'USER_LEAVE' || event.action === 'PLAY_START') {
        return;
    }

    // 2. 使用后端传来的格式化消息
    let msgText = event.message || event.payload || `${userName} ${event.action}`;

    // 如果是 ERROR_LOAD，强制类型为 error，否则使用后端传来的 type (INFO/WARN/SUCCESS)
    let type = event.type ? event.type.toLowerCase() : 'info';
    if (event.action === 'ERROR_LOAD') type = 'error';
    if (event.action === 'RESET' || event.action === 'REMOVE') type = 'warning';

    const titleMap = {
        ERROR_LOAD: '播放错误',
        SYSTEM_MESSAGE: '系统消息',
        CONTROL_DENIED: '操作未生效',
        SEEK_DENIED: '无法调整进度',
        RESET: '房间已重置',
        REMOVE: '队列已更新',
        LIKE: '收到回应'
    };

    show({
        title: titleMap[event.action] || '房间通知',
        message: msgText,
        type: type,
        duration: 3000
    });
}

/**
 * 创建并返回 Socket 消息处理配置
 * @returns {Object} message type -> 回调函数 的映射
 */
export const createSocketHandlers = () => {
    const playerStore = usePlayerStore();
    const userStore = useUserStore();
    const chatStore = useChatStore();
    const roomStore = useRoomStore();

    return {
        // 1. 状态同步
        [WS_DEST.PLAYER_STATE]: (state) => playerStore.syncState(state),
        [WS_DEST.SYNC_PONG]: (pong) => playerStore.handleSyncPong(pong),

        // 2. 用户列表
        [WS_DEST.USERS_ONLINE]: (users) => userStore.setOnlineUsers(users),

        // 3. 队列更新
        [WS_DEST.PLAYER_QUEUE]: (data) => { playerStore.queue = data; },

        // 4. 事件通知 (Toast)
        [WS_DEST.PLAYER_EVENTS]: handleGameEvent,

        // 5. 聊天相关
        [WS_DEST.CHAT_MESSAGE]: (msg) => chatStore.addMessage(msg),
        [WS_DEST.PUBLIC_CHAT_MESSAGE]: (msg) => chatStore.addPublicMessage(msg),
        [WS_DEST.ROOMS_LIST]: (rooms) => roomStore.setRooms(rooms),
        [WS_DEST.CHAT_PRIVATE]: (msg) => chatStore.addMessage(msg),

        // 分页历史记录回调
        [WS_DEST.CHAT_HISTORY]: (moreMessages) => {
            if (chatStore.messages.length === 0) chatStore.setHistory(moreMessages);
            else chatStore.prependHistory(moreMessages);
        },
        [WS_DEST.PUBLIC_CHAT_HISTORY]: (moreMessages) => {
            if (chatStore.publicMessages.length === 0) chatStore.setPublicHistory(moreMessages);
            else chatStore.prependPublicHistory(moreMessages);
        },
        [WS_DEST.ROOM_CREATED]: (room) => {
            roomStore.setRooms([...roomStore.rooms.filter(item => item.roomId !== room.roomId), room]);
            playerStore.switchRoom(room.roomId);
        }
    };
};

/**
 * [新增] 创建 Socket 生命周期回调
 * 包含：连接成功处理、断连处理、错误处理
 */
export const createSocketCallbacks = () => {
    const playerStore = usePlayerStore();
    const userStore = useUserStore();
    const roomStore = useRoomStore();

    return {
        // 连接成功
        onConnect: () => {
            playerStore.connected = true;
            playerStore.resetSyncGate();
            socketService.send(WS_DEST.USER_ME);
            socketService.send(WS_DEST.USERS_ONLINE);
            playerStore.requestPing('connect', true);
            // 发起同步
            setTimeout(() => {
                playerStore.requestResync('connect', true);
                playerStore.requestChatHistory(true);
                playerStore.requestPublicChatHistory(true);
            }, 300);
            // 恢复绑定
            Object.entries(userStore.bindings).forEach(([platform, id]) => {
                if (id) playerStore.bindAccount(platform, id);
            });
        },

        // 连接断开 (含异常断开)
        onDisconnect: () => {
            playerStore.connected = false;
        },

        onAuthError: () => {
            if (roomStore.isPrivateRoom(roomStore.currentRoomId)) {
                roomStore.clearRoomAccessToken(roomStore.currentRoomId);
                roomStore.setCurrentRoom('lounge');
                window.location.reload();
                return;
            }
            userStore.resetAuthentication();
            window.location.reload();
        }
    };
};

