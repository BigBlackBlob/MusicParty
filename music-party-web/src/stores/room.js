import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { STORAGE_KEYS } from '../constants/keys';
import { socketService } from '../services/socket';
import { WS_DEST } from '../constants/api';
import { roomApi } from '../api/rooms';
import { useUserStore } from './user';

const DEFAULT_ROOM_ID = 'lounge';

export const useRoomStore = defineStore('room', () => {
    const userStore = useUserStore();
    const rooms = ref([]);
    const currentRoomId = ref(localStorage.getItem(STORAGE_KEYS.ROOM_ID) || DEFAULT_ROOM_ID);
    const isLoading = ref(false);
    const createError = ref('');
    const roomAccessTokens = ref(JSON.parse(localStorage.getItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS) || '{}'));

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
        rooms.value = nextRooms.length ? nextRooms : [{ roomId: DEFAULT_ROOM_ID, name: 'Lounge', system: true, onlineCount: 0 }];
        if (!rooms.value.some(room => room.roomId === currentRoomId.value)) {
            setCurrentRoom(DEFAULT_ROOM_ID);
        }
    };

    const setCurrentRoom = (roomId) => {
        currentRoomId.value = roomId || DEFAULT_ROOM_ID;
        localStorage.setItem(STORAGE_KEYS.ROOM_ID, currentRoomId.value);
    };

    const getRoomAccessToken = (roomId) => roomAccessTokens.value[roomId || DEFAULT_ROOM_ID] || '';

    const setRoomAccessToken = (roomId, token) => {
        const key = roomId || DEFAULT_ROOM_ID;
        if (token) {
            roomAccessTokens.value[key] = token;
        } else {
            delete roomAccessTokens.value[key];
        }
        localStorage.setItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS, JSON.stringify(roomAccessTokens.value));
    };

    const clearRoomAccessToken = (roomId) => {
        setRoomAccessToken(roomId, '');
    };

    const createRoom = (name) => {
        createError.value = '';
        socketService.send(WS_DEST.ROOM_CREATE, { name });
    };

    const deleteRoom = (roomId) => {
        socketService.send(WS_DEST.ROOM_DELETE, { roomId });
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
        createRoom,
        deleteRoom
    };
});
