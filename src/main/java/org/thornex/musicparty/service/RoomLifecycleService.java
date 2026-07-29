package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class RoomLifecycleService {

    private final RoomService roomService;
    private final UserService userService;
    private final RoomStatePersistenceService roomStatePersistenceService;
    private final MusicPlayerService musicPlayerService;
    private final ChatService chatService;
    private final RoomSessionCoordinator roomSessionCoordinator;
    private final RoomStateMutationService roomStateMutationService;
    private final AfterCommitExecutor afterCommitExecutor;

    public boolean deleteRoom(String roomId, String requesterPublicId, boolean admin) {
        return roomStateMutationService.supplyInTransaction(() -> {
            if (!roomService.deleteRoom(roomId, requesterPublicId, admin)) {
                return false;
            }

            userService.movePersistedUsersToDefaultRoom(roomId);
            roomStatePersistenceService.deleteRoomData(roomId);
            afterCommitExecutor.run(() -> roomSessionCoordinator.cleanupDeletedRoom(
                    roomId,
                    () -> userService.moveUsersToDefaultRoom(roomId),
                    () -> musicPlayerService.removeRoom(roomId, true),
                    () -> chatService.evictRoomHistory(roomId)
            ));
            return true;
        });
    }
}
