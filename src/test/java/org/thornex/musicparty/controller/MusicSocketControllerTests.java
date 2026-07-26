package org.thornex.musicparty.controller;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.JsonNodeFactory;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.dto.SeekRequest;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.service.AccountService;
import org.thornex.musicparty.service.ChatService;
import org.thornex.musicparty.service.MusicPlayerService;
import org.thornex.musicparty.service.MusicSocketSessionFacade;
import org.thornex.musicparty.service.RoomLifecycleService;
import org.thornex.musicparty.service.RoomCommandCoordinator;
import org.thornex.musicparty.service.RoomPlaylistService;
import org.thornex.musicparty.service.RoomService;
import org.thornex.musicparty.service.SocketRateLimiter;
import org.thornex.musicparty.service.UserService;

import java.util.Optional;

import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class MusicSocketControllerTests {

    @Test
    void seekPassesAccountAdminOverrideToPlayerService() {
        TestContext context = new TestContext();
        User admin = new User("admin-token", "u-admin", "ws-admin", "Admin");
        when(context.socketRateLimiter.allow("ws-admin", "control")).thenReturn(true);
        when(context.userService.getUser("ws-admin")).thenReturn(Optional.of(admin));
        when(context.accountService.isAdminSession("admin-token")).thenReturn(true);
        when(context.musicPlayerService.seekTo(42_000L, "ws-admin", true)).thenReturn(Optional.empty());

        context.controller.seek(new SeekRequest(42_000L), "ws-admin");

        verify(context.musicPlayerService).seekTo(42_000L, "ws-admin", true);
    }

    @Test
    void seekKeepsGuestDeniedBeforeAdminOverride() {
        TestContext context = new TestContext();
        User guest = new User("guest-token", "u-guest", "ws-guest", "Guest");
        when(context.socketRateLimiter.allow("ws-guest", "control")).thenReturn(true);
        when(context.userService.getUser("ws-guest")).thenReturn(Optional.of(guest));

        context.controller.seek(new SeekRequest(42_000L), "ws-guest");

        verify(context.musicSocketSessionFacade).sendControlDenied("ws-guest", "CONTROL_DENIED", "请先设置昵称再调整进度");
    }

    @Test
    void dispatchRoutesCurrentUserRequest() {
        TestContext context = new TestContext();

        context.controller.dispatch("user.me", JsonNodeFactory.instance.objectNode(), "ws-user");

        verify(context.musicSocketSessionFacade).sendCurrentUser("ws-user");
    }

    @Test
    void dispatchRoutesOnlineUsersRequest() {
        TestContext context = new TestContext();

        context.controller.dispatch("users.online", JsonNodeFactory.instance.objectNode(), "ws-user");

        verify(context.musicSocketSessionFacade).sendOnlineUsers("ws-user");
    }

    private static final class TestContext {
        private final MusicPlayerService musicPlayerService = mock(MusicPlayerService.class);
        private final UserService userService = mock(UserService.class);
        private final ChatService chatService = mock(ChatService.class);
        private final RoomService roomService = mock(RoomService.class);
        private final RoomLifecycleService roomLifecycleService = mock(RoomLifecycleService.class);
        private final MusicSocketSessionFacade musicSocketSessionFacade = mock(MusicSocketSessionFacade.class);
        private final SocketRateLimiter socketRateLimiter = mock(SocketRateLimiter.class);
        private final RoomPlaylistService roomPlaylistService = mock(RoomPlaylistService.class);
        private final AccountService accountService = mock(AccountService.class);
        private final RoomCommandCoordinator roomCommandCoordinator = mock(RoomCommandCoordinator.class);
        private final MusicSocketController controller = new MusicSocketController(
                musicPlayerService,
                userService,
                chatService,
                roomService,
                roomLifecycleService,
                musicSocketSessionFacade,
                socketRateLimiter,
                roomPlaylistService,
                accountService,
                new ObjectMapper(),
                roomCommandCoordinator
        );
    }
}
