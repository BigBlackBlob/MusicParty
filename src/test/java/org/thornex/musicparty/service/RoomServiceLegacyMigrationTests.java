package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;

import java.nio.file.Path;
import java.nio.file.Files;

import static org.assertj.core.api.Assertions.assertThat;

class RoomServiceLegacyMigrationTests {

    @Test
    void skipsLegacyRoomJsonWhenMigrationAlreadyMarkedComplete() throws Exception {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        InMemoryMigrationStateRepository migrationStateRepository = new InMemoryMigrationStateRepository();
        migrationStateRepository.markCompleted("legacy.rooms.json");

        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                roomRepository,
                migrationStateRepository
        );

        Path tmpDir = Path.of("target", "tmp");
        Files.createDirectories(tmpDir);
        Path legacyRooms = Files.createTempFile(tmpDir, "musicparty-rooms", ".json");
        Files.writeString(legacyRooms, """
                [
                  {"roomId":"room-legacy","name":"Legacy Room","creatorPublicId":"u_old","createdAt":1,"lastActiveAt":2,"system":false}
                ]
                """);

        roomService.loadRoomsForTest(legacyRooms.toFile());

        assertThat(roomRepository.findById("room-legacy")).isEmpty();
    }
}
