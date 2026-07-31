package org.thornex.musicparty.service;

import lombok.RequiredArgsConstructor;
import jakarta.annotation.PreDestroy;
import org.springframework.stereotype.Service;

import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

@Service
@RequiredArgsConstructor
public class WebSocketSessionCoordinator {

    private final UserService userService;
    private final MusicPlayerService musicPlayerService;
    private final AtomicBoolean onlineStateBroadcastPending = new AtomicBoolean();
    private final ScheduledExecutorService onlineStateBroadcaster = Executors.newSingleThreadScheduledExecutor(runnable -> {
        Thread thread = new Thread(runnable, "mp-websocket-presence");
        thread.setDaemon(true);
        return thread;
    });

    public void handleConnect(String sessionId, String sessionToken, String initialName, String roomId) {
        userService.handleConnect(sessionId, sessionToken, initialName, roomId);
        scheduleOnlineStateBroadcast();
    }

    public void handleDisconnect(String sessionId) {
        userService.disconnectUser(sessionId);
        scheduleOnlineStateBroadcast();
    }

    @PreDestroy
    void shutdown() {
        onlineStateBroadcaster.shutdownNow();
    }

    private void scheduleOnlineStateBroadcast() {
        if (!onlineStateBroadcastPending.compareAndSet(false, true)) {
            return;
        }
        onlineStateBroadcaster.schedule(() -> {
            try {
                musicPlayerService.broadcastOnlineUsers();
            } finally {
                onlineStateBroadcastPending.set(false);
            }
        // A full player-state fan-out is substantially larger than a presence
        // delta.  Leave enough room to collapse an entire reconnect burst into
        // one broadcast instead of repeatedly writing the same snapshots.
        }, 750, TimeUnit.MILLISECONDS);
    }
}
