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

import java.util.HashMap;
import java.util.Map;

@Component
@Slf4j
public class WebSocketAuthInterceptor implements ChannelInterceptor {
    private static final String AUTHORIZED_ROOM_ID_ATTRIBUTE = "authorizedRoomId";
    private static final String ROOM_TOPIC_PREFIX = "/topic/rooms/";

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

        if (accessor == null) {
            return message;
        }

        if (StompCommand.CONNECT.equals(accessor.getCommand())) {
            authorizeConnect(accessor);
            return message;
        }

        if (StompCommand.SUBSCRIBE.equals(accessor.getCommand())) {
            authorizeSubscribe(accessor);
        }

        return message;
    }

    private void authorizeConnect(StompHeaderAccessor accessor) {
        String roomId = accessor.getFirstNativeHeader("room-id");
        RoomService.RoomAccessMetadata metadata = roomService.getRoomAccessMetadata(roomId)
                .orElseThrow(() -> new MessageDeliveryException("UNKNOWN_ROOM"));

        if (!metadata.privateRoom()) {
            rememberAuthorizedRoom(accessor, metadata.roomId());
            return;
        }

        String sessionToken = accessor.getFirstNativeHeader("session-token");
        String roomAccessToken = accessor.getFirstNativeHeader("room-access-token");
        String publicId = userService.resolvePublicIdBySessionToken(sessionToken).orElse(null);

        if (!StringUtils.hasText(publicId) || !roomAccessService.validateAccessToken(metadata.roomId(), publicId, roomAccessToken)) {
            log.warn("WebSocket Connection Refused: Invalid room access token. Session={}, Room={}", accessor.getSessionId(), metadata.roomId());
            throw new MessageDeliveryException("INVALID_ROOM_ACCESS_TOKEN");
        }

        rememberAuthorizedRoom(accessor, metadata.roomId());
        log.info("WebSocket room access granted: session={}, room={}", accessor.getSessionId(), metadata.roomId());
    }

    private void authorizeSubscribe(StompHeaderAccessor accessor) {
        String roomId = extractRoomIdFromDestination(accessor.getDestination());
        if (!StringUtils.hasText(roomId)) {
            return;
        }

        RoomService.RoomAccessMetadata metadata = roomService.getRoomAccessMetadata(roomId)
                .orElseThrow(() -> new MessageDeliveryException("UNKNOWN_ROOM"));
        if (!metadata.privateRoom()) {
            return;
        }

        Object authorizedRoomId = sessionAttributes(accessor).get(AUTHORIZED_ROOM_ID_ATTRIBUTE);
        if (!metadata.roomId().equals(authorizedRoomId)) {
            log.warn("WebSocket subscription refused: unauthorized private room topic. Session={}, Room={}, Destination={}",
                    accessor.getSessionId(), metadata.roomId(), accessor.getDestination());
            throw new MessageDeliveryException("INVALID_ROOM_SUBSCRIPTION");
        }
    }

    private String extractRoomIdFromDestination(String destination) {
        if (!StringUtils.hasText(destination) || !destination.startsWith(ROOM_TOPIC_PREFIX)) {
            return null;
        }
        String remainder = destination.substring(ROOM_TOPIC_PREFIX.length());
        int separatorIndex = remainder.indexOf('/');
        if (separatorIndex <= 0) {
            return null;
        }
        return remainder.substring(0, separatorIndex);
    }

    private void rememberAuthorizedRoom(StompHeaderAccessor accessor, String roomId) {
        sessionAttributes(accessor).put(AUTHORIZED_ROOM_ID_ATTRIBUTE, roomId);
    }

    private Map<String, Object> sessionAttributes(StompHeaderAccessor accessor) {
        Map<String, Object> attributes = accessor.getSessionAttributes();
        if (attributes == null) {
            attributes = new HashMap<>();
            accessor.setSessionAttributes(attributes);
        }
        return attributes;
    }
}
