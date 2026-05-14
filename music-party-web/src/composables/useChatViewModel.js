import { computed, nextTick } from 'vue';
import dayjs from 'dayjs';
import { useChatStore } from '../stores/chat';
import { usePlayerStore } from '../stores/player';
import { useUserStore } from '../stores/user';

const TIME_THRESHOLD = 3 * 60 * 1000;

export const useChatViewModel = (activeTabRef) => {
    const chat = useChatStore();
    const player = usePlayerStore();
    const user = useUserStore();

    const isSelf = (msg) => msg.userId === user.userToken;
    const tabLabel = (tab) => tab === 'CHAT' ? '聊天' : '系统';
    const formatTime = (ts) => dayjs(ts).format('MM-DD HH:mm');

    const filteredMessages = computed(() => {
        const activeTab = activeTabRef?.value || 'CHAT';
        return chat.messages.filter((msg) => {
            if (activeTab === 'CHAT') return msg.type === 'CHAT' || msg.type === 'LIKE' || msg.type === 'PLAY_START';
            if (activeTab === 'SYSTEM') return msg.type === 'SYSTEM' || msg.type === 'LIKE' || msg.type === 'PLAY_START';
            return false;
        });
    });

    const processedMessages = computed(() => {
        const result = [];
        let lastTime = 0;

        for (const msg of filteredMessages.value) {
            const timestamp = Number(msg.timestamp || 0);
            const showTime = timestamp - lastTime > TIME_THRESHOLD;
            if (showTime) lastTime = timestamp;
            result.push({ msg, showTime });
        }

        return result;
    });

    const markAsRead = () => {
        chat.unreadCount = 0;
    };

    const sendMessage = (content) => {
        const text = String(content || '').trim();
        if (!text) return false;
        player.sendChatMessage(text);
        return true;
    };

    const scrollToBottom = async (listRef, force = false) => {
        await nextTick();
        const el = listRef?.value;
        if (!el) return;
        const isAtBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 50;
        if (isAtBottom || force) {
            el.scrollTop = el.scrollHeight;
            if (force) requestAnimationFrame(() => {
                if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight;
            });
        }
    };

    return {
        chat,
        player,
        user,
        filteredMessages,
        processedMessages,
        isSelf,
        tabLabel,
        formatTime,
        markAsRead,
        sendMessage,
        scrollToBottom
    };
};
