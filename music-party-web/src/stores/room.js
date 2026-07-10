import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { STORAGE_KEYS } from '../constants/keys';
import { socketService } from '../services/socket';
import { WS_DEST } from '../constants/api';
import { roomApi } from '../api/rooms';
import { useUserStore } from './user';

const DEFAULT_ROOM_ID = 'lounge';

const fallbackRooms = [{ roomId: DEFAULT_ROOM_ID, name: 'Lounge', system: true, onlineCount: 0 }];

export const useRoomStore = defineStore('room', () => {
    const userStore = useUserStore();
    const rooms = ref([]);
    const currentRoomId = ref(localStorage.getItem(STORAGE_KEYS.ROOM_ID) || DEFAULT_ROOM_ID);
    const isLoading = ref(false);
    const createError = ref('');
    const roomAccessTokens = ref({});

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

    const findRoom = (roomId) => rooms.value.find(room => room.roomId === (roomId || DEFAULT_ROOM_ID));

    const isPrivateRoom = (roomId) => Boolean(findRoom(roomId)?.privateRoom);

    const getRoomAccessToken = (roomId) => {
        const key = roomId || DEFAULT_ROOM_ID;
        return roomAccessTokens.value[key] ? 'granted' : '';
    };

    const setRoomAccessToken = (roomId, token) => {
        const key = roomId || DEFAULT_ROOM_ID;
        if (token) {
            roomAccessTokens.value[key] = true;
        } else {
            delete roomAccessTokens.value[key];
        }
    };

    const clearRoomAccessToken = (roomId) => {
        setRoomAccessToken(roomId, '');
    };

    const hasValidRoomAccess = (roomId) => !isPrivateRoom(roomId) || Boolean(getRoomAccessToken(roomId));

    const verifyRoomAccess = async (roomId, password) => {
        const response = await roomApi.verify(roomId || DEFAULT_ROOM_ID, password);
        setRoomAccessToken(roomId, response?.valid ? 'granted' : '');
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
