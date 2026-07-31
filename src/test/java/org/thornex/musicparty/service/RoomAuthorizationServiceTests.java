package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.persistence.InMemoryRoomAccessRepository;
import org.thornex.musicparty.persistence.PersistedRoomMembership;

import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class RoomAuthorizationServiceTests {

    @Test
    void publicRoomsAllowAuthenticatedMembersWithoutExplicitMembership() {
        InMemoryRoomAccessRepository access = new InMemoryRoomAccessRepository();
        AccountService accounts = mock(AccountService.class);
        RoomService rooms = mock(RoomService.class);
        when(accounts.resolveSession("member-token")).thenReturn(Optional.empty());
        when(rooms.getRoomAccessMetadata("lounge"))
                .thenReturn(Optional.of(new RoomService.RoomAccessMetadata("lounge", false, null, 0)));

        RoomAuthorizationService authorization = new RoomAuthorizationService(access, accounts, rooms);

        assertThat(authorization.canAccessRoom("lounge", "u_member", "member-token")).isTrue();
    }

    @Test
    void privateRoomsRequireMembershipUnlessSessionIsPlatformAdmin() {
        InMemoryRoomAccessRepository access = new InMemoryRoomAccessRepository();
        AccountService accounts = mock(AccountService.class);
        RoomService rooms = mock(RoomService.class);
        when(rooms.getRoomAccessMetadata("room-1"))
                .thenReturn(Optional.of(new RoomService.RoomAccessMetadata("room-1", true, null, 1)));
        when(accounts.resolveSession("admin-token")).thenReturn(Optional.of(new AccountSession(
                "admin-token", "u_admin", "admin", "Admin", "PLATFORM_ADMIN", false, true, 1L)));

        RoomAuthorizationService authorization = new RoomAuthorizationService(access, accounts, rooms);

        assertThat(authorization.canAccessRoom("room-1", "u_member", "member-token")).isFalse();
        access.upsertMembership(new PersistedRoomMembership("room-1", "u_member", "MEMBER", 1L, 1L));
        assertThat(authorization.canAccessRoom("room-1", "u_member", "member-token")).isTrue();
        assertThat(authorization.canAccessRoom("room-1", "u_admin", "admin-token")).isTrue();
    }
}
