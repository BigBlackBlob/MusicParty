import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useToast } from './useToast';
import { musicApi } from '../api/music';
import { authApi } from '../api/auth';
import { useRoomStore } from '../stores/room';

export function useSearchLogic(emit) {
    const { success, error } = useToast();
    const { t } = useI18n();
    const roomStore = useRoomStore();

    const platform = ref('netease');
    const keyword = ref('');
    const songs = ref([]);
    const loading = ref(false);
    const listMode = ref('search'); // 'search' | 'playlist'
    const isAdminMode = ref(false);

    // 管理员指令状态
    const adminCommandType = ref('');
    const adminCommandArg = ref('');

    const handleAdminCommand = async (pwd) => {
        try {
            if (adminCommandType.value === 'RESET') {
                await authApi.adminReset(pwd);
                success(t('search.adminExecuted'));
            } else if (adminCommandType.value === 'PASS') {
                await authApi.adminSetPassword(pwd, adminCommandArg.value);
                success(t('search.adminExecuted'));
            } else if (adminCommandType.value === 'OPEN') {
                await authApi.adminSetPassword(pwd, "");
                success(t('search.adminExecuted'));
            }
            emit('close');
        } catch {
            error(t('search.adminFailed'));
        } finally {
            isAdminMode.value = false;
            keyword.value = '';
            adminCommandType.value = '';
        }
    };

    const doSearch = async () => {
        const val = keyword.value.trim();
        if (!val) return;

        // 1. 管理员密码输入模式
        if (isAdminMode.value) {
            await handleAdminCommand(val);
            return;
        }

        // 2. 指令拦截
        if (val === '//RESET') {
            isAdminMode.value = true;
            adminCommandType.value = 'RESET';
            keyword.value = '';
            return;
        }
        if (val.startsWith('//PASS ')) {
            isAdminMode.value = true;
            adminCommandType.value = 'PASS';
            adminCommandArg.value = val.substring(7);
            keyword.value = '';
            return;
        }
        if (val === '//OPEN') {
            isAdminMode.value = true;
            adminCommandType.value = 'OPEN';
            keyword.value = '';
            return;
        }

        // 3. 普通搜索
        listMode.value = 'search';
        loading.value = true;
        songs.value = [];
        try {
            const data = await musicApi.search(platform.value, val, undefined, 0, 20, roomStore.currentRoomId);
            songs.value = data;
        } catch (e) {
            console.error(e);
            error(t('search.failed'));
        } finally {
            loading.value = false;
        }
    };

    return {
        platform,
        keyword,
        songs,
        loading,
        listMode,
        isAdminMode,
        doSearch
    };
}
