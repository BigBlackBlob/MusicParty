package org.thornex.musicparty.config;

import lombok.extern.slf4j.Slf4j;
import org.springframework.messaging.Message;
import org.springframework.messaging.MessageChannel;
import org.springframework.messaging.MessageDeliveryException;
import org.springframework.messaging.simp.stomp.StompCommand;
import org.springframework.messaging.simp.stomp.StompHeaderAccessor;
import org.springframework.messaging.support.ChannelInterceptor;
import org.springframework.messaging.support.MessageHeaderAccessor;
import org.springframework.stereotype.Component;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.UserService;

@Component
@Slf4j
public class WebSocketAuthInterceptor implements ChannelInterceptor {

    private final UserService userService;
    private final RoomService roomService;
    private final RoomAccessService roomAccessService;

    public WebSocketAuthInterceptor(UserService userService, RoomService roomService, RoomAccessService roomAccessService) {
        this.userService = userService;
        this.roomService = roomService;
        this.roomAccessService = roomAccessService;
    }

    @Override
    public Message<?> preSend(Message<?> message, MessageChannel channel) {
        StompHeaderAccessor accessor = MessageHeaderAccessor.getAccessor(message, StompHeaderAccessor.class);

        if (accessor == null || !StompCommand.CONNECT.equals(accessor.getCommand())) {
            return message;
        }

        String roomId = accessor.getFirstNativeHeader("room-id");
        RoomService.RoomAccessMetadata metadata = roomService.getRoomAccessMetadata(roomId)
                .orElseThrow(() -> new MessageDeliveryException("UNKNOWN_ROOM"));

        if (!metadata.privateRoom()) {
            return message;
        }

        String sessionToken = accessor.getFirstNativeHeader("session-token");
        String roomAccessToken = accessor.getFirstNativeHeader("room-access-token");
        String publicId = userService.resolvePublicIdBySessionToken(sessionToken).orElse(null);

        if (!StringUtils.hasText(publicId) || !roomAccessService.validateAccessToken(metadata.roomId(), publicId, roomAccessToken)) {
            log.warn("WebSocket Connection Refused: Invalid room access token. Session={}, Room={}", accessor.getSessionId(), metadata.roomId());
            throw new MessageDeliveryException("INVALID_ROOM_ACCESS_TOKEN");
        }

        log.info("WebSocket room access granted: session={}, room={}", accessor.getSessionId(), metadata.roomId());
        return message;
    }
}
