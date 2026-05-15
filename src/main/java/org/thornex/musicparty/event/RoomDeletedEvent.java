package org.thornex.musicparty.event;

import lombok.Getter;
import org.springframework.context.ApplicationEvent;

@Getter
public class RoomDeletedEvent extends ApplicationEvent {
    private final String roomId;

    public RoomDeletedEvent(Object source, String roomId) {
        super(source);
        this.roomId = roomId;
    }
}
