package org.thornex.musicparty.controller;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Component;
import lombok.extern.slf4j.Slf4j;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.service.ChatService;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.MusicPlayerService;
import org.thornex.musicparty.service.MusicPlayerService.ControlResult;
import org.thornex.musicparty.service.MusicSocketSessionFacade;
import org.thornex.musicparty.service.RoomLifecycleService;
import org.thornex.musicparty.service.RoomCommandCoordinator;
import org.thornex.musicparty.service.RoomPlaylistService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.SocketRateLimiter;
import org.thornex.musicparty.service.UserService;

import java.util.List;
import java.util.UUID;
import java.util.concurrent.CompletableFuture;

@Component
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
    private final AccountService accountService;
    private final ObjectMapper objectMapper;
    private final RoomCommandCoordinator roomCommandCoordinator;

    public MusicSocketController(MusicPlayerService musicPlayerService,
                                 UserService userService,
                                 ChatService chatService,
                                 RoomService roomService,
                                 RoomLifecycleService roomLifecycleService,
                                 MusicSocketSessionFacade musicSocketSessionFacade,
                                 SocketRateLimiter socketRateLimiter,
                                 RoomPlaylistService roomPlaylistService,
                                  AccountService accountService,
                                  ObjectMapper objectMapper,
                                  RoomCommandCoordinator roomCommandCoordinator) {
        this.musicPlayerService = musicPlayerService;
        this.userService = userService;
        this.chatService = chatService;
        this.roomService = roomService;
        this.roomLifecycleService = roomLifecycleService;
        this.musicSocketSessionFacade = musicSocketSessionFacade;
        this.socketRateLimiter = socketRateLimiter;
        this.roomPlaylistService = roomPlaylistService;
        this.accountService = accountService;
        this.objectMapper = objectMapper;
        this.roomCommandCoordinator = roomCommandCoordinator;
    }

    public void dispatch(String type, JsonNode payload, String sessionId) {
        dispatchAsync(type, payload, sessionId).join();
    }

    public CompletableFuture<Void> dispatchAsync(String type, JsonNode payload, String sessionId) {
        if (isRoomMutation(type)) {
            return roomCommandCoordinator.executeAsync(userService.getRoomIdForSession(sessionId), () -> {
                dispatchNow(type, payload, sessionId);
                return null;
            });
        }
        dispatchNow(type, payload, sessionId);
        return CompletableFuture.completedFuture(null);
    }

    private void dispatchNow(String type, JsonNode payload, String sessionId) {
        switch (type) {
            case "player.resync", "/player/resync" -> requestResync(sessionId);
            case "sync.ping", "/sync/ping" -> syncPing(read(payload, SyncPingRequest.class), sessionId);
            case "user.me", "/user/me" -> sendCurrentUser(sessionId);
            case "users.online", "/users/online" -> sendOnlineUsers(sessionId);
            case "enqueue", "/enqueue" -> enqueue(read(payload, EnqueueRequest.class), sessionId);
            case "enqueue.playlist", "/enqueue/playlist" -> enqueuePlaylist(read(payload, EnqueuePlaylistRequest.class), sessionId);
            case "enqueue.room-playlist", "/enqueue/room-playlist" -> enqueueRoomPlaylist(read(payload, org.thornex.musicparty.dto.RoomPlaylistRequests.EnqueueRoomPlaylistRequest.class), sessionId);
            case "enqueue.album", "/enqueue/album" -> enqueueAlbum(read(payload, EnqueueAlbumRequest.class), sessionId);
            case "control.next", "/control/next" -> nextSong(sessionId);
            case "control.toggle-shuffle", "/control/toggle-shuffle" -> toggleShuffle(sessionId);
            case "control.toggle-pause", "/control/toggle-pause" -> togglePause(sessionId);
            case "control.seek", "/control/seek" -> seek(read(payload, SeekRequest.class), sessionId);
            case "queue.top", "/queue/top" -> topSong(read(payload, QueueActionRequest.class), sessionId);
            case "queue.batch-top", "/queue/batch-top" -> topSongs(read(payload, QueueBatchActionRequest.class), sessionId);
            case "queue.remove", "/queue/remove" -> removeSong(read(payload, QueueActionRequest.class), sessionId);
            case "queue.batch-remove", "/queue/batch-remove" -> removeSongs(read(payload, QueueBatchActionRequest.class), sessionId);
            case "queue.reorder", "/queue/reorder" -> reorderQueue(read(payload, QueueReorderRequest.class), sessionId);
            case "control.like", "/control/like" -> likeSong(sessionId);
            case "user.rename", "/user/rename" -> rename(read(payload, RenameRequest.class), sessionId);
            case "user.bind", "/user/bind" -> bindAccount(read(payload, BindRequest.class), sessionId);
            case "rooms.create", "/rooms/create" -> createRoom(read(payload, RoomCreateRequest.class), sessionId);
            case "rooms.delete", "/rooms/delete" -> deleteRoom(read(payload, RoomDeleteRequest.class), sessionId);
            case "chat.message", "/chat" -> handleChat(read(payload, ChatRequest.class), sessionId);
            case "public-chat.message", "/public-chat" -> handlePublicChat(read(payload, ChatRequest.class), sessionId);
            case "chat.history.fetch", "/chat/history/fetch" -> fetchChatHistory(read(payload, ChatHistoryFetchRequest.class), sessionId);
            case "public-chat.history.fetch", "/public-chat/history/fetch" -> fetchPublicChatHistory(read(payload, ChatHistoryFetchRequest.class), sessionId);
            default -> log.debug("Ignoring unknown websocket message type={} from session={}", type, sessionId);
        }
    }

    private boolean isRoomMutation(String type) {
        return switch (type) {
            case "enqueue", "/enqueue", "enqueue.playlist", "/enqueue/playlist", "enqueue.room-playlist", "/enqueue/room-playlist",
                    "enqueue.album", "/enqueue/album", "control.next", "/control/next", "control.toggle-shuffle", "/control/toggle-shuffle",
                    "control.toggle-pause", "/control/toggle-pause", "control.seek", "/control/seek", "queue.top", "/queue/top",
                    "queue.batch-top", "/queue/batch-top", "queue.remove", "/queue/remove", "queue.batch-remove", "/queue/batch-remove",
                    "queue.reorder", "/queue/reorder", "control.like", "/control/like" -> true;
            default -> false;
        };
    }

    private <T> T read(JsonNode payload, Class<T> type) {
        return objectMapper.convertValue(payload == null || payload.isMissingNode() ? objectMapper.createObjectNode() : payload, type);
    }

    public void requestResync(String sessionId) {
        musicSocketSessionFacade.sendPlayerResync(sessionId);
    }

    public void syncPing(SyncPingRequest request, String sessionId) {
        musicSocketSessionFacade.sendSyncPong(sessionId, request);
    }

    public void sendCurrentUser(String sessionId) {
        musicSocketSessionFacade.sendCurrentUser(sessionId);
    }

    public void sendOnlineUsers(String sessionId) {
        musicSocketSessionFacade.sendOnlineUsers(sessionId);
    }

    public void enqueue(EnqueueRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加歌曲")) return;
        musicPlayerService.enqueue(request, sessionId);
    }

    public void enqueuePlaylist(EnqueuePlaylistRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加歌单")) return;
        musicPlayerService.enqueuePlaylist(request, sessionId);
    }

    public void enqueueRoomPlaylist(org.thornex.musicparty.dto.RoomPlaylistRequests.EnqueueRoomPlaylistRequest request,
                                    String sessionId) {
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加歌单")) return;
        String roomId = userService.getRoomIdForSession(sessionId);
        musicPlayerService.enqueueSavedPlaylist(roomPlaylistService.getPlaylistMusics(roomId, request.playlistId()), sessionId);
    }

    public void enqueueAlbum(EnqueueAlbumRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "enqueue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再添加专辑")) return;
        musicPlayerService.enqueueAlbum(request, sessionId);
    }

    public void nextSong(String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再切歌")) {
            logControlResult("next", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        ControlResult result = musicPlayerService.skipToNext(sessionId);
        logControlResult("next", sessionId, result, null);
        sendControlResultIfDenied(sessionId, "CONTROL_DENIED", result);
    }

    public void toggleShuffle(String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再切换随机播放")) {
            logControlResult("shuffle", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        ControlResult result = musicPlayerService.toggleShuffle(sessionId);
        logControlResult("shuffle", sessionId, result, null);
        sendControlResultIfDenied(sessionId, "CONTROL_DENIED", result);
    }

    public void togglePause(String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再控制播放")) {
            logControlResult("pause", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        ControlResult result = musicPlayerService.togglePause(sessionId);
        logControlResult("pause", sessionId, result, null);
        sendControlResultIfDenied(sessionId, "CONTROL_DENIED", result);
    }

    public void seek(SeekRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再调整进度")) {
            logControlResult("seek", sessionId, ControlResult.LOCKED, "guest");
            return;
        }
        boolean adminOverride = userService.getUser(sessionId)
                .map(user -> accountService.isAdminSession(user.getSessionToken()))
                .orElse(false);
        var denial = musicPlayerService.seekTo(request.positionMs(), sessionId, adminOverride);
        if (denial.isPresent()) {
            log.info("Playback control seek from session {} rejected: result=DENIED, reason={}, positionMs={}", sessionId, denial.get(), request.positionMs());
        } else {
            log.debug("Playback control seek from session {} accepted: positionMs={}", sessionId, request.positionMs());
        }
        denial.ifPresent(message -> musicSocketSessionFacade.sendSeekDenied(sessionId, message));
    }

    public void topSong(QueueActionRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.topSong(request.queueId(), sessionId);
    }

    public void topSongs(QueueBatchActionRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.topSongs(request.queueIds(), sessionId);
    }

    public void removeSong(QueueActionRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.removeSongFromQueue(request.queueId(), sessionId);
    }

    public void removeSongs(QueueBatchActionRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "queue")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) return;
        musicPlayerService.removeSongsFromQueue(request.queueIds(), sessionId);
    }

    public void reorderQueue(QueueReorderRequest request, String sessionId) {
        if (denyQueueReorderRateLimited(sessionId, request.mutationId())) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再操作队列")) {
            musicSocketSessionFacade.sendQueueReorderNack(sessionId, request.mutationId(), "GUEST");
            return;
        }
        boolean changed;
        if (request.queueId() != null && request.targetQueueId() != null) {
            changed = musicPlayerService.reorderQueue(request.queueId(), request.targetQueueId(), request.position(), sessionId);
        } else {
            changed = musicPlayerService.reorderQueue(request.oldIndex(), request.newIndex(), sessionId);
        }
        if (changed) {
            musicSocketSessionFacade.sendQueueReorderAck(sessionId, request.mutationId());
        } else {
            musicSocketSessionFacade.sendQueueReorderNack(sessionId, request.mutationId(), "STALE_QUEUE");
        }
    }

    // 点赞接口
    public void likeSong(String sessionId) {
        if (denyRateLimited(sessionId, "control")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再点赞")) return;
        musicPlayerService.likeSong(sessionId);
    }

    public void rename(RenameRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "profile")) return;
        if (!musicSocketSessionFacade.renameAndBroadcast(sessionId, request.newName())) {
            musicSocketSessionFacade.sendRenameFailed(sessionId);
        }
    }

    public void bindAccount(BindRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "profile")) return;
        userService.bindAccount(sessionId, request.platform(), request.accountId());
    }

    public void createRoom(RoomCreateRequest request, String sessionId) {
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

    public void deleteRoom(RoomDeleteRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "room")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再删除房间")) return;
        userService.getUser(sessionId).ifPresent(user -> {
            String roomId = request.roomId();
            boolean isAdmin = accountService.isAdminSession(user.getSessionToken());
            if (roomLifecycleService.deleteRoom(roomId, user.getPublicId(), isAdmin)) {
            } else {
                musicSocketSessionFacade.sendRoomDeleteFailed(sessionId);
            }
        });
    }

    public PlayerState getInitialPlayerState() {
        return musicPlayerService.getCurrentPlayerState(RoomService.DEFAULT_ROOM_ID);
    }

    public List<UserSummary> getInitialOnlineUsers() {
        return userService.getOnlineUserSummaries(RoomService.DEFAULT_ROOM_ID);
    }

    public CurrentUserResponse getMyUserInfo(String sessionId) {
        return userService.getUser(sessionId)
                .map(u -> {
                    String role = accountService.roleForPublicId(u.getPublicId()).orElse("GUEST");
                    return new CurrentUserResponse(u.getSessionToken(), u.getPublicId(), u.getName(), u.isGuest(), role, "ADMIN".equals(role));
                })
                .orElse(new CurrentUserResponse("", "", "Unknown", true, "GUEST", false));
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
    public void handleChat(ChatRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "chat")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再发送聊天")) return;
        chatService.handleRoomChat(sessionId, request);
    }

    public void handlePublicChat(ChatRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "chat")) return;
        if (denyGuest(sessionId, "CONTROL_DENIED", "请先设置昵称再发送聊天")) return;
        chatService.handlePublicChat(sessionId, request);
    }

    // 订阅时获取历史记录
    public List<ChatMessage> getChatHistory() {
        return chatService.getHistory(RoomService.DEFAULT_ROOM_ID, 0, 50);
    }

    // 处理分页获取历史记录的请求
    public void fetchChatHistory(ChatHistoryFetchRequest request, String sessionId) {
        if (denyRateLimited(sessionId, "chat")) return;
        List<ChatMessage> history = chatService.getHistory(userService.getRoomIdForSession(sessionId), request.offset(), request.limit());
        musicSocketSessionFacade.sendRoomChatHistory(sessionId, history);
    }

    public void fetchPublicChatHistory(ChatHistoryFetchRequest request, String sessionId) {
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

    private boolean denyQueueReorderRateLimited(String sessionId, String mutationId) {
        if (socketRateLimiter.allow(sessionId, "queue.reorder")) {
            return false;
        }
        log.info("Socket queue reorder from session {} rejected by rate limiter", sessionId);
        musicSocketSessionFacade.sendQueueReorderNack(sessionId, mutationId, "RATE_LIMITED");
        return true;
    }
}
