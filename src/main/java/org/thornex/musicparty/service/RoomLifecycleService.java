package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.thornex.musicparty.event.RoomDeletedEvent;

@Service
@RequiredArgsConstructor
public class RoomLifecycleService {

    private final RoomService roomService;
    private final UserService userService;
    private final RoomStatePersistenceService roomStatePersistenceService;
    private final MusicPlayerService musicPlayerService;
    private final ChatService chatService;
    private final ApplicationEventPublisher eventPublisher;

    @Transactional
    public boolean deleteRoom(String roomId, String requesterPublicId, boolean admin) {
        if (!roomService.deleteRoom(roomId, requesterPublicId, admin)) {
            return false;
        }

        userService.movePersistedUsersToDefaultRoom(roomId);
        roomStatePersistenceService.deleteRoomData(roomId);
        Runnable afterCommit = () -> {
            userService.moveUsersToDefaultRoom(roomId);
            musicPlayerService.removeRoom(roomId, true);
            chatService.evictRoomHistory(roomId);
            roomService.publishRoomList();
            eventPublisher.publishEvent(new RoomDeletedEvent(this, roomId));
        };

        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCommit() {
                    afterCommit.run();
                }
            });
        } else {
            afterCommit.run();
        }
        return true;
    }
}
