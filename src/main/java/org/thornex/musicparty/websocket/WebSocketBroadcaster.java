package org.thornex.musicparty.websocket;

import lombok.RequiredArgsConstructor;
import jakarta.annotation.PreDestroy;
import org.springframework.context.event.EventListener;
import org.springframework.stereotype.Component;
import org.thornex.musicparty.dto.PlayerEvent;
import org.thornex.musicparty.event.PlayerStateEvent;
import org.thornex.musicparty.event.QueueUpdateEvent;
import org.thornex.musicparty.event.RoomDeletedEvent;
import org.thornex.musicparty.event.RoomListUpdateEvent;
import org.thornex.musicparty.event.RoomPlaylistUpdateEvent;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.service.AfterCommitExecutor;
import org.thornex.musicparty.service.UserService;
import org.thornex.musicparty.util.MessageFormatter;

import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

@Component
@RequiredArgsConstructor
public class WebSocketBroadcaster {

    private final ReactiveSocketBroker broker;
    private final UserService userService;
    private final AfterCommitExecutor afterCommitExecutor;
    private final Map<String, PlayerStateEvent> pendingPlayerStates = new ConcurrentHashMap<>();
    private volatile List<org.thornex.musicparty.dto.RoomInfo> pendingRoomList;
    private final AtomicBoolean stateFlushScheduled = new AtomicBoolean();
    private final ScheduledExecutorService stateFlushExecutor = Executors.newSingleThreadScheduledExecutor(runnable -> {
        Thread thread = new Thread(runnable, "mp-websocket-state-flush");
        thread.setDaemon(true);
        return thread;
    });

    /**
     * 监听播放器完整状态变更事件
     */
    @EventListener
    public void onPlayerStateChanged(PlayerStateEvent event) {
        afterCommitExecutor.run(() -> {
            pendingPlayerStates.put(event.getRoomId(), event);
            scheduleStateFlush();
        });
    }

    /**
     * 监听队列更新事件
     */
    @EventListener
    public void onQueueChanged(QueueUpdateEvent event) {
        afterCommitExecutor.run(() ->
                broker.broadcastRoom(event.getRoomId(), "queue.patch", new QueuePatchPayload(
                        event.getPatch().operation(), event.getQueueVersion(), event.getQueue(), event.getPatch().items(),
                        event.getPatch().queueIds(), event.getPatch().queueId(), event.getPatch().targetQueueId(), event.getPatch().position())));
    }

    private record QueuePatchPayload(String operation, long queueVersion,
                                     java.util.List<org.thornex.musicparty.dto.MusicQueueItem> queue,
                                     java.util.List<org.thornex.musicparty.dto.MusicQueueItem> items,
                                     java.util.List<String> queueIds, String queueId, String targetQueueId, String position) {}

    /**
     * 监听系统消息事件（用于 Toast 通知等）
     */
    @EventListener
    public void onSystemMessage(SystemMessageEvent event) {
        // 将内部的 SystemMessageEvent 转换为对外的 PlayerEvent DTO
        String actionCode = event.getAction() != null ? event.getAction().name() : "";
        String type = event.getLevel().name();

        String userName = "SYSTEM";
        if (!"SYSTEM".equals(event.getUserId())) {
            userName = userService.getUserByPublicId(event.getUserId())
                    .map(org.thornex.musicparty.dto.User::getName)
                    .orElse("Unknown");
        }

        String formattedMessage = MessageFormatter.format(event, userName);

        // 特殊处理密码修改的广播
        if ("PASSWORD_CHANGED".equals(event.getPayload())) {
            actionCode = "PASSWORD_CHANGED";
            type = "ERROR";
        }

        PlayerEvent playerEvent = new PlayerEvent(
                type,
                actionCode,
                event.getUserId(),
                formattedMessage,
                event.getPayload()
        );
        afterCommitExecutor.run(() ->
                broker.broadcastRoom(event.getRoomId(), "player.events", playerEvent));
    }

    @EventListener
    public void onRoomListChanged(RoomListUpdateEvent event) {
        afterCommitExecutor.run(() -> {
            pendingRoomList = event.getRooms();
            scheduleStateFlush();
        });
    }

    @PreDestroy
    void shutdown() {
        stateFlushExecutor.shutdownNow();
    }

    private void scheduleStateFlush() {
        if (stateFlushScheduled.compareAndSet(false, true)) {
            stateFlushExecutor.schedule(this::flushLatestState, 200, TimeUnit.MILLISECONDS);
        }
    }

    private void flushLatestState() {
        try {
            Map<String, PlayerStateEvent> states = Map.copyOf(pendingPlayerStates);
            states.forEach(pendingPlayerStates::remove);
            states.forEach((roomId, event) -> broker.broadcastRoom(roomId, "player.state", event.getState()));
            List<org.thornex.musicparty.dto.RoomInfo> rooms = pendingRoomList;
            pendingRoomList = null;
            if (rooms != null) {
                broker.broadcastAll("rooms.list", rooms);
            }
        } finally {
            stateFlushScheduled.set(false);
            if (!pendingPlayerStates.isEmpty() || pendingRoomList != null) {
                scheduleStateFlush();
            }
        }
    }

    @EventListener
    public void onRoomDeleted(RoomDeletedEvent event) {
        PlayerEvent playerEvent = new PlayerEvent("WARN", "ROOM_DELETED", "SYSTEM", "房间已被删除，已返回 Lounge", event.getRoomId());
        afterCommitExecutor.run(() ->
                broker.broadcastRoom(event.getRoomId(), "player.events", playerEvent));
    }

    @EventListener
    public void onRoomPlaylistsChanged(RoomPlaylistUpdateEvent event) {
        PlayerEvent playerEvent = new PlayerEvent("INFO", "ROOM_PLAYLISTS_UPDATE", "SYSTEM", "", "room-playlists:update");
        afterCommitExecutor.run(() ->
                broker.broadcastRoom(event.getRoomId(), "player.events", playerEvent));
    }
}
