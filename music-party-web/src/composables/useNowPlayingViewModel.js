import { computed, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { usePlayerStore } from '../stores/player';
import { useUiStore } from '../stores/ui';
import { useUserStore } from '../stores/user';

const platformLabelKeyMap = {
    netease: 'platforms.netease',
    bilibili: 'platforms.bilibili',
    navidrome: 'platforms.navidrome'
};

const resolveRequesterName = (nowPlaying, userStore) => {
    if (nowPlaying?.enqueuedByName) return nowPlaying.enqueuedByName;
    const requester = nowPlaying?.enqueuedById || nowPlaying?.requestedBy || nowPlaying?.userId || nowPlaying?.requesterId;
    return requester ? userStore.resolveName(requester) : '';
};

export const useNowPlayingViewModel = (options = {}) => {
    const player = usePlayerStore();
    const ui = useUiStore();
    const user = useUserStore();
    const { t } = useI18n();
    const artistSeparator = options.artistSeparator || ' / ';

    const nowPlaying = computed(() => player.nowPlaying);
    const music = computed(() => nowPlaying.value?.music || null);
    const coverUrl = computed(() => music.value?.coverUrl || '');
    const trackTitle = computed(() => music.value?.name || t('player.waitingFirstTrack'));
    const artistLine = computed(() => (
        Array.isArray(music.value?.artists) && music.value.artists.length
            ? music.value.artists.join(artistSeparator)
            : t('app.brand')
    ));
    const platformLabel = computed(() => {
        const key = platformLabelKeyMap[music.value?.platform];
        return key ? t(key) : t('platforms.room');
    });
    const requesterName = computed(() => resolveRequesterName(nowPlaying.value, user));
    const durationMs = computed(() => music.value?.duration || 0);
    const progressMs = computed(() => player.playbackPositionMs || 0);
    const progressPercent = computed(() => {
        if (!durationMs.value) return '0%';
        return `${Math.max(0, Math.min(100, (progressMs.value / durationMs.value) * 100))}%`;
    });

    const isRequester = computed(() => {
        if (!nowPlaying.value) return false;
        const requesterId = nowPlaying.value.enqueuedById || nowPlaying.value.requestedBy || nowPlaying.value.userId || nowPlaying.value.requesterId;
        return requesterId === user.userToken;
    });

    const canSeek = computed(() => {
        const hasValidTrack = !!music.value && durationMs.value > 0;
        // Allow seek if it's a valid track AND (no requester exists OR user is the requester OR user is ADMIN)
        const hasPermission = !requesterName.value || isRequester.value || user.currentUser.name === 'ADMIN' || user.currentUser.name === 'AUTO_DJ';
        return hasValidTrack && hasPermission;
    });
    
    const isLiked = computed(() => player.isSongLiked(music.value));
    const activeUserCount = computed(() => user.onlineUsers.length || 1);
    const ambientAccent = computed(() => ui.dynamicAccent?.accent || '#ede1ff');

    const seekToRatio = (ratio) => {
        if (!canSeek.value) return;
        const clamped = Math.max(0, Math.min(1, ratio));
        player.seek(Math.floor(clamped * durationMs.value));
    };

    const toggleLike = () => {
        if (!music.value) return;
        player.sendLike();
    };

    watch(coverUrl, (nextCoverUrl) => {
        ui.updateAccentFromCover(nextCoverUrl);
    }, { immediate: true });

    return {
        player,
        ui,
        user,
        nowPlaying,
        music,
        coverUrl,
        trackTitle,
        artistLine,
        platformLabel,
        requesterName,
        durationMs,
        progressMs,
        progressPercent,
        canSeek,
        isLiked,
        activeUserCount,
        ambientAccent,
        seekToRatio,
        toggleLike
    };
};
