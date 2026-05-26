package org.thornex.musicparty.service.command;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.service.stream.LiveStreamService;
import org.thornex.musicparty.service.stream.StreamTokenService;
import org.thornex.musicparty.websocket.ReactiveSocketBroker;

import java.util.UUID;

@Component
@RequiredArgsConstructor
public class StreamCommand implements ChatCommand {

    private final StreamTokenService tokenService;
    private final LiveStreamService liveStreamService;
    private final ReactiveSocketBroker broker;
    private final AppProperties appProperties;

    @Override
    public String getCommand() {
        return "stream";
    }

    @Override
    public void execute(String args, User user) {
        if (!liveStreamService.isEnabled()) {
            sendPrivateSystemMessage(user, "当前直播流服务未开启。请联系管理员启用。");
            return;
        }

        String token = tokenService.generateToken(user.getPublicId());
        
        String base = appProperties.getBaseUrl();
        if (base == null || base.isEmpty()) {
            base = "";
        } else if (base.endsWith("/")) {
            base = base.substring(0, base.length() - 1);
        }
        
        String link = base + "/radio/stream?key=" + token;
        
        String msg = String.format("直播流链接已生成（24小时有效，4小时闲置失效）： %s", link);
        sendPrivateSystemMessage(user, msg);
    }

    private void sendPrivateSystemMessage(User user, String content) {
        ChatMessage message = new ChatMessage(
                UUID.randomUUID().toString(),
                "SYSTEM",
                "SYSTEM",
                content,
                System.currentTimeMillis(),
                MessageType.SYSTEM
        );

        broker.sendToSession(user.getSessionId(), "chat.private", message);
    }
}
