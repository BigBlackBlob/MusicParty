package org.thornex.musicparty.controller;

import org.springframework.messaging.handler.annotation.Header;
import org.springframework.messaging.handler.annotation.MessageMapping;
import org.springframework.messaging.handler.annotation.Payload;
import org.springframework.messaging.simp.annotation.SubscribeMapping;
import org.springframework.stereotype.Controller;
import lombok.extern.slf4j.Slf4j;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.service.ChatService;
import org.thornex.musicparty.service.MusicPlayerService;
import org.thornex.musicparty.service.MusicPlayerService.ControlResult;
import org.thornex.musicparty.service.MusicSocketSessionFacade;
import org.thornex.musicparty.service.RoomLifecycleService;
import org.thornex.musicparty.service.RoomPlaylistService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.SocketRateLimiter;
import org.thornex.musicparty.service.UserService;

import java.util.List;
import java.util.UUID;

@Controller
@Slf4j
public class MusicSocketController {

    private final MusicPlayerService musicPlayerService;
    private final UserService userService;
    private final ChatService chatService;
    private final RoomService roomService;
    private final RoomLifecycleService roomLifecycleService;
    private final MusicSocketSessionFacade musicSocketSessionFacade;
    private final SocketRateLimiter socketRateLimiter;
    private final RoomPlaylistService roomPlaylistService;

    public MusicSocketController(MusicPlayerService musicPlayerService,
                                 UserService userService,
                                 ChatService chatService,
                                 RoomService roomService,
                                 RoomLifecycleService roomLifecycleService,
                                 MusicSocketSessionFacade musicSocketSessionFacade,
                                 SocketRateLimiter socketRateLimiter,
                                 RoomPlaylistService roomPlaylistService) {
        this.musicPlayerService = musicPlayerService;
        this.userService = userService;
        this.chatService = chatService;
        this.roomService = roomService;
        this.roomLifecycleService = roomLifecycleService;
        this.musicSocketSessionFacade = musicSocketSessionFacade;
        this.socketRateLimiter = socketRateLimiter;
        this.roomPlaylistService = roomPlaylistService;
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
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加歌曲")) return;
        musicPlayerService.enqueue(request, sessionId);
    }

    @MessageMapping("/enqueue/playlist")
    public void enqueuePlaylist(EnqueuePlaylistRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加歌单")) return;
        musicPlayerService.enqueuePlaylist(request, sessionId);
    }

    @MessageMapping("/enqueue/room-playlist")
    public void enqueueRoomPlaylist(org.thornex.musicparty.dto.RoomPlaylistRequests.EnqueueRoomPlaylistRequest request,
                                    @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加歌单")) return;
        String roomId = userService.getRoomIdForSession(sessionId);
        musicPlayerService.enqueueSavedPlaylist(roomPlaylistService.getPlaylistMusics(roomId, request.playlistId()), sessionId);
    }

    @MessageMapping("/enqueue/album")
    public void enqueueAlbum(EnqueueAlbumRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加专辑")) return;
        musicPlayerService.enqueueAlbum(request, sessionId);
    }

    @MessageMapping("/control/next")
    public void nextSong(@Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再切歌")) {
            logControlResult("next", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        ControlResult result = musicPlayerService.skipToNext(sessionId);
        logControlResult("next", sessionId, result, null);
        sendControlResultIfDenied(sessionId, "CONTROL_DENIED", result);
    }

    @MessageMapping("/control/toggle-shuffle")
    public void toggleShuffle(@Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再切换随机播放")) {
            logControlResult("shuffle", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        ControlResult result = musicPlayerService.toggleShuffle(sessionId);
        logControlResult("shuffle", sessionId, result, null);
        sendControlResultIfDenied(sessionId, "CONTROL_DENIED", result);
    }

    @MessageMapping("/control/toggle-pause")
    public void togglePause(@Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再控制播放")) {
            logControlResult("pause", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        ControlResult result = musicPlayerService.togglePause(sessionId);
        logControlResult("pause", sessionId, result, null);
        sendControlResultIfDenied(sessionId, "CONTROL_DENIED", result);
    }

    @MessageMapping("/control/seek")
    public void seek(@Payload SeekRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再调整进度")) {
            logControlResult("seek", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        var denial = musicPlayerService.seekTo(request.positionMs(), sessionId);
        if (denial.isPresent()) {
            log.info("Playback control seek from session {} rejected: result=DENIED, reason={}, positionMs={}", sessionId, denial.get(), request.positionMs());
        } else {
            log.debug("Playback control seek from session {} accepted: positionMs={}", sessionId, request.positionMs());
        }
        denial.ifPresent(message -> musicSocketSessionFacade.sendSeekDenied(sessionId, message));
    }

    @MessageMapping("/queue/top")
    public void topSong(@Payload QueueActionRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.topSong(request.queueId(), sessionId);
    }

    @MessageMapping("/queue/batch-top")
    public void topSongs(@Payload QueueBatchActionRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.topSongs(request.queueIds(), sessionId);
    }

    @MessageMapping("/queue/remove")
    public void removeSong(@Payload QueueActionRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.removeSongFromQueue(request.queueId(), sessionId);
    }

    @MessageMapping("/queue/batch-remove")
    public void removeSongs(@Payload QueueBatchActionRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.removeSongsFromQueue(request.queueIds(), sessionId);
    }

    @MessageMapping("/queue/reorder")
    public void reorderQueue(@Payload QueueReorderRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        if (request.queueId() != null && request.targetQueueId() != null) {
            musicPlayerService.reorderQueue(request.queueId(), request.targetQueueId(), request.position(), sessionId);
            return;
        }
        musicPlayerService.reorderQueue(request.oldIndex(), request.newIndex(), sessionId);
    }

    // 点赞接口
    @MessageMapping("/control/like")
    public void likeSong(@Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再点赞")) return;
        musicPlayerService.likeSong(sessionId);
    }

    @MessageMapping("/user/rename")
    public void rename(RenameRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "profile")) return;
        if (!musicSocketSessionFacade.renameAndBroadcast(sessionId, request.newName())) {
            musicSocketSessionFacade.sendRenameFailed(sessionId);
        }
    }

    @MessageMapping("/user/bind")
    public void bindAccount(BindRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "profile")) return;
        userService.bindAccount(sessionId, request.platform(), request.accountId());
    }

    @MessageMapping("/rooms/create")
    public void createRoom(RoomCreateRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "room")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再创建房间")) return;
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
        if (denyRateLimited(sessionId, "room")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再删除房间")) return;
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

    private boolean denyGuest(String sessionId, String action, String message) {
        if (!isGuest(sessionId)) return false;
        musicSocketSessionFacade.sendControlDenied(sessionId, action, message);
        return true;
    }

    private void sendControlResultIfDenied(String sessionId, String action, ControlResult result) {
        if (result == null || result == ControlResult.OK) return;
        String message = switch (result) {
            case COOLDOWN -> "操作太快了，稍等一下再试";
            case LOCKED -> "当前控制已被锁定";
            case EMPTY_QUEUE -> "当前没有正在播放的歌曲，队列也是空的";
            case OK -> "";
        };
        musicSocketSessionFacade.sendControlDenied(sessionId, action, message);
    }

    private void logControlResult(String control, String sessionId, ControlResult result, String reason) {
        if (result == ControlResult.OK) {
            log.debug("Playback control {} from session {} accepted", control, sessionId);
            return;
        }
        log.info("Playback control {} from session {} rejected: result={}, reason={}", control, sessionId, result, reason);
    }

    // 聊天消息处理
    @MessageMapping("/chat")
    public void handleChat(ChatRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "chat")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再发送聊天")) return;
        chatService.handleRoomChat(sessionId, request);
    }

    @MessageMapping("/public-chat")
    public void handlePublicChat(ChatRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "chat")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再发送聊天")) return;
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
        if (denyRateLimited(sessionId, "chat")) return;
        List<ChatMessage> history = chatService.getHistory(userService.getRoomIdForSession(sessionId), request.offset(), request.limit());
        musicSocketSessionFacade.sendRoomChatHistory(sessionId, history);
    }

    @MessageMapping("/public-chat/history/fetch")
    public void fetchPublicChatHistory(@Payload ChatHistoryFetchRequest request, @Header("simpSessionId") String sessionId) {
        if (denyRateLimited(sessionId, "chat")) return;
        List<ChatMessage> history = chatService.getPublicHistory(request.offset(), request.limit());
        musicSocketSessionFacade.sendPublicChatHistory(sessionId, history);
    }

    private boolean denyRateLimited(String sessionId, String action) {
        if (socketRateLimiter.allow(sessionId, action)) {
            return false;
        }
        log.info("Socket action from session {} rejected by rate limiter: action={}", sessionId, action);
        musicSocketSessionFacade.sendControlDenied(sessionId, "CONTROL_DENIED", "操作太快了，稍等一下再试");
        return true;
    }
}
