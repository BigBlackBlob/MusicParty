import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { authApi } from '../api/auth';
import { STORAGE_KEYS } from '../constants/keys';
import { isPlainObject, safeJsonStorage } from '../utils/safeJsonStorage.js';

// WebSocket-issued capability token used for stream/resource proxy requests.
// Keep it memory-only; never persist it to browser storage.
const sessionToken = ref('');
const publicId = ref('');

export const useUserStore = defineStore('user', () => {
    const onlineUsers = ref([]);
    const role = ref('GUEST');
    const accountLastLoginAt = ref(null);

    const isAuthPassed = ref(false);

    // 启动时：严格从 LocalStorage 读取，默认值只在这里设定一次
    const storageName = localStorage.getItem('mp_username');
    const currentUser = ref({
        name: storageName || '游客',
        sessionId: ''
    });

    const bindings = ref(safeJsonStorage.read(STORAGE_KEYS.BINDINGS, {}, { validate: isPlainObject }));
    // 全局状态：控制改名弹窗显示
    const showNameModal = ref(false);

    const onNameSetCallback = ref(null);

    const isGuest = ref(!storageName);

    const accountType = computed(() => {
        if (role.value === 'PLATFORM_ADMIN' || role.value === 'ADMIN') return 'admin';
        if (isGuest.value || role.value === 'GUEST') return 'guest';
        return 'user';
    });

    const canManageRooms = computed(() => accountType.value === 'admin');
    const canCreatePlaylists = computed(() => true); // 访客也可以创建个人歌单（根据决策2）

    // 核心方法：将 SessionID 翻译成名字
    const resolveName = (id, fallbackName) => {
        if (!id) return 'Unknown';
        if (id === 'ADMIN') return 'AUTO_DJ';

        if (id === publicId.value) return currentUser.value.name;

        const u = onlineUsers.value.find(u => u.publicId === id);

        return u ? u.name : (fallbackName || 'Unknown Agent');
    };

    /**
     * 2. 初始化用户身份 (来自 user.me)
     * 逻辑：对比服务器认为的名字 (serverName) 和我本地存储的名字
     * serverIsGuest: 后端返回的当前是否为游客状态
     */
    const initUser = (serverSessionToken, serverPublicId, serverName, serverIsGuest, serverRole = 'GUEST', serverIsAdmin = false) => {
        sessionToken.value = serverSessionToken || sessionToken.value || '';
        if (serverPublicId) {
            publicId.value = serverPublicId;
        }
        role.value = serverIsAdmin ? 'PLATFORM_ADMIN' : (serverRole || 'MEMBER');

        // 1. 同步名字
        if (serverName) {
            currentUser.value.name = serverName;
            onlineUsers.value = onlineUsers.value.map(user => (
                user.publicId === serverPublicId ? { ...user, name: serverName } : user
            ));
        }

        // 2. 同步身份状态 (以服务端为准)
        if (serverIsGuest !== undefined) {
            // 状态变更检测: Guest -> User (转正)
            if (isGuest.value && !serverIsGuest) {
                console.log("Identity upgraded to User");
                isGuest.value = false;
                localStorage.setItem(STORAGE_KEYS.USERNAME, serverName);
                showNameModal.value = false; // 成功改名后自动关闭弹窗

                // 执行待办回调 (如打开搜索框)
                if (onNameSetCallback.value) {
                    onNameSetCallback.value();
                    onNameSetCallback.value = null;
                }
            }
            // 状态变更检测: User -> Guest (降级/重置)
            else if (!isGuest.value && serverIsGuest) {
                console.log("Identity degraded to Guest");
                isGuest.value = true;
                localStorage.removeItem(STORAGE_KEYS.USERNAME);
            }
        }

        // 3. 如果是正式用户，确保本地存储名字与服务端一致 (处理去重后缀)
        if (!isGuest.value && serverName) {
            localStorage.setItem(STORAGE_KEYS.USERNAME, serverName);
        }

        return false;
    };

    const initAccount = (session) => {
        if (!session) return;
        initUser(
            session.sessionToken,
            session.publicId,
            session.displayName || session.username,
            session.guest !== undefined ? session.guest : true, // 默认为访客
            session.role,
            session.role === 'PLATFORM_ADMIN' || session.role === 'ADMIN'
        );
        accountLastLoginAt.value = session.lastLoginAt || null;
    };

    const refreshAccount = async () => {
        const session = await authApi.getAccountMe();
        initAccount(session);
        return session;
    };

    const updateProfile = async (displayName) => {
        const session = await authApi.updateAccountProfile(displayName);
        initAccount(session);
        return session;
    };


    const logout = async () => {
        try {
            await authApi.logoutAccount();
        } finally {
            clearAccountIdentity();
        }
    };

    const setOnlineUsers = (users) => {
        onlineUsers.value = users;
    };

    const updateBinding = (platform, accountId) => {
        bindings.value[platform] = accountId;
        localStorage.setItem(STORAGE_KEYS.BINDINGS, JSON.stringify(bindings.value));
    };

    // 废弃: 不再直接修改本地状态，改为等待 initUser 的后端回调
    const saveName = () => {
        // Logic moved to initUser response handling
    }

    const setPostNameAction = (fn) => {
        onNameSetCallback.value = fn;
    }

    const resetAuthentication = () => {
        isAuthPassed.value = false;
    };

    const clearAccountIdentity = () => {
        sessionToken.value = '';
        publicId.value = '';
        role.value = 'GUEST';
        accountLastLoginAt.value = null;
        isGuest.value = true;
        isAuthPassed.value = false;
        currentUser.value = { name: '游客', sessionId: '' };
        bindings.value = {};
        localStorage.removeItem(STORAGE_KEYS.USERNAME);
        localStorage.removeItem(STORAGE_KEYS.BINDINGS);
    };

    return {
        onlineUsers,
        currentUser,
        bindings,
        initUser,
        setOnlineUsers,
        updateBinding,
        saveName,
        isGuest,
        showNameModal,
        resolveName,
        sessionToken,
        publicId,
        role,
        accountLastLoginAt,
        isAdmin: computed(() => role.value === 'PLATFORM_ADMIN' || role.value === 'ADMIN'),
        accountType,
        canManageRooms,
        canCreatePlaylists,
        initAccount,
        refreshAccount,
        updateProfile,
        logout,
        setPostNameAction,
        isAuthPassed,
        resetAuthentication
    };
});
