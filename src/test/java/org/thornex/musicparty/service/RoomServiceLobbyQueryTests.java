package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class RoomServiceLobbyQueryTests {

    @Test
    void anonymousLobbyQueryReturnsOnlyPublicRooms() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService service = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                roomRepository,
                new InMemoryMigrationStateRepository()
        );
        service.init();

        RoomInfo privateRoom = service.createRoom("Secret Room", "u_owner", true, "secret123");
        RoomInfo publicRoom = service.createRoom("Open Room", "u_owner", false, null);

        List<RoomInfo> rooms = service.listLobbyRooms(null);

        assertThat(rooms).extracting(RoomInfo::roomId)
                .contains(RoomService.DEFAULT_ROOM_ID, publicRoom.roomId())
                .doesNotContain(privateRoom.roomId());
    }

    @Test
    void ownerLobbyQueryIncludesOwnedPrivateRoomsAndSortsByActivity() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService service = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                roomRepository,
                new InMemoryMigrationStateRepository()
        );
        service.init();

        RoomInfo lessActivePublic = service.createRoom("Open Room", "u_owner", false, null);
        RoomInfo privateRoom = service.createRoom("Secret Room", "u_owner", true, "secret123");
        service.markRoomActive(lessActivePublic.roomId());
        service.markRoomActive(privateRoom.roomId());

        List<RoomInfo> rooms = service.listLobbyRooms("u_owner");

        assertThat(rooms).extracting(RoomInfo::roomId)
                .contains(privateRoom.roomId());
        assertThat(rooms.get(0).roomId()).isEqualTo(RoomService.DEFAULT_ROOM_ID);
        assertThat(rooms.stream().filter(RoomInfo::privateRoom).map(RoomInfo::roomId))
                .containsExactly(privateRoom.roomId());
    }
}
