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
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomService;

@Component
@Slf4j
public class WebSocketAuthInterceptor implements ChannelInterceptor {

    private final AccountService accountService;
    private final RoomService roomService;
    private final RoomAccessService roomAccessService;

    public WebSocketAuthInterceptor(AccountService accountService, RoomService roomService, RoomAccessService roomAccessService) {
        this.accountService = accountService;
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

        String sessionToken = accessor.getFirstNativeHeader("session-token");
        var accountSession = accountService.resolveSession(sessionToken)
                .orElseThrow(() -> new MessageDeliveryException("INVALID_ACCOUNT_SESSION"));
        if (!metadata.privateRoom()) {
            return message;
        }

        String roomAccessToken = accessor.getFirstNativeHeader("room-access-token");
        String publicId = accountSession.publicId();

        if (!StringUtils.hasText(publicId) || !roomAccessService.validateAccessToken(metadata.roomId(), publicId, roomAccessToken)) {
            log.warn("WebSocket Connection Refused: Invalid room access token. Session={}, Room={}", accessor.getSessionId(), metadata.roomId());
            throw new MessageDeliveryException("INVALID_ROOM_ACCESS_TOKEN");
        }

        log.info("WebSocket room access granted: session={}, room={}", accessor.getSessionId(), metadata.roomId());
        return message;
    }
}
