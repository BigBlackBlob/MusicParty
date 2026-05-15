package org.thornex.musicparty.event;

import org.springframework.context.ApplicationEvent;

public class RoomSessionEvictedEvent extends ApplicationEvent {
    private final String roomId;

    public RoomSessionEvictedEvent(Object source, String roomId) {
        super(source);
        this.roomId = roomId;
    }

    public String getRoomId() {
        return roomId;
    }
}
