package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.event.RoomDeletedEvent;
import org.thornex.musicparty.event.RoomSessionEvictedEvent;
import org.thornex.musicparty.event.UserCountChangeEvent;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.PersistedRoom;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicBoolean;

import static org.assertj.core.api.Assertions.assertThat;

class RoomSessionCoordinatorTests {

    @Test
    void coordinatesUserEntryEvictionAndDeleteCleanup() {
        List<Object> events = new ArrayList<>();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                events::add,
                new AppProperties(),
                new InMemoryRoomRepository(),
                new InMemoryMigrationStateRepository()
        );
        roomService.init();

        RoomSessionCoordinator coordinator = new RoomSessionCoordinator(roomService, events::add);
        var room = roomService.createRoom("Focus", "u_owner", false, null);

        coordinator.onUserEnteredRoom(room.roomId(), 2);

        assertThat(events).anySatisfy(event -> {
            assertThat(event).isInstanceOf(UserCountChangeEvent.class);
            assertThat(((UserCountChangeEvent) event).getRoomId()).isEqualTo(room.roomId());
            assertThat(((UserCountChangeEvent) event).getOnlineUserCount()).isEqualTo(2);
        });

        AtomicBoolean flushed = new AtomicBoolean(false);
        coordinator.evictColdRoom(room.roomId(), () -> flushed.set(true));

        assertThat(flushed).isTrue();
        assertThat(events).anySatisfy(event -> {
            assertThat(event).isInstanceOf(RoomSessionEvictedEvent.class);
            assertThat(((RoomSessionEvictedEvent) event).getRoomId()).isEqualTo(room.roomId());
        });

        AtomicBoolean movedUsers = new AtomicBoolean(false);
        AtomicBoolean removedPlayer = new AtomicBoolean(false);
        AtomicBoolean evictedChat = new AtomicBoolean(false);
        coordinator.cleanupDeletedRoom(
                room.roomId(),
                () -> movedUsers.set(true),
                () -> removedPlayer.set(true),
                () -> evictedChat.set(true)
        );

        assertThat(movedUsers).isTrue();
        assertThat(removedPlayer).isTrue();
        assertThat(evictedChat).isTrue();
        assertThat(events).anySatisfy(event -> {
            assertThat(event).isInstanceOf(RoomDeletedEvent.class);
            assertThat(((RoomDeletedEvent) event).getRoomId()).isEqualTo(room.roomId());
        });
    }

    @Test
    void roomActiveTouchIsAppliedByChatAndPlaybackStateMutations() {
        InMemoryRoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                new AppProperties(),
                roomRepository,
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        var room = roomService.createRoom("Focus", "u_owner", false, null);
        long initialActiveAt = findRoom(roomRepository, room.roomId()).lastActiveAt();

        RoomSessionCoordinator coordinator = new RoomSessionCoordinator(roomService, event -> {});
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                new InMemoryQueueRepository(),
                new InMemoryChatRepository(),
                new InMemoryPlaybackStateRepository()
        );
        UserService userService = new UserService(
                event -> {},
                roomService,
                coordinator,
                new InMemoryUserProfileRepository()
        );
        ChatService chatService = new ChatService(
                null,
                userService,
                new AppProperties(),
                persistenceService,
                new RoomStateMutationService(new TestTransactionManager()),
                coordinator,
                new AfterCommitExecutor(),
                List.of()
        );
        User owner = userService.handleConnect("session-1", null, "Alice", room.roomId());

        chatService.addMessage(room.roomId(), new ChatMessage("msg-1", "u-1", "Alice", "hello", 1L, MessageType.CHAT));
        long afterChatActiveAt = findRoom(roomRepository, room.roomId()).lastActiveAt();

        MusicPlayerService musicPlayerService = new MusicPlayerService(
                List.of(),
                userService,
                null,
                null,
                event -> {},
                new AppProperties(),
                null,
                roomService,
                coordinator,
                persistenceService,
                new RoomStateMutationService(new TestTransactionManager()),
                new PlaybackTransitionService(
                        persistenceService,
                        new RoomStateMutationService(new TestTransactionManager()),
                        event -> {},
                        new AfterCommitExecutor()
                )
        );
        musicPlayerService.getSession(room.roomId()).setLock("PAUSE", true);
        long afterPlaybackActiveAt = findRoom(roomRepository, room.roomId()).lastActiveAt();

        musicPlayerService.clearQueue();
        long afterQueueActiveAt = findRoom(roomRepository, room.roomId()).lastActiveAt();
        assertThat(afterChatActiveAt).isGreaterThanOrEqualTo(initialActiveAt);
        assertThat(afterPlaybackActiveAt).isGreaterThanOrEqualTo(afterChatActiveAt);
        assertThat(afterQueueActiveAt).isGreaterThanOrEqualTo(afterPlaybackActiveAt);
    }

    private PersistedRoom findRoom(InMemoryRoomRepository roomRepository, String roomId) {
        return roomRepository.findById(roomId).orElseThrow();
    }
}
