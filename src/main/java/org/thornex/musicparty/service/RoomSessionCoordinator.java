package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.event.RoomDeletedEvent;
import org.thornex.musicparty.event.RoomSessionEvictedEvent;
import org.thornex.musicparty.event.UserCountChangeEvent;

@Service
@RequiredArgsConstructor
@Slf4j
public class RoomSessionCoordinator {

    private final RoomService roomService;
    private final ApplicationEventPublisher eventPublisher;

    public void markRoomActive(String roomId) {
        roomService.markRoomActive(roomId);
    }

    public void onUserEnteredRoom(String roomId, int onlineUserCount) {
        markRoomActive(roomId);
        eventPublisher.publishEvent(new UserCountChangeEvent(this, roomId, onlineUserCount));
        roomService.publishRoomList();
    }

    public void onUserMovedRooms(String previousRoomId,
                                 String currentRoomId,
                                 int previousOnlineCount,
                                 int currentOnlineCount) {
        String previous = roomKey(previousRoomId);
        String current = roomKey(currentRoomId);
        if (previous.equals(current)) {
            onUserEnteredRoom(current, currentOnlineCount);
            return;
        }
        markRoomActive(current);
        eventPublisher.publishEvent(new UserCountChangeEvent(this, previous, previousOnlineCount));
        eventPublisher.publishEvent(new UserCountChangeEvent(this, current, currentOnlineCount));
        roomService.publishRoomList();
    }

    public void onUserDisconnected(String roomId, int onlineUserCount) {
        eventPublisher.publishEvent(new UserCountChangeEvent(this, roomKey(roomId), onlineUserCount));
        roomService.publishRoomList();
    }

    public void onUsersMovedToRoom(String fromRoomId, String toRoomId, int fromOnlineCount, int toOnlineCount) {
        String from = roomKey(fromRoomId);
        String to = roomKey(toRoomId);
        if (!from.equals(to)) {
            markRoomActive(to);
            eventPublisher.publishEvent(new UserCountChangeEvent(this, from, fromOnlineCount));
        }
        eventPublisher.publishEvent(new UserCountChangeEvent(this, to, toOnlineCount));
        roomService.publishRoomList();
    }

    public void evictColdRoom(String roomId, Runnable flushAction) {
        if (flushAction != null) {
            flushAction.run();
        }
        eventPublisher.publishEvent(new RoomSessionEvictedEvent(this, roomId));
        log.info("Evicted cold room session {}", roomId);
    }

    public void cleanupDeletedRoom(String roomId,
                                   Runnable moveUsersToDefaultRoomAction,
                                   Runnable removePlayerSessionAction,
                                   Runnable evictChatHistoryAction) {
        if (moveUsersToDefaultRoomAction != null) {
            moveUsersToDefaultRoomAction.run();
        }
        if (removePlayerSessionAction != null) {
            removePlayerSessionAction.run();
        }
        if (evictChatHistoryAction != null) {
            evictChatHistoryAction.run();
        }
        roomService.publishRoomList();
        eventPublisher.publishEvent(new RoomDeletedEvent(this, roomId));
    }

    private String roomKey(String roomId) {
        return roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
    }
}
