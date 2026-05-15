package org.thornex.musicparty.event;

import lombok.Getter;
import org.springframework.context.ApplicationEvent;
import org.thornex.musicparty.dto.RoomInfo;

import java.util.List;

@Getter
public class RoomListUpdateEvent extends ApplicationEvent {
    private final List<RoomInfo> rooms;

    public RoomListUpdateEvent(Object source, List<RoomInfo> rooms) {
        super(source);
        this.rooms = rooms;
    }
}
