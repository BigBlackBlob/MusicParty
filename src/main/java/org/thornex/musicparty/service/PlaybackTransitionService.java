package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.event.PlayerStateEvent;
import org.thornex.musicparty.event.QueueUpdateEvent;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.persistence.PersistedPlaybackState;

import java.util.List;

@Service
@RequiredArgsConstructor
public class PlaybackTransitionService {

    private final RoomStatePersistenceService roomStatePersistenceService;
    private final RoomStateMutationService roomStateMutationService;
    private final ApplicationEventPublisher eventPublisher;
    private final AfterCommitExecutor afterCommitExecutor;

    public void apply(PlaybackTransition transition) {
        roomStateMutationService.runInTransaction(() -> {
            if (transition.persistedQueueSnapshot() != null) {
                roomStatePersistenceService.persistQueueSnapshot(transition.roomId(), transition.persistedQueueSnapshot());
            }
            if (transition.persistedPlaybackState() != null) {
                roomStatePersistenceService.persistPlaybackState(transition.persistedPlaybackState());
            }
            afterCommitExecutor.run(() -> publishEvents(transition));
        });
    }

    private void publishEvents(PlaybackTransition transition) {
        if (transition.queueUpdateEvent() != null) {
            eventPublisher.publishEvent(transition.queueUpdateEvent());
        }
        if (transition.playerStateEvent() != null) {
            eventPublisher.publishEvent(transition.playerStateEvent());
        }
        if (transition.systemMessageEvent() != null) {
            eventPublisher.publishEvent(transition.systemMessageEvent());
        }
    }

    public record PlaybackTransition(
            String roomId,
            List<MusicQueueItem> persistedQueueSnapshot,
            PersistedPlaybackState persistedPlaybackState,
            QueueUpdateEvent queueUpdateEvent,
            PlayerStateEvent playerStateEvent,
            SystemMessageEvent systemMessageEvent
    ) {
    }
}
