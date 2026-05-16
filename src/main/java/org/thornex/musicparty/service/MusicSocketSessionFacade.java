package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.messaging.MessageHeaders;
import org.springframework.messaging.simp.SimpMessageHeaderAccessor;
import org.springframework.messaging.simp.SimpMessageType;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.CurrentUserResponse;
import org.thornex.musicparty.dto.PlayerEvent;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.dto.SyncPingRequest;
import org.thornex.musicparty.dto.SyncPongResponse;

import java.util.List;

@Service
@RequiredArgsConstructor
public class MusicSocketSessionFacade {

    private final MusicPlayerService musicPlayerService;
    private final UserService userService;
    private final SimpMessagingTemplate messagingTemplate;

    public void sendPlayerResync(String sessionId) {
        messagingTemplate.convertAndSendToUser(
                sessionId,
                "/queue/player/state",
                musicPlayerService.getCurrentPlayerStateForSession(sessionId),
                createSessionHeaders(sessionId)
        );
    }

    public void sendSyncPong(String sessionId, SyncPingRequest request) {
        long serverReceiveTime = System.currentTimeMillis();
        SyncPongResponse response = new SyncPongResponse(
                request.pingId(),
                request.clientSendTime(),
                serverReceiveTime,
                System.currentTimeMillis()
        );
        messagingTemplate.convertAndSendToUser(
                sessionId,
                "/queue/sync/pong",
                response,
                createSessionHeaders(sessionId)
        );
    }

    public void sendSeekDenied(String sessionId, String message) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "SEEK_DENIED", user.getPublicId(), message, null);
            messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
        });
    }

    public boolean renameAndBroadcast(String sessionId, String newName) {
        if (!userService.renameUser(sessionId, newName)) {
            return false;
        }
        musicPlayerService.broadcastOnlineUsers();
        userService.getUser(sessionId).ifPresent(user -> {
            CurrentUserResponse summary = new CurrentUserResponse(
                    user.getSessionToken(),
                    user.getPublicId(),
                    user.getName(),
                    user.isGuest()
            );
            messagingTemplate.convertAndSendToUser(sessionId, "/queue/me", summary, createSessionHeaders(sessionId));
        });
        return true;
    }

    public void sendRenameFailed(String sessionId) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "RENAME_FAILED", user.getPublicId(), "该名称已被占用或包含非法字符，请更换。", null);
            messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
        });
    }

    public void sendRoomCreated(String sessionId, RoomInfo room) {
        messagingTemplate.convertAndSendToUser(sessionId, "/queue/rooms/created", room, createSessionHeaders(sessionId));
    }

    public void sendRoomCreateFailed(String sessionId, String message) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "ROOM_CREATE_FAILED", user.getPublicId(), message, null);
            messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
        });
    }

    public void sendRoomDeleteFailed(String sessionId) {
        userService.getUser(sessionId).ifPresent(user -> {
            PlayerEvent errorEvent = new PlayerEvent("ERROR", "ROOM_DELETE_FAILED", user.getPublicId(), "无权删除该房间", null);
            messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
        });
    }

    public void sendRoomChatHistory(String sessionId, List<ChatMessage> history) {
        messagingTemplate.convertAndSendToUser(sessionId, "/queue/chat/history", history, createSessionHeaders(sessionId));
    }

    public void sendPublicChatHistory(String sessionId, List<ChatMessage> history) {
        messagingTemplate.convertAndSendToUser(sessionId, "/queue/public-chat/history", history, createSessionHeaders(sessionId));
    }

    private MessageHeaders createSessionHeaders(String sessionId) {
        SimpMessageHeaderAccessor headerAccessor = SimpMessageHeaderAccessor.create(SimpMessageType.MESSAGE);
        headerAccessor.setSessionId(sessionId);
        headerAccessor.setLeaveMutable(true);
        return headerAccessor.getMessageHeaders();
    }
}
