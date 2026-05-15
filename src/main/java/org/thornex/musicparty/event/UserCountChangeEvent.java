package org.thornex.musicparty.event;

import lombok.Getter;
import org.springframework.context.ApplicationEvent;

/**
 * 当在线用户数量发生变化时触发
 */
@Getter
public class UserCountChangeEvent extends ApplicationEvent {
    private final String roomId;
    private final int onlineUserCount;

    public UserCountChangeEvent(Object source, int onlineUserCount) {
        this(source, "lounge", onlineUserCount);
    }

    public UserCountChangeEvent(Object source, String roomId, int onlineUserCount) {
        super(source);
        this.roomId = roomId;
        this.onlineUserCount = onlineUserCount;
    }
}
