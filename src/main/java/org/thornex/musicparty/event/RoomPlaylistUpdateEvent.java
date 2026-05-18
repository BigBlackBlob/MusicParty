package org.thornex.musicparty.event;

import org.springframework.context.ApplicationEvent;

public class RoomPlaylistUpdateEvent extends ApplicationEvent {
    private final String roomId;

    public RoomPlaylistUpdateEvent(Object source, String roomId) {
        super(source);
        this.roomId = roomId;
    }

    public String getRoomId() {
        return roomId;
    }
}
