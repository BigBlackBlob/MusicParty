package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.CurrentUserResponse;
import org.thornex.musicparty.dto.PlayerEvent;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.dto.SyncPingRequest;
import org.thornex.musicparty.dto.SyncPongResponse;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.websocket.ReactiveSocketBroker;

import java.util.List;

@Service
@RequiredArgsConstructor
public class MusicSocketSessionFacade {

    private final MusicPlayerService musicPlayerService;
    private final UserService userService;
    private final ReactiveSocketBroker broker;
    private final AccountService accountService;

    public void sendPlayerResync(String sessionId) {
        broker.sendToSession(sessionId, "player.state", musicPlayerService.getCurrentPlayerStateForSession(sessionId));
    }

    public void sendSyncPong(String sessionId, SyncPingRequest request) {
        long serverReceiveTime = System.currentTimeMillis();
        SyncPongResponse response = new SyncPongResponse(
                request.pingId(),
                request.clientSendTime(),
                serverReceiveTime,
                System.currentTimeMillis()
        );
        broker.sendToSession(sessionId, "sync.pong", response);
    }

    public void sendCurrentUser(String sessionId) {
        userService.getUser(sessionId).ifPresentOrElse(user -> {
            CurrentUserResponse summary = new CurrentUserResponse(
                    user.getSessionToken(),
                    user.getPublicId(),
                    user.getName(),
                    user.isGuest(),
                    accountService.roleForPublicId(user.getPublicId()).orElse("GUEST"),
                    accountService.isAdminSession(user.getSessionToken())
            );
            broker.sendToSession(sessionId, "user.me", summary);
        }, () -> broker.sendToSession(sessionId, "user.me", new CurrentUserResponse("", "", "Unknown", true, "GUEST", false)));
    }

    public void sendOnlineUsers(String sessionId) {
        String roomId = userService.getRoomIdForSession(sessionId);
        List<UserSummary> users = userService.getOnlineUserSummaries(roomId);
        broker.sendToSession(sessionId, "users.online", roomId, users);
    }

    public void sendSeekDenied(String sessionId, String message) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "SEEK_DENIED", user.getPublicId(), message, null);
            broker.sendToSession(sessionId, "player.events", errorEvent);
        });
    }

    public void sendControlDenied(String sessionId, String action, String message) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("WARN", action, user.getPublicId(), message, null);
            broker.sendToSession(sessionId, "player.events", errorEvent);
        });
    }

    public void sendQueueReorderAck(String sessionId, String mutationId) {
        broker.sendToSession(sessionId, "queue.reorder.ack", new QueueReorderResult(mutationId, null));
    }

    public void sendQueueReorderNack(String sessionId, String mutationId, String reason) {
        broker.sendToSession(sessionId, "queue.reorder.nack", new QueueReorderResult(mutationId, reason));
    }

    public boolean renameAndBroadcast(String sessionId, String newName) {
        if (!userService.renameUser(sessionId, newName)) {
            return false;
        }
        musicPlayerService.broadcastOnlineUsers();
        sendCurrentUser(sessionId);
        return true;
    }

    public void sendRenameFailed(String sessionId) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "RENAME_FAILED", user.getPublicId(), "该名称已被占用或包含非法字符，请更换。", null);
            broker.sendToSession(sessionId, "player.events", errorEvent);
        });
    }

    public void sendRoomCreated(String sessionId, RoomInfo room) {
        broker.sendToSession(sessionId, "rooms.created", room);
    }

    public void sendRoomCreateFailed(String sessionId, String message) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "ROOM_CREATE_FAILED", user.getPublicId(), message, null);
            broker.sendToSession(sessionId, "player.events", errorEvent);
        });
    }

    public void sendRoomDeleteFailed(String sessionId) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "ROOM_DELETE_FAILED", user.getPublicId(), "无权删除该房间", null);
            broker.sendToSession(sessionId, "player.events", errorEvent);
        });
    }

    public void sendRoomChatHistory(String sessionId, List<ChatMessage> history) {
        broker.sendToSession(sessionId, "chat.history", history);
    }

    public void sendPublicChatHistory(String sessionId, List<ChatMessage> history) {
        broker.sendToSession(sessionId, "public-chat.history", history);
    }

    private record QueueReorderResult(String mutationId, String reason) {}
}
