package org.thornex.musicparty.controller;

import org.springframework.messaging.handler.annotation.Header;
import org.springframework.messaging.handler.annotation.MessageMapping;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.messaging.simp.annotation.SubscribeMapping;
import org.springframework.stereotype.Controller;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.service.ChatService;
import org.thornex.musicparty.service.MusicPlayerService;
import org.thornex.musicparty.service.MusicSocketSessionFacade;
import org.thornex.musicparty.service.RoomLifecycleService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.UserService;

import java.util.List;
import java.util.UUID;

@Controller
public class MusicSocketController {

    private final MusicPlayerService musicPlayerService;
    private final UserService userService;
    private final ChatService chatService;
    private final RoomService roomService;
    private final RoomLifecycleService roomLifecycleService;
    private final MusicSocketSessionFacade musicSocketSessionFacade;

    public MusicSocketController(MusicPlayerService musicPlayerService,
                                 UserService userService,
                                 ChatService chatService,
                                 RoomService roomService,
                                 RoomLifecycleService roomLifecycleService,
                                 MusicSocketSessionFacade musicSocketSessionFacade) {
        this.musicPlayerService = musicPlayerService;
        this.userService = userService;
        this.chatService = chatService;
        this.roomService = roomService;
        this.roomLifecycleService = roomLifecycleService;
        this.musicSocketSessionFacade = musicSocketSessionFacade;
    }

    @MessageMapping("/player/resync")
    public void requestResync(@Header("simpSessionId") String sessionId) {
        musicSocketSessionFacade.sendPlayerResync(sessionId);
    }

    @MessageMapping("/sync/ping")
    public void syncPing(@Payload SyncPingRequest request, @Header("simpSessionId") String sessionId) {
        musicSocketSessionFacade.sendSyncPong(sessionId, request);
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
        musicPlayerService.seekTo(request.positionMs(), sessionId)
                .ifPresent(message -> musicSocketSessionFacade.sendSeekDenied(sessionId, message));
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

    @MessageMapping("/queue/reorder")
    public void reorderQueue(@Payload QueueReorderRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        if (request.queueId() != null && request.targetQueueId() != null) {
            musicPlayerService.reorderQueue(request.queueId(), request.targetQueueId(), request.position(), sessionId);
            return;
        }
        musicPlayerService.reorderQueue(request.oldIndex(), request.newIndex(), sessionId);
    }

    // 点赞接口
    @MessageMapping("/control/like")
    public void likeSong(@Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        musicPlayerService.likeSong(sessionId);
    }

    @MessageMapping("/user/rename")
    public void rename(RenameRequest request, @Header("simpSessionId") String sessionId) {
        if (!musicSocketSessionFacade.renameAndBroadcast(sessionId, request.newName())) {
            musicSocketSessionFacade.sendRenameFailed(sessionId);
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
                RoomInfo room = roomService.createRoom(
                        request.name(),
                        user.getPublicId(),
                        Boolean.TRUE.equals(request.isPrivate()),
                        request.password()
                );
                musicSocketSessionFacade.sendRoomCreated(sessionId, room);
            } catch (IllegalArgumentException ex) {
                musicSocketSessionFacade.sendRoomCreateFailed(sessionId, ex.getMessage());
            }
        });
    }

    @MessageMapping("/rooms/delete")
    public void deleteRoom(RoomDeleteRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        userService.getUser(sessionId).ifPresent(user -> {
            String roomId = request.roomId();
            boolean isAdmin = roomService.isAdminPassword(request.adminPassword());
            if (roomLifecycleService.deleteRoom(roomId, user.getPublicId(), isAdmin)) {
            } else {
                musicSocketSessionFacade.sendRoomDeleteFailed(sessionId);
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
        chatService.handleRoomChat(sessionId, request);
    }

    @MessageMapping("/public-chat")
    public void handlePublicChat(ChatRequest request, @Header("simpSessionId") String sessionId) {
        if (isGuest(sessionId)) return;
        chatService.handlePublicChat(sessionId, request);
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
        musicSocketSessionFacade.sendRoomChatHistory(sessionId, history);
    }

    @MessageMapping("/public-chat/history/fetch")
    public void fetchPublicChatHistory(@Payload ChatHistoryFetchRequest request, @Header("simpSessionId") String sessionId) {
        List<ChatMessage> history = chatService.getPublicHistory(request.offset(), request.limit());
        musicSocketSessionFacade.sendPublicChatHistory(sessionId, history);
    }
}
