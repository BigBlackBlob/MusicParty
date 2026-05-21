package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomLifecycleService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.UserService;

import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class RoomControllerTests {

    @Test
    void adminCanDeleteAnotherUsersRoom() {
        TestContext context = new TestContext();
        when(context.userService.resolvePublicIdBySessionToken("admin-token")).thenReturn(Optional.of("u_admin"));
        when(context.accountService.isAdminSession("admin-token")).thenReturn(true);
        when(context.roomLifecycleService.deleteRoom("room-1", "u_admin", true)).thenReturn(true);

        var response = context.controller.deleteRoom("room-1", "admin-token");

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
        verify(context.roomLifecycleService).deleteRoom("room-1", "u_admin", true);
    }

    @Test
    void ownerCanDeleteOwnRoom() {
        TestContext context = new TestContext();
        when(context.userService.resolvePublicIdBySessionToken("owner-token")).thenReturn(Optional.of("u_owner"));
        when(context.accountService.isAdminSession("owner-token")).thenReturn(false);
        when(context.roomLifecycleService.deleteRoom("room-1", "u_owner", false)).thenReturn(true);

        var response = context.controller.deleteRoom("room-1", "owner-token");

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
        verify(context.roomLifecycleService).deleteRoom("room-1", "u_owner", false);
    }

    @Test
    void ordinaryUserCannotDeleteAnotherUsersRoom() {
        TestContext context = new TestContext();
        when(context.userService.resolvePublicIdBySessionToken("user-token")).thenReturn(Optional.of("u_user"));
        when(context.accountService.isAdminSession("user-token")).thenReturn(false);
        when(context.roomLifecycleService.deleteRoom("room-1", "u_user", false)).thenReturn(false);

        var response = context.controller.deleteRoom("room-1", "user-token");

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.FORBIDDEN);
    }

    @Test
    void missingOrUnknownSessionCannotDeleteRoom() {
        TestContext context = new TestContext();
        when(context.userService.resolvePublicIdBySessionToken("bad-token")).thenReturn(Optional.empty());

        assertThat(context.controller.deleteRoom("room-1", null).getStatusCode()).isEqualTo(HttpStatus.UNAUTHORIZED);
        assertThat(context.controller.deleteRoom("room-1", "bad-token").getStatusCode()).isEqualTo(HttpStatus.UNAUTHORIZED);
    }

    private static final class TestContext {
        private final RoomService roomService = mock(RoomService.class);
        private final RoomAccessService roomAccessService = mock(RoomAccessService.class);
        private final UserService userService = mock(UserService.class);
        private final AccountService accountService = mock(AccountService.class);
        private final RoomLifecycleService roomLifecycleService = mock(RoomLifecycleService.class);
        private final RoomController controller = new RoomController(
                roomService,
                roomAccessService,
                userService,
                accountService,
                roomLifecycleService
        );
    }
}
