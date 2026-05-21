import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { authApi } from '../api/auth';
import { STORAGE_KEYS } from '../constants/keys';

const sessionToken = ref(localStorage.getItem(STORAGE_KEYS.SESSION_TOKEN) || '');
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

    const bindings = ref(JSON.parse(localStorage.getItem(STORAGE_KEYS.BINDINGS) || '{}'));
    // 全局状态：控制改名弹窗显示
    const showNameModal = ref(false);

    const onNameSetCallback = ref(null);

    const isGuest = ref(!storageName);

    // 核心方法：将 SessionID 翻译成名字
    const resolveName = (id, fallbackName) => {
        if (!id) return 'Unknown';
        if (id === 'ADMIN') return 'AUTO_DJ';

        if (id === publicId.value) return currentUser.value.name;

        const u = onlineUsers.value.find(u => u.publicId === id);

        return u ? u.name : (fallbackName || 'Unknown Agent');
    };

    /**
     * 2. 初始化用户身份 (来自 /app/user/me)
     * 逻辑：对比服务器认为的名字 (serverName) 和我本地存储的名字
     * serverIsGuest: 后端返回的当前是否为游客状态
     */
    const initUser = (serverSessionToken, serverPublicId, serverName, serverIsGuest, serverRole = 'GUEST', serverIsAdmin = false) => {
        if (serverSessionToken) {
            sessionToken.value = serverSessionToken;
            localStorage.setItem(STORAGE_KEYS.SESSION_TOKEN, serverSessionToken);
        }
        if (serverPublicId) {
            publicId.value = serverPublicId;
        }
        role.value = serverIsAdmin ? 'ADMIN' : (serverRole || 'USER');

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
        initUser(session.sessionToken, session.publicId, session.displayName || session.username, session.guest, session.role, session.admin);
        accountLastLoginAt.value = session.lastLoginAt || null;
        if (session.username) {
            localStorage.setItem(STORAGE_KEYS.ACCOUNT_USERNAME, session.username);
        }
    };

    const refreshAccount = async () => {
        if (!sessionToken.value) return null;
        const session = await authApi.getAccountMe(sessionToken.value);
        initAccount(session);
        return session;
    };

    const updateProfile = async (displayName) => {
        const session = await authApi.updateAccountProfile(sessionToken.value, displayName);
        initAccount(session);
        return session;
    };

    const changePassword = async (currentPassword, newPassword) => {
        await authApi.changeAccountPassword(sessionToken.value, currentPassword, newPassword);
        clearAccountIdentity();
    };

    const logout = async () => {
        const token = sessionToken.value;
        if (token) {
            try {
                await authApi.logoutAccount(token);
            } finally {
                clearAccountIdentity();
            }
            return;
        }
        clearAccountIdentity();
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
        localStorage.removeItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS);
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
        localStorage.removeItem(STORAGE_KEYS.SESSION_TOKEN);
        localStorage.removeItem(STORAGE_KEYS.ACCOUNT_USERNAME);
        localStorage.removeItem(STORAGE_KEYS.USERNAME);
        localStorage.removeItem(STORAGE_KEYS.BINDINGS);
        localStorage.removeItem(STORAGE_KEYS.ROOM_ACCESS_TOKENS);
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
        isAdmin: computed(() => role.value === 'ADMIN'),
        initAccount,
        refreshAccount,
        updateProfile,
        changePassword,
        logout,
        setPostNameAction,
        isAuthPassed,
        resetAuthentication
    };
});
