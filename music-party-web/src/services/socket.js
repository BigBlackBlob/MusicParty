import { getConnectionProfile } from '../api/connectionProfile';

class SocketService {
    constructor() {
        this.client = null;
        this.connected = false;
        this.socketConfig = null;
        this.reconnectTimer = null;
        this.reconnectDelay = 2000;
        this.intentionalClose = false;
        this.requestSeq = 0;
    }

    connect(authParams = {}, callbacks = {}, handlers = {}) {
        const nextConfig = { authParams, callbacks, handlers };
        if (this.client && this.isSocketActive(this.client)) {
            if (!this.shouldReconnectForConfig(nextConfig)) return;
            console.info('Socket context changed, reconnecting.');
            this.disconnect();
        }

        this.socketConfig = nextConfig;
        this.intentionalClose = false;
        this.clearReconnectTimer();

        const socket = new WebSocket(this.buildUrl(authParams));
        this.client = socket;

        socket.onopen = (event) => {
            if (this.client !== socket) return;
            this.connected = true;
            callbacks.onConnect?.(event);
        };

        socket.onmessage = (event) => {
            if (this.client !== socket) return;
            this.handleMessage(event.data, handlers);
        };

        socket.onerror = (event) => {
            if (this.client !== socket) return;
            callbacks.onError?.(event);
        };

        socket.onclose = (event) => {
            if (this.client !== socket) return;
            this.connected = false;
            callbacks.onDisconnect?.(event);
            if (event?.code === 1008) {
                callbacks.onAuthError?.(event);
                return;
            }
            if (!this.intentionalClose) this.scheduleReconnect();
        };
    }

    buildUrl(authParams = {}) {
        const profile = getConnectionProfile();
        const origin = profile.baseUrl || window.location.origin;
        const httpUrl = new URL(origin);
        const protocol = httpUrl.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = new URL(`${protocol}//${httpUrl.host}/ws`);
        Object.entries(authParams).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                url.searchParams.set(key, value);
            }
        });
        return url.toString();
    }

    handleMessage(rawData, handlers = {}) {
        let envelope;
        try {
            envelope = JSON.parse(rawData);
        } catch (error) {
            console.warn('Ignoring malformed socket message:', rawData, error);
            return;
        }

        const type = envelope?.type;
        if (!type) return;
        const handler = handlers[type];
        if (!handler) {
            console.debug('No socket handler registered for message type:', type);
            return;
        }
        handler(envelope.payload, envelope);
    }

    send(type, payload = {}) {
        if (this.client && this.connected && this.client.readyState === WebSocket.OPEN) {
            this.client.send(JSON.stringify({
                type,
                requestId: this.nextRequestId(type),
                payload
            }));
            return true;
        }
        console.warn('Socket not connected, cannot send:', {
            type,
            connected: this.connected,
            readyState: this.client?.readyState
        });
        return false;
    }

    forceReconnect() {
        this.reconnectNow();
    }

    reconnectNow() {
        const config = this.socketConfig;
        this.disconnect();
        if (config) {
            setTimeout(() => this.connect(config.authParams, config.callbacks, config.handlers), 100);
        }
    }

    disconnect() {
        this.intentionalClose = true;
        this.clearReconnectTimer();
        if (this.client) {
            const socket = this.client;
            this.client = null;
            socket.onopen = null;
            socket.onmessage = null;
            socket.onerror = null;
            socket.onclose = null;
            if (this.isSocketActive(socket)) socket.close();
        }
        this.connected = false;
    }

    scheduleReconnect() {
        if (this.reconnectTimer || !this.socketConfig) return;
        const config = this.socketConfig;
        this.reconnectTimer = setTimeout(() => {
            this.reconnectTimer = null;
            this.connect(config.authParams, config.callbacks, config.handlers);
        }, this.reconnectDelay);
    }

    clearReconnectTimer() {
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
    }

    nextRequestId(type) {
        this.requestSeq += 1;
        return `${type}-${Date.now()}-${this.requestSeq}`;
    }

    isSocketActive(socket) {
        return socket.readyState === WebSocket.CONNECTING || socket.readyState === WebSocket.OPEN;
    }

    shouldReconnectForConfig(nextConfig) {
        if (!this.socketConfig) return true;
        return !this.sameParams(this.socketConfig.authParams, nextConfig.authParams)
            || !this.sameHandlerTypes(this.socketConfig.handlers, nextConfig.handlers);
    }

    sameParams(current = {}, next = {}) {
        const currentKeys = Object.keys(current).sort();
        const nextKeys = Object.keys(next).sort();
        if (currentKeys.length !== nextKeys.length) return false;
        return currentKeys.every((key, index) => key === nextKeys[index] && current[key] === next[key]);
    }

    sameHandlerTypes(current = {}, next = {}) {
        const currentTypes = Object.keys(current).sort();
        const nextTypes = Object.keys(next).sort();
        if (currentTypes.length !== nextTypes.length) return false;
        return currentTypes.every((type, index) => type === nextTypes[index]);
    }
}

export const socketService = new SocketService();
