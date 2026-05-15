package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.PlayableMusic;

public record PersistedPlaybackState(
        String roomId,
        PlayableMusic currentMusic,
        String currentEnqueuerId,
        String currentEnqueuerName,
        long positionAnchor,
        long timestampAnchor,
        long positionUpdatedAt,
        boolean shuffle,
        boolean paused,
        boolean pauseLocked,
        boolean skipLocked,
        boolean shuffleLocked,
        boolean loading,
        long playEpoch,
        long stateVersion,
        long lastPersistedAt
) {
}
