import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { STORAGE_KEYS } from '../constants/keys';
import { socketService } from '../services/socket';
import { WS_DEST } from '../constants/api';
import { roomApi } from '../api/rooms';
import { useUserStore } from './user';

const DEFAULT_ROOM_ID = 'lounge';

const fallbackRooms = [{ roomId: DEFAULT_ROOM_ID, name: 'Lounge', system: true, onlineCount: 0 }];

const parseStoredAccessTokens = () => {
    try {
        const parsed = JSON.parse(localStorage.getItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS) || '{}');
        return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
    } catch {
        localStorage.removeItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS);
        return {};
    }
};

export const useRoomStore = defineStore('room', () => {
    const userStore = useUserStore();
    const rooms = ref([]);
    const currentRoomId = ref(localStorage.getItem(STORAGE_KEYS.ROOM_ID) || DEFAULT_ROOM_ID);
    const isLoading = ref(false);
    const createError = ref('');
    const roomAccessTokens = ref(parseStoredAccessTokens());

    const currentRoom = computed(() => rooms.value.find(room => room.roomId === currentRoomId.value) || rooms.value[0] || {
        roomId: DEFAULT_ROOM_ID,
        name: 'Lounge',
        system: true,
        onlineCount: 0
    });

    const fetchRooms = async () => {
        isLoading.value = true;
        try {
            const data = await roomApi.list(userStore.sessionToken);
            setRooms(Array.isArray(data) ? data : []);
        } finally {
            isLoading.value = false;
        }
    };

    const setRooms = (nextRooms) => {
        rooms.value = nextRooms.length ? nextRooms : fallbackRooms;
        if (!rooms.value.some(room => room.roomId === currentRoomId.value)) {
            setCurrentRoom(DEFAULT_ROOM_ID);
        }
    };

    const setCurrentRoom = (roomId) => {
        currentRoomId.value = roomId || DEFAULT_ROOM_ID;
        localStorage.setItem(STORAGE_KEYS.ROOM_ID, currentRoomId.value);
    };

    const persistRoomAccessTokens = () => {
        localStorage.setItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS, JSON.stringify(roomAccessTokens.value));
    };

    const findRoom = (roomId) => rooms.value.find(room => room.roomId === (roomId || DEFAULT_ROOM_ID));

    const isPrivateRoom = (roomId) => Boolean(findRoom(roomId)?.privateRoom);

    const getRoomAccessToken = (roomId) => {
        const key = roomId || DEFAULT_ROOM_ID;
        const entry = roomAccessTokens.value[key];
        if (!entry) return '';

        if (typeof entry === 'string') {
            return entry;
        }

        if (typeof entry !== 'object') {
            delete roomAccessTokens.value[key];
            persistRoomAccessTokens();
            return '';
        }

        const token = entry.token || '';
        const expiresAt = Number(entry.expiresAt);
        if (!token) {
            delete roomAccessTokens.value[key];
            persistRoomAccessTokens();
            return '';
        }

        if (Number.isFinite(expiresAt) && expiresAt <= Date.now()) {
            delete roomAccessTokens.value[key];
            persistRoomAccessTokens();
            return '';
        }

        return token;
    };

    const setRoomAccessToken = (roomId, token, expiresAt = null) => {
        const key = roomId || DEFAULT_ROOM_ID;
        if (token) {
            const numericExpiresAt = Number(expiresAt);
            roomAccessTokens.value[key] = {
                token,
                expiresAt: Number.isFinite(numericExpiresAt) ? numericExpiresAt : undefined
            };
        } else {
            delete roomAccessTokens.value[key];
        }
        persistRoomAccessTokens();
    };

    const clearRoomAccessToken = (roomId) => {
        setRoomAccessToken(roomId, '');
    };

    const hasValidRoomAccess = (roomId) => !isPrivateRoom(roomId) || Boolean(getRoomAccessToken(roomId));

    const verifyRoomAccess = async (roomId, password) => {
        const response = await roomApi.verify(roomId || DEFAULT_ROOM_ID, password, userStore.sessionToken);
        setRoomAccessToken(roomId, response?.roomAccessToken || '', response?.expiresAt);
        return response;
    };

    const createRoom = (name) => {
        createError.value = '';
        socketService.send(WS_DEST.ROOM_CREATE, { name });
    };

    const updateRoom = async (roomId, payload) => {
        const updated = await roomApi.update(roomId, {
            ...payload,
            sessionToken: userStore.sessionToken
        });
        await fetchRooms();
        return updated;
    };

    const deleteRoom = async (roomId) => {
        await roomApi.remove(roomId, userStore.sessionToken);
        if (currentRoomId.value === roomId) {
            setCurrentRoom(DEFAULT_ROOM_ID);
        }
        await fetchRooms();
    };

    return {
        rooms,
        currentRoomId,
        currentRoom,
        isLoading,
        createError,
        roomAccessTokens,
        fetchRooms,
        setRooms,
        setCurrentRoom,
        getRoomAccessToken,
        setRoomAccessToken,
        clearRoomAccessToken,
        hasValidRoomAccess,
        verifyRoomAccess,
        isPrivateRoom,
        createRoom,
        updateRoom,
        deleteRoom
    };
});
