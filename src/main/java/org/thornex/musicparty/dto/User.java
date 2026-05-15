package org.thornex.musicparty.dto;

import lombok.Data;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

@Data
public class User {
    private final String sessionToken;
    private final String publicId;
    private String sessionId;   // 当前的 WebSocket 会话 ID (会变)
    private String roomId = "lounge";
    private String name;
    private boolean isGuest;
    private long lastActiveTime;
    private final Map<String, String> bindings = new ConcurrentHashMap<>();

    public User(String sessionToken, String publicId, String sessionId, String name) {
        this.sessionToken = sessionToken;
        this.publicId = publicId;
        this.sessionId = sessionId;
        this.name = name;
        this.isGuest = name == null || name.trim().toLowerCase().startsWith("guest") || name.trim().startsWith("游客");
        this.lastActiveTime = System.currentTimeMillis();
    }
}

