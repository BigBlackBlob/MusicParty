package org.thornex.musicparty.dto;

import java.util.List;

public record PlayerState(
        NowPlayingInfo nowPlaying,
        List<MusicQueueItem> queue,
        boolean isShuffle,
        List<UserSummary> onlineUsers,
        boolean isPaused,
        boolean isPauseLocked,
        boolean isSkipLocked,
        boolean isShuffleLocked,
        boolean isLoading,
        long serverTimestamp,
        long stateVersion,
        long playEpoch,
        long queueVersion
) {
    public PlayerState(NowPlayingInfo nowPlaying, List<MusicQueueItem> queue, boolean isShuffle,
                       List<UserSummary> onlineUsers, boolean isPaused, boolean isPauseLocked,
                       boolean isSkipLocked, boolean isShuffleLocked, boolean isLoading,
                       long serverTimestamp, long stateVersion, long playEpoch) {
        this(nowPlaying, queue, isShuffle, onlineUsers, isPaused, isPauseLocked, isSkipLocked, isShuffleLocked,
                isLoading, serverTimestamp, stateVersion, playEpoch, 0);
    }

    public PlayerState withQueueVersion(long queueVersion) {
        return new PlayerState(nowPlaying, queue, isShuffle, onlineUsers, isPaused, isPauseLocked, isSkipLocked,
                isShuffleLocked, isLoading, serverTimestamp, stateVersion, playEpoch, queueVersion);
    }
}
