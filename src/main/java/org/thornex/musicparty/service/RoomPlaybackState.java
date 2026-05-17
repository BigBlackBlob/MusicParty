package org.thornex.musicparty.service;

import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.NowPlayingInfo;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.PlayerState;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.persistence.PersistedPlaybackState;

import java.util.List;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;

final class RoomPlaybackState {

    private final AtomicReference<PlayableMusic> currentMusic = new AtomicReference<>(null);
    private final AtomicReference<String> currentEnqueuerId = new AtomicReference<>(null);
    private final AtomicReference<String> currentEnqueuerName = new AtomicReference<>(null);
    private final AtomicLong positionAnchor = new AtomicLong(0);
    private final AtomicLong timestampAnchor = new AtomicLong(0);
    private final AtomicLong positionUpdatedAt = new AtomicLong(0);
    private final AtomicBoolean shuffle = new AtomicBoolean(false);
    private final AtomicBoolean paused = new AtomicBoolean(false);
    private final AtomicBoolean pauseLocked = new AtomicBoolean(false);
    private final AtomicBoolean skipLocked = new AtomicBoolean(false);
    private final AtomicBoolean shuffleLocked = new AtomicBoolean(false);
    private final AtomicBoolean loading = new AtomicBoolean(false);
    private final Set<String> likedUserIds = ConcurrentHashMap.newKeySet();
    private final List<Long> likeMarkers = new CopyOnWriteArrayList<>();
    private final AtomicLong playEpoch = new AtomicLong(0);
    private final AtomicLong stateVersion = new AtomicLong(0);
    private final AtomicLong lastHotActivityAt = new AtomicLong(System.currentTimeMillis());

    synchronized PlayableMusic currentMusic() {
        return currentMusic.get();
    }

    synchronized String currentEnqueuerId() {
        return currentEnqueuerId.get();
    }

    synchronized String currentEnqueuerName() {
        return currentEnqueuerName.get();
    }

    boolean isShuffle() {
        return shuffle.get();
    }

    void setShuffle(boolean value) {
        shuffle.set(value);
    }

    boolean isPaused() {
        return paused.get();
    }

    void setPaused(boolean value) {
        paused.set(value);
    }

    boolean isPauseLocked() {
        return pauseLocked.get();
    }

    void setPauseLocked(boolean value) {
        pauseLocked.set(value);
    }

    boolean isSkipLocked() {
        return skipLocked.get();
    }

    void setSkipLocked(boolean value) {
        skipLocked.set(value);
    }

    boolean isShuffleLocked() {
        return shuffleLocked.get();
    }

    void setShuffleLocked(boolean value) {
        shuffleLocked.set(value);
    }

    boolean isLoading() {
        return loading.get();
    }

    void setLoading(boolean value) {
        loading.set(value);
    }

    Set<String> likedUserIds() {
        return likedUserIds;
    }

    List<Long> likeMarkers() {
        return likeMarkers;
    }

    long playEpoch() {
        return playEpoch.get();
    }

    void setPlayEpoch(long value) {
        playEpoch.set(value);
    }

    long stateVersion() {
        return stateVersion.get();
    }

    void setStateVersion(long value) {
        stateVersion.set(value);
    }

    long lastHotActivityAt() {
        return lastHotActivityAt.get();
    }

    void setLastHotActivityAt(long value) {
        lastHotActivityAt.set(value);
    }

    synchronized void setCurrentTrack(PlayableMusic music, String enqueuerId, String enqueuerName) {
        currentMusic.set(music);
        currentEnqueuerId.set(enqueuerId);
        currentEnqueuerName.set(enqueuerName);
    }

    synchronized void clearCurrentTrack() {
        currentMusic.set(null);
        currentEnqueuerId.set(null);
        currentEnqueuerName.set(null);
        updatePlaybackAnchor(0);
    }

    synchronized void startNewTrack(PlayableMusic music, MusicQueueItem queueItem) {
        likedUserIds.clear();
        likeMarkers.clear();
        setCurrentTrack(music, queueItem.enqueuedBy().publicId(), queueItem.enqueuedBy().name());
        updatePlaybackAnchor(0);
        paused.set(false);
        loading.set(false);
        touchHotActivity();
        bumpPlayEpochAndStateVersion();
    }

    synchronized long calculateCurrentPosition() {
        if (currentMusic.get() == null) {
            return 0;
        }
        if (paused.get()) {
            return positionAnchor.get();
        }
        return positionAnchor.get() + (System.currentTimeMillis() - timestampAnchor.get());
    }

    synchronized void updatePlaybackAnchor(long positionMs) {
        long now = System.currentTimeMillis();
        positionAnchor.set(Math.max(0, positionMs));
        timestampAnchor.set(now);
        positionUpdatedAt.set(now);
    }

    void bumpStateVersion() {
        stateVersion.incrementAndGet();
    }

    void bumpPlayEpochAndStateVersion() {
        playEpoch.incrementAndGet();
        bumpStateVersion();
    }

    void touchHotActivity() {
        lastHotActivityAt.set(System.currentTimeMillis());
    }

    synchronized PersistedPlaybackState snapshot(String roomId) {
        return new PersistedPlaybackState(
                roomId,
                currentMusic.get(),
                currentEnqueuerId.get(),
                currentEnqueuerName.get(),
                positionAnchor.get(),
                timestampAnchor.get(),
                positionUpdatedAt.get(),
                shuffle.get(),
                paused.get(),
                pauseLocked.get(),
                skipLocked.get(),
                shuffleLocked.get(),
                loading.get(),
                new java.util.LinkedHashSet<>(likedUserIds),
                new java.util.ArrayList<>(likeMarkers),
                playEpoch.get(),
                stateVersion.get(),
                System.currentTimeMillis()
        );
    }

    synchronized void applySnapshot(PersistedPlaybackState state) {
        currentMusic.set(state.currentMusic());
        currentEnqueuerId.set(state.currentEnqueuerId());
        currentEnqueuerName.set(state.currentEnqueuerName());
        positionAnchor.set(state.positionAnchor());
        timestampAnchor.set(state.timestampAnchor());
        positionUpdatedAt.set(state.positionUpdatedAt());
        shuffle.set(state.shuffle());
        paused.set(state.paused());
        pauseLocked.set(state.pauseLocked());
        skipLocked.set(state.skipLocked());
        shuffleLocked.set(state.shuffleLocked());
        loading.set(state.loading() && state.currentMusic() != null);
        likedUserIds.clear();
        likedUserIds.addAll(state.likedUserIds());
        likeMarkers.clear();
        likeMarkers.addAll(state.likeMarkers());
        playEpoch.set(state.playEpoch());
        stateVersion.set(state.stateVersion());
        lastHotActivityAt.set(Math.max(state.lastPersistedAt(), System.currentTimeMillis()));
    }

    synchronized PlayerState toPlayerState(String roomId,
                              List<MusicQueueItem> queue,
                              List<UserSummary> onlineUsers,
                              long streamListenerCount) {
        PlayableMusic music = currentMusic.get();
        NowPlayingInfo info = music == null ? null : new NowPlayingInfo(
                music,
                calculateCurrentPosition(),
                currentEnqueuerId.get(),
                currentEnqueuerName.get(),
                likedUserIds,
                likeMarkers,
                playEpoch.get(),
                positionUpdatedAt.get()
        );
        return new PlayerState(
                info,
                queue,
                shuffle.get(),
                onlineUsers,
                paused.get(),
                pauseLocked.get(),
                skipLocked.get(),
                shuffleLocked.get(),
                loading.get(),
                RoomService.DEFAULT_ROOM_ID.equals(roomId) ? (int) streamListenerCount : 0,
                System.currentTimeMillis(),
                stateVersion.get(),
                playEpoch.get()
        );
    }
}
