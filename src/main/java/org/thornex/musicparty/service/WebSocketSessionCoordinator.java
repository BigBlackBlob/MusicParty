package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class WebSocketSessionCoordinator {

    private final UserService userService;
    private final MusicPlayerService musicPlayerService;

    public void handleConnect(String sessionId, String sessionToken, String initialName, String roomId) {
        userService.handleConnect(sessionId, sessionToken, initialName, roomId);
        musicPlayerService.broadcastOnlineUsers();
    }

    public void handleDisconnect(String sessionId) {
        userService.disconnectUser(sessionId);
        musicPlayerService.broadcastOnlineUsers();
    }
}
