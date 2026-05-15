package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.event.RoomDeletedEvent;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.RoomRepository;

import java.util.ArrayList;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class RoomLifecycleServiceTests {

    @Test
    void deleteRoomMovesPersistedAndOnlineUsersBackToDefaultAndPublishesEvent() {
        AppProperties properties = new AppProperties();
        RoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                roomRepository,
                new InMemoryMigrationStateRepository()
        );
        roomService.init();

        InMemoryUserProfileRepository userProfiles = new InMemoryUserProfileRepository();
        UserService userService = new UserService(event -> {}, roomService, userProfiles);
        User owner = userService.handleConnect("session-1", null, "Alice");
        String roomId = roomService.createRoom("Focus", owner.getPublicId(), false, null).roomId();

        userService.handleConnect("session-2", owner.getSessionToken(), "Ignored", roomId);
        userService.disconnectUser("session-2");
        User onlineInRoom = userService.handleConnect("session-3", null, "Bob", roomId);

        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                new InMemoryQueueRepository(),
                new InMemoryChatRepository(),
                new InMemoryPlaybackStateRepository()
        );
        TestMusicPlayerService musicPlayerService = new TestMusicPlayerService(roomId);
        TestChatService chatService = new TestChatService();
        List<Object> publishedEvents = new ArrayList<>();
        RoomLifecycleService lifecycleService = new RoomLifecycleService(
                roomService,
                userService,
                persistenceService,
                musicPlayerService,
                chatService,
                publishedEvents::add
        );

        boolean deleted = lifecycleService.deleteRoom(roomId, owner.getPublicId(), false);

        assertThat(deleted).isTrue();
        assertThat(roomService.listRooms()).extracting(room -> room.roomId()).doesNotContain(roomId);
        assertThat(userProfiles.findByPublicId(owner.getPublicId())).isPresent()
                .get()
                .extracting(profile -> profile.currentRoomId())
                .isEqualTo(RoomService.DEFAULT_ROOM_ID);
        assertThat(userProfiles.findByPublicId(onlineInRoom.getPublicId())).isPresent()
                .get()
                .extracting(profile -> profile.currentRoomId())
                .isEqualTo(RoomService.DEFAULT_ROOM_ID);
        assertThat(userService.getUser("session-3")).isPresent()
                .get()
                .extracting(User::getRoomId)
                .isEqualTo(RoomService.DEFAULT_ROOM_ID);
        assertThat(musicPlayerService.removedRoomId).isEqualTo(roomId);
        assertThat(musicPlayerService.skipPersistenceCleanup).isTrue();
        assertThat(chatService.evictedRoomId).isEqualTo(roomId);
        assertThat(publishedEvents).anySatisfy(event -> {
            assertThat(event).isInstanceOf(RoomDeletedEvent.class);
            assertThat(((RoomDeletedEvent) event).getRoomId()).isEqualTo(roomId);
        });
    }

    private static final class TestMusicPlayerService extends MusicPlayerService {
        private String removedRoomId;
        private boolean skipPersistenceCleanup;

        private TestMusicPlayerService(String roomId) {
            super(
                    List.of(),
                    null,
                    null,
                    null,
                    event -> {},
                    new AppProperties(),
                    null,
                    null,
                    new RoomStatePersistenceService(
                            new InMemoryQueueRepository(),
                            new InMemoryChatRepository(),
                            new InMemoryPlaybackStateRepository()
                    )
            );
            this.removedRoomId = null;
            this.skipPersistenceCleanup = false;
        }

        @Override
        public void removeRoom(String roomId, boolean skipPersistenceCleanup) {
            this.removedRoomId = roomId;
            this.skipPersistenceCleanup = skipPersistenceCleanup;
        }
    }

    private static final class TestChatService extends ChatService {
        private String evictedRoomId;

        private TestChatService() {
            super(
                    null,
                    null,
                    new AppProperties(),
                    new RoomStatePersistenceService(
                            new InMemoryQueueRepository(),
                            new InMemoryChatRepository(),
                            new InMemoryPlaybackStateRepository()
                    ),
                    List.of()
            );
        }

        @Override
        public void evictRoomHistory(String roomId) {
            this.evictedRoomId = roomId;
        }
    }
}
