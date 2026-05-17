import { Client } from '@stomp/stompjs';

class SocketService {
    constructor() {
        this.client = null;
        this.connected = false;
        this.stompConfig = null;
        this.subscriptionIds = [];
    }

    /**
     * 初始化连接
     * @param {Object} authHeaders - { 'user-name':..., 'session-token':..., 'room-password':... }
     * @param {Object} callbacks - 回调函数集合
     * @param {Function} callbacks.onConnect - 连接成功
     * @param {Function} callbacks.onDisconnect - 连接断开
     * @param {Function} callbacks.onStompError - STOMP 错误 (如密码错误)
     * @param {Object} subscriptions - 订阅配置 { topic: callbackFn }
     */
    connect(authHeaders, callbacks, subscriptions) {
        const nextConfig = { authHeaders, callbacks, subscriptions };
        if (this.client && this.client.active) {
            if (!this.shouldReconnectForConfig(nextConfig)) return;
            console.info('Socket context changed, reconnecting with fresh subscriptions.');
            this.disconnect();
        }
        this.stompConfig = nextConfig;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const brokerURL = `${protocol}//${window.location.host}/ws`;

        this.client = new Client({
            brokerURL,
            connectHeaders: authHeaders,
            heartbeatIncoming: 10000,
            heartbeatOutgoing: 10000,
            reconnectDelay: 2000,

            onConnect: (frame) => {
                this.connected = true;

                // 1. 注册所有订阅
                this.subscriptionIds = [];
                Object.entries(subscriptions).forEach(([topic, handler]) => {
                    const subscription = this.client.subscribe(topic, (message) => {
                        const body = JSON.parse(message.body);
                        handler(body);
                    });
                    this.subscriptionIds.push(subscription.id);
                });

                // 2. 触发连接成功回调
                if (callbacks.onConnect) callbacks.onConnect(frame);
            },

            // 监听非正常关闭 (如网络中断、服务器重启)
            onWebSocketClose: () => {
                console.warn('WebSocket connection closed.');
                this.connected = false;
                // 触发断开回调，让 Store 感知状态变化
                if (callbacks.onDisconnect) callbacks.onDisconnect();
            },

            onDisconnect: () => {
                this.connected = false;
                if (callbacks.onDisconnect) callbacks.onDisconnect();
            },

            onStompError: (frame) => {
                console.error('STOMP Error:', frame.body);
                if (callbacks.onStompError) callbacks.onStompError(frame);
            }
        });

        this.client.activate();
    }

    /**
     * 发送指令 (通用)
     * @param {string} destination - 目标地址 (来自 WS_DEST)
     * @param {Object} body - 消息体
     */
    send(destination, body = {}) {
        if (this.client && this.connected) {
            this.client.publish({ destination, body: JSON.stringify(body) });
            return true;
        }
        console.warn('Socket not connected, cannot send:', {
            destination,
            connected: this.connected,
            active: Boolean(this.client?.active)
        });
        return false;
    }

    /**
     * 强制重连 (用于网络恢复或从后台切回时)
     */
    forceReconnect() {
        if (this.client && !this.client.active) {
            console.log('Force reconnecting socket...');
            this.client.activate();
        }
    }

    reconnectNow() {
        const config = this.stompConfig;
        this.disconnect();
        if (config) {
            setTimeout(() => this.connect(config.authHeaders, config.callbacks, config.subscriptions), 100);
        }
    }

    /**
     * 断开连接
     */
    disconnect() {
        if (this.client) {
            this.client.deactivate();
            this.client = null;
            this.connected = false;
            this.subscriptionIds = [];
        }
    }

    shouldReconnectForConfig(nextConfig) {
        if (!this.stompConfig) return true;
        return !this.sameHeaders(this.stompConfig.authHeaders, nextConfig.authHeaders)
            || !this.sameSubscriptionTopics(this.stompConfig.subscriptions, nextConfig.subscriptions);
    }

    sameHeaders(current = {}, next = {}) {
        const currentKeys = Object.keys(current).sort();
        const nextKeys = Object.keys(next).sort();
        if (currentKeys.length !== nextKeys.length) return false;
        return currentKeys.every((key, index) => key === nextKeys[index] && current[key] === next[key]);
    }

    sameSubscriptionTopics(current = {}, next = {}) {
        const currentTopics = Object.keys(current).sort();
        const nextTopics = Object.keys(next).sort();
        if (currentTopics.length !== nextTopics.length) return false;
        return currentTopics.every((topic, index) => topic === nextTopics[index]);
    }
}

// 导出单例
export const socketService = new SocketService();
