package org.thornex.musicparty.controller;

import org.springframework.messaging.MessageHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.messaging.handler.annotation.MessageMapping;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.messaging.simp.SimpMessageHeaderAccessor;
import org.springframework.messaging.simp.SimpMessageType;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.messaging.simp.annotation.SubscribeMapping;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.stereotype.Controller;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.event.RoomDeletedEvent;
import org.thornex.musicparty.service.ChatService;
import org.thornex.musicparty.service.MusicPlayerService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.UserService;

import java.util.List;
import java.util.UUID;

@Controller
public class MusicSocketController {

    private final MusicPlayerService musicPlayerService;
    private final UserService userService;
    private final SimpMessagingTemplate messagingTemplate;
    private final ChatService chatService;
    private final RoomService roomService;
    private final ApplicationEventPublisher eventPublisher;

    public MusicSocketController(MusicPlayerService musicPlayerService, UserService userService, SimpMessagingTemplate messagingTemplate, ChatService chatService, RoomService roomService, ApplicationEventPublisher eventPublisher) {
        this.musicPlayerService = musicPlayerService;
        this.userService = userService;
        this.messagingTemplate = messagingTemplate;
        this.chatService = chatService;
        this.roomService = roomService;
        this.eventPublisher = eventPublisher;
    }

    @MessageMapping("/player/resync")
    public void requestResync(@Header("simpSessionId") String sessionId) {
        messagingTemplate.convertAndSendToUser(
                sessionId,
                "/queue/player/state",
                musicPlayerService.getCurrentPlayerStateForSession(sessionId),
                createSessionHeaders(sessionId)
        );
    }

    @MessageMapping("/sync/ping")
    public void syncPing(@Payload SyncPingRequest request, @Header("simpSessionId") String sessionId) {
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

    @MessageMapping("/enqueue")
    public void enqueue(EnqueueRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.enqueue(request, sessionId);
    }

    @MessageMapping("/enqueue/playlist")
    public void enqueuePlaylist(EnqueuePlaylistRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.enqueuePlaylist(request, sessionId);
    }

    @MessageMapping("/enqueue/album")
    public void enqueueAlbum(EnqueueAlbumRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.enqueueAlbum(request, sessionId);
    }

    @MessageMapping("/control/next")
    public void nextSong(@Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.skipToNext(sessionId);
    }

    @MessageMapping("/control/toggle-shuffle")
    public void toggleShuffle(@Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.toggleShuffle(sessionId);
    }

    @MessageMapping("/control/toggle-pause")
    public void togglePause(@Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.togglePause(sessionId);
    }

    @MessageMapping("/control/seek")
    public void seek(@Payload SeekRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.seekTo(request.positionMs(), sessionId).ifPresent(message -> {
            userService.getUser(sessionId).ifPresent(user -> {
                PlayerEvent errorEvent = new PlayerEvent("ERROR", "SEEK_DENIED", user.getPublicId(), message, null);
                messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
            });
        });
    }

    @MessageMapping("/queue/top")
    public void topSong(@Payload QueueActionRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.topSong(request.queueId(), sessionId);
    }

    @MessageMapping("/queue/batch-top")
    public void topSongs(@Payload QueueBatchActionRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.topSongs(request.queueIds(), sessionId);
    }

    @MessageMapping("/queue/remove")
    public void removeSong(@Payload QueueActionRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.removeSongFromQueue(request.queueId(), sessionId);
    }

    @MessageMapping("/queue/batch-remove")
    public void removeSongs(@Payload QueueBatchActionRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.removeSongsFromQueue(request.queueIds(), sessionId);
    }

    // 点赞接口
    @MessageMapping("/control/like")
    public void likeSong(@Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.likeSong(sessionId);
    }

    @MessageMapping("/user/rename")
    public void rename(RenameRequest request, @Header("simpSessionId") String sessionId) {
        if (userService.renameUser(sessionId, request.newName())) {
            musicPlayerService.broadcastOnlineUsers();
            // PUSH updated user info to the user
            userService.getUser(sessionId).ifPresent(user -> {
                CurrentUserResponse summary = new CurrentUserResponse(user.getSessionToken(), user.getPublicId(), user.getName(), user.isGuest());
                messagingTemplate.convertAndSendToUser(sessionId, "/queue/me", summary, createSessionHeaders(sessionId));
            });
        } else {
            // RENAME_FAILED
            userService.getUser(sessionId).ifPresent(user -> {
                PlayerEvent errorEvent = new PlayerEvent("ERROR", "RENAME_FAILED", user.getPublicId(), "该名称已被占用或包含非法字符，请更换。", null);
                messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
            });
        }
    }

    @MessageMapping("/user/bind")
    public void bindAccount(BindRequest request, @Header("simpSessionId") String sessionId) {
        userService.bindAccount(sessionId, request.platform(), request.accountId());
    }

    @MessageMapping("/rooms/create")
    public void createRoom(RoomCreateRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        userService.getUser(sessionId).ifPresent(user -> {
            try {
                RoomInfo room = roomService.createRoom(request.name(), user.getPublicId());
                messagingTemplate.convertAndSendToUser(sessionId, "/queue/rooms/created", room, createSessionHeaders(sessionId));
            } catch (IllegalArgumentException ex) {
                PlayerEvent errorEvent = new PlayerEvent("ERROR", "ROOM_CREATE_FAILED", user.getPublicId(), ex.getMessage(), null);
                messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
            }
        });
    }

    @MessageMapping("/rooms/delete")
    public void deleteRoom(RoomDeleteRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        userService.getUser(sessionId).ifPresent(user -> {
            String roomId = request.roomId();
            boolean isAdmin = roomService.isAdminPassword(request.adminPassword());
            if (roomService.deleteRoom(roomId, user.getPublicId(), isAdmin)) {
                userService.moveUsersToDefaultRoom(roomId);
                musicPlayerService.removeRoom(roomId);
                chatService.deleteRoomHistory(roomId);
                eventPublisher.publishEvent(new RoomDeletedEvent(this, roomId));
            } else {
                PlayerEvent errorEvent = new PlayerEvent("ERROR", "ROOM_DELETE_FAILED", user.getPublicId(), "无权删除该房间", null);
                messagingTemplate.convertAndSendToUser(sessionId, "/queue/events", errorEvent, createSessionHeaders(sessionId));
            }
        });
    }

    @SubscribeMapping("/topic/player/state")
    public PlayerState getInitialPlayerState() {
        return musicPlayerService.getCurrentPlayerState(RoomService.DEFAULT_ROOM_ID);
    }

    @SubscribeMapping("/topic/users/online")
    public List<UserSummary> getInitialOnlineUsers() {
        return userService.getOnlineUserSummaries(RoomService.DEFAULT_ROOM_ID);
    }

    @SubscribeMapping("/user/me")
    public CurrentUserResponse getMyUserInfo(@Header("simpSessionId") String sessionId) {
        return userService.getUser(sessionId)
                .map(u -> new CurrentUserResponse(u.getSessionToken(), u.getPublicId(), u.getName(), u.isGuest()))
                .orElse(new CurrentUserResponse("", "", "Unknown", true));
    }

    private boolean isGuest(String sessionId) {
        return userService.getUser(sessionId).map(User::isGuest).orElse(true);
    }

    // 聊天消息处理
    @MessageMapping("/chat")
    public void handleChat(ChatRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        
        // 1. Check content validity
        if (request.content() == null || request.content().trim().isEmpty()) return;
        if (!chatService.isMessageLengthValid(request.content())) return;

        // 2. Try process as command
        if (chatService.processIncomingMessage(sessionId, request.content().trim())) {
            return;
        }

        userService.getUser(sessionId).ifPresent(user -> {
            // 3. Rate Limit Check
            if (!chatService.canUserSendMessage(user.getPublicId())) return;

            ChatMessage message = new ChatMessage(
                    java.util.UUID.randomUUID().toString(),
                    user.getPublicId(),
                    user.getName(), 
                    request.content().trim(),
                    System.currentTimeMillis(),
                    MessageType.CHAT
            );

            String roomId = userService.getRoomIdForSession(sessionId);
            chatService.addMessage(roomId, message);

            messagingTemplate.convertAndSend("/topic/rooms/" + roomId + "/chat", message);
        });
    }

    @MessageMapping("/public-chat")
    public void handlePublicChat(ChatRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        if (request.content() == null || request.content().trim().isEmpty()) return;
        if (!chatService.isMessageLengthValid(request.content())) return;

        userService.getUser(sessionId).ifPresent(user -> {
            if (!chatService.canUserSendMessage("public:" + user.getPublicId())) return;
            ChatMessage message = new ChatMessage(
                    UUID.randomUUID().toString(),
                    user.getPublicId(),
                    user.getName(),
                    request.content().trim(),
                    System.currentTimeMillis(),
                    MessageType.CHAT
            );
            chatService.addPublicMessage(message);
            messagingTemplate.convertAndSend("/topic/public/chat", message);
        });
    }

    // 订阅时获取历史记录
    @SubscribeMapping("/chat/history")
    public List<ChatMessage> getChatHistory() {
        return chatService.getHistory(RoomService.DEFAULT_ROOM_ID, 0, 50);
    }

    // 处理分页获取历史记录的请求
    @MessageMapping("/chat/history/fetch")
    public void fetchChatHistory(@Payload ChatHistoryFetchRequest request, @Header("simpSessionId") String sessionId) {
        List<ChatMessage> history = chatService.getHistory(userService.getRoomIdForSession(sessionId), request.offset(), request.limit());
        messagingTemplate.convertAndSendToUser(
                sessionId,
                "/queue/chat/history",
                history,
                createSessionHeaders(sessionId)
        );
    }

    @MessageMapping("/public-chat/history/fetch")
    public void fetchPublicChatHistory(@Payload ChatHistoryFetchRequest request, @Header("simpSessionId") String sessionId) {
        List<ChatMessage> history = chatService.getPublicHistory(request.offset(), request.limit());
        messagingTemplate.convertAndSendToUser(
                sessionId,
                "/queue/public-chat/history",
                history,
                createSessionHeaders(sessionId)
        );
    }

    private MessageHeaders createSessionHeaders(String sessionId) {
        SimpMessageHeaderAccessor headerAccessor = SimpMessageHeaderAccessor.create(SimpMessageType.MESSAGE);
        headerAccessor.setSessionId(sessionId);
        headerAccessor.setLeaveMutable(true);
        return headerAccessor.getMessageHeaders();
    }
}
