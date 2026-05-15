package org.thornex.musicparty.websocket;

import lombok.RequiredArgsConstructor;
import org.springframework.context.event.EventListener;
import org.springframework.messaging.simp.SimpMessagingTemplate;
import org.springframework.stereotype.Component;
import org.thornex.musicparty.dto.PlayerEvent;
import org.thornex.musicparty.event.PlayerStateEvent;
import org.thornex.musicparty.event.QueueUpdateEvent;
import org.thornex.musicparty.event.RoomDeletedEvent;
import org.thornex.musicparty.event.RoomListUpdateEvent;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.service.AfterCommitExecutor;
import org.thornex.musicparty.service.UserService;
import org.thornex.musicparty.util.MessageFormatter;

@Component
@RequiredArgsConstructor
public class WebSocketBroadcaster {

    private final SimpMessagingTemplate messagingTemplate;
    private final UserService userService;
    private final AfterCommitExecutor afterCommitExecutor;

    /**
     * 监听播放器完整状态变更事件
     */
    @EventListener
    public void onPlayerStateChanged(PlayerStateEvent event) {
        afterCommitExecutor.run(() ->
                messagingTemplate.convertAndSend("/topic/rooms/" + event.getRoomId() + "/player/state", event.getState()));
    }

    /**
     * 监听队列更新事件
     */
    @EventListener
    public void onQueueChanged(QueueUpdateEvent event) {
        afterCommitExecutor.run(() ->
                messagingTemplate.convertAndSend("/topic/rooms/" + event.getRoomId() + "/player/queue", event.getQueue()));
    }

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
                messagingTemplate.convertAndSend("/topic/rooms/" + event.getRoomId() + "/player/events", playerEvent));
    }

    @EventListener
    public void onRoomListChanged(RoomListUpdateEvent event) {
        afterCommitExecutor.run(() ->
                messagingTemplate.convertAndSend("/topic/rooms/list", event.getRooms()));
    }

    @EventListener
    public void onRoomDeleted(RoomDeletedEvent event) {
        PlayerEvent playerEvent = new PlayerEvent("WARN", "ROOM_DELETED", "SYSTEM", "房间已被删除，已返回 Lounge", event.getRoomId());
        afterCommitExecutor.run(() ->
                messagingTemplate.convertAndSend("/topic/rooms/" + event.getRoomId() + "/player/events", playerEvent));
    }
}
