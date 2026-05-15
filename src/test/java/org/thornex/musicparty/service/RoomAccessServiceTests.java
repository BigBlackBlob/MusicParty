package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.PersistedRoom;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class RoomAccessServiceTests {

    @Test
    void createsPrivateRoomWithHashedPasswordAndIssuesAccessToken() {
        AppProperties properties = new AppProperties();
        properties.setAdminPassword("admin-password-12345");
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, roomRepository, new InMemoryMigrationStateRepository());
        roomService.init();

        RoomInfo room = roomService.createRoom("Secret Room", "u_owner", true, "letmein123");
        PersistedRoom stored = roomRepository.findById(room.roomId()).orElseThrow();

        assertThat(stored.visibility()).isEqualTo("PRIVATE");
        assertThat(stored.passwordHash()).isNotBlank().isNotEqualTo("letmein123");
        assertThat(stored.passwordVersion()).isEqualTo(1);

        RoomAccessService accessService = new RoomAccessService(properties, roomService);
        RoomAccessGrant grant = accessService.verifyAccess(room.roomId(), "u_member", "letmein123");

        assertThat(grant.allowed()).isTrue();
        assertThat(grant.roomAccessToken()).isNotBlank();
        assertThat(accessService.validateAccessToken(room.roomId(), "u_member", grant.roomAccessToken())).isTrue();
    }

    @Test
    void rejectsWrongPasswordForPrivateRoom() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, roomRepository, new InMemoryMigrationStateRepository());
        roomService.init();
        RoomInfo room = roomService.createRoom("Secret Room", "u_owner", true, "letmein123");

        RoomAccessService accessService = new RoomAccessService(properties, roomService);
        RoomAccessGrant grant = accessService.verifyAccess(room.roomId(), "u_member", "wrong-password");

        assertThat(grant.allowed()).isFalse();
        assertThat(grant.roomAccessToken()).isBlank();
    }

    @Test
    void publicRoomsDoNotNeedRoomAccessToken() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, roomRepository, new InMemoryMigrationStateRepository());
        roomService.init();
        RoomInfo room = roomService.createRoom("Open Room", "u_owner", false, null);

        RoomAccessService accessService = new RoomAccessService(properties, roomService);
        RoomAccessGrant grant = accessService.verifyAccess(room.roomId(), "u_member", "");

        assertThat(grant.allowed()).isTrue();
        assertThat(grant.roomAccessToken()).isBlank();
        assertThat(accessService.validateAccessToken(room.roomId(), "u_member", null)).isTrue();
    }

    @Test
    void unknownRoomDoesNotFallBackToDefaultRoomDuringVerification() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, roomRepository, new InMemoryMigrationStateRepository());
        roomService.init();
        RoomAccessService accessService = new RoomAccessService(properties, roomService);

        assertThatThrownBy(() -> accessService.verifyAccess("missing-room", "u_member", "anything"))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessage("Room not found");
    }

    @Test
    void rotatingPrivateRoomPasswordInvalidatesExistingAccessTokens() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, roomRepository, new InMemoryMigrationStateRepository());
        roomService.init();
        RoomInfo room = roomService.createRoom("Secret Room", "u_owner", true, "letmein123");

        RoomAccessService accessService = new RoomAccessService(properties, roomService);
        RoomAccessGrant oldGrant = accessService.verifyAccess(room.roomId(), "u_member", "letmein123");

        roomService.updateRoomSettings(room.roomId(), "u_owner", false, "Secret Room", true, "new-secret", false);

        assertThat(accessService.validateAccessToken(room.roomId(), "u_member", oldGrant.roomAccessToken())).isFalse();

        RoomAccessGrant newGrant = accessService.verifyAccess(room.roomId(), "u_member", "new-secret");
        assertThat(newGrant.allowed()).isTrue();
        assertThat(accessService.validateAccessToken(room.roomId(), "u_member", newGrant.roomAccessToken())).isTrue();
    }

    @Test
    void ownerCanSwitchPrivateRoomBackToPublicWithoutPassword() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, roomRepository, new InMemoryMigrationStateRepository());
        roomService.init();
        RoomInfo room = roomService.createRoom("Secret Room", "u_owner", true, "letmein123");

        RoomInfo updated = roomService.updateRoomSettings(room.roomId(), "u_owner", false, "Open Room", false, null, false);
        PersistedRoom stored = roomRepository.findById(room.roomId()).orElseThrow();

        assertThat(updated.privateRoom()).isFalse();
        assertThat(updated.name()).isEqualTo("Open Room");
        assertThat(stored.visibility()).isEqualTo("PUBLIC");
        assertThat(stored.passwordHash()).isNull();
        assertThat(stored.passwordVersion()).isEqualTo(2);
    }

    @Test
    void nonOwnerCannotUpdateRoomSettings() {
        AppProperties properties = new AppProperties();
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(new ObjectMapper(), event -> {}, properties, roomRepository, new InMemoryMigrationStateRepository());
        roomService.init();
        RoomInfo room = roomService.createRoom("Secret Room", "u_owner", true, "letmein123");

        assertThatThrownBy(() -> roomService.updateRoomSettings(room.roomId(), "u_other", false, "Hijacked", false, null, false))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessage("No permission to update room");
    }
}
