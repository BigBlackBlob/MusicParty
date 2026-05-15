import { defineStore } from 'pinia';
import { ref } from 'vue';
import { useUserStore } from './user';
import { socketService } from '../services/socket';
import { WS_DEST } from '../constants/api';

export const useChatStore = defineStore('chat', () => {
    // 状态
    const messages = ref([]);
    const publicMessages = ref([]);
    const unreadCount = ref(0);
    const publicUnreadCount = ref(0);
    const isOpen = ref(false);

    // 分页相关状态
    const hasMore = ref(true);
    const publicHasMore = ref(true);
    const isLoadingMore = ref(false);
    const isLoadingPublicMore = ref(false);

    const userStore = useUserStore();
    const LIMIT_PER_PAGE = 50;

    // 1. 添加单条消息 (来自 WebSocket 推送)
    const addMessage = (msg) => {
        messages.value.push(msg);

        if (messages.value.length > 2000) {
            messages.value = messages.value.slice(-1000);
        }

        // 未读计数逻辑：窗口关闭 && 不是自己发的 && 是普通聊天消息
        const isSelf = msg.userId === userStore.publicId;
        if (!isOpen.value && !isSelf && msg.type === 'CHAT') {
            unreadCount.value++;
        }
    };

    const addPublicMessage = (msg) => {
        publicMessages.value.push(msg);
        if (publicMessages.value.length > 2000) {
            publicMessages.value = publicMessages.value.slice(-1000);
        }
        const isSelf = msg.userId === userStore.publicId;
        if (!isOpen.value && !isSelf && msg.type === 'CHAT') {
            publicUnreadCount.value++;
        }
    };

    // 2. 初始化历史记录 (连接成功后获取最近 50 条)
    const setHistory = (history) => {
        messages.value = history; // 覆盖
        unreadCount.value = 0;

        // 如果返回数量少于分页限制，说明没有更多了
        hasMore.value = history.length >= LIMIT_PER_PAGE;
    };

    const setPublicHistory = (history) => {
        publicMessages.value = history;
        publicUnreadCount.value = 0;
        publicHasMore.value = history.length >= LIMIT_PER_PAGE;
    };

    // 3. 加载更多历史记录 (向上滚动触发)
    const loadMoreHistory = () => {
        if (!hasMore.value || isLoadingMore.value) return;

        isLoadingMore.value = true;
        const currentCount = messages.value.length;

        // 发送 WebSocket 请求
        socketService.send(WS_DEST.CHAT_HISTORY_FETCH, {
            offset: currentCount,
            limit: LIMIT_PER_PAGE
        });
    };

    const loadMorePublicHistory = () => {
        if (!publicHasMore.value || isLoadingPublicMore.value) return;
        isLoadingPublicMore.value = true;
        socketService.send(WS_DEST.PUBLIC_CHAT_HISTORY_FETCH, {
            offset: publicMessages.value.length,
            limit: LIMIT_PER_PAGE
        });
    };

    // 4. 处理加载到的更多历史数据 (回调)
    const prependHistory = (moreMessages) => {
        if (moreMessages.length === 0) {
            hasMore.value = false;
        } else {
            // 将旧消息拼接到数组头部
            messages.value = [...moreMessages, ...messages.value];
            if (moreMessages.length < LIMIT_PER_PAGE) {
                hasMore.value = false;
            }
        }
        isLoadingMore.value = false;
    };

    const prependPublicHistory = (moreMessages) => {
        if (moreMessages.length === 0) {
            publicHasMore.value = false;
        } else {
            publicMessages.value = [...moreMessages, ...publicMessages.value];
            if (moreMessages.length < LIMIT_PER_PAGE) {
                publicHasMore.value = false;
            }
        }
        isLoadingPublicMore.value = false;
    };

    const resetRoomMessages = () => {
        messages.value = [];
        unreadCount.value = 0;
        hasMore.value = true;
        isLoadingMore.value = false;
    };

    const toggleChat = () => {
        isOpen.value = !isOpen.value;
        if (isOpen.value) {
            unreadCount.value = 0;
            publicUnreadCount.value = 0;
        }
    };

    return {
        messages,
        publicMessages,
        unreadCount,
        publicUnreadCount,
        isOpen,
        hasMore,
        publicHasMore,
        isLoadingMore,
        isLoadingPublicMore,
        addMessage,
        addPublicMessage,
        toggleChat,
        setHistory,
        setPublicHistory,
        loadMoreHistory,
        loadMorePublicHistory,
        prependHistory,
        prependPublicHistory,
        resetRoomMessages
    };
});
