package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.ChatMessage;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.RoomInfo;
import org.thornex.musicparty.dto.User;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.enums.QueueItemStatus;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.RoomRepository;

import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ColdRoomLifecycleTests {

    @Test
    void evictedRoomRehydratesQueueHistoryPlaybackAndChatState() throws Exception {
        AppProperties properties = new AppProperties();
        properties.getQueue().setHistorySize(50);
        properties.getPlayer().setRoomEvictionIdleMs(0L);
        RoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                roomRepository,
                new InMemoryMigrationStateRepository()
        );
        roomService.init();

        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        InMemoryChatRepository chatRepository = new InMemoryChatRepository();
        InMemoryPlaybackStateRepository playbackStateRepository = new InMemoryPlaybackStateRepository();
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                queueRepository,
                chatRepository,
                playbackStateRepository
        );
        RoomStateMutationService mutationService = new RoomStateMutationService(new TestTransactionManager());
        PlaybackTransitionService playbackTransitionService = new PlaybackTransitionService(
                persistenceService,
                mutationService,
                event -> {},
                new AfterCommitExecutor()
        );
        InMemoryUserProfileRepository userProfiles = new InMemoryUserProfileRepository();
        ChatService[] chatServiceHolder = new ChatService[1];
        RoomSessionCoordinator roomSessionCoordinator = new RoomSessionCoordinator(roomService, event -> {
            if (event instanceof org.thornex.musicparty.event.RoomSessionEvictedEvent evictedEvent) {
                chatServiceHolder[0].onRoomSessionEvicted(evictedEvent);
            }
        });
        UserService userService = new UserService(event -> {}, roomService, roomSessionCoordinator, userProfiles);
        ChatService chatService = new ChatService(
                null,
                userService,
                properties,
                persistenceService,
                mutationService,
                roomSessionCoordinator,
                new AfterCommitExecutor(),
                List.of()
        );
        chatServiceHolder[0] = chatService;
        MusicPlayerService musicPlayerService = new MusicPlayerService(
                List.of(),
                userService,
                null,
                null,
                event -> {},
                properties,
                null,
                roomService,
                roomSessionCoordinator,
                persistenceService,
                mutationService,
                playbackTransitionService
        );

        User owner = userService.handleConnect("session-1", null, "Alice");
        RoomInfo room = roomService.createRoom("Focus", owner.getPublicId(), false, null);
        String roomId = room.roomId();

        MusicPlayerService.RoomPlayerSession session = musicPlayerService.getSession(roomId);

        MusicQueueItem queueItem = new MusicQueueItem(
                "queue-1",
                new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a"),
                new UserSummary(owner.getPublicId(), owner.getName(), false),
                QueueItemStatus.READY
        );
        Music historyMusic = new Music("music-2", "Song B", List.of("Artist B"), 200_000L, "netease", "cover-b");
        ChatMessage chatMessage = new ChatMessage("chat-1", owner.getPublicId(), owner.getName(), "hello", 1L, MessageType.CHAT);

        session.getQueueManager().restore(List.of(queueItem), List.of(historyMusic));
        chatService.addMessage(roomId, chatMessage);
        Object playbackState = getFieldValue(session, "playbackState");
        invokeMethod(playbackState, "setCurrentTrack", new Class[]{PlayableMusic.class, String.class, String.class},
                new PlayableMusic("now-1", "Now Playing", List.of("Artist Now"), 210_000L, "netease", "url", "cover-now", false),
                owner.getPublicId(),
                owner.getName());
        invokeMethod(playbackState, "setPaused", new Class[]{boolean.class}, true);
        invokeMethod(playbackState, "setLoading", new Class[]{boolean.class}, false);
        invokeMethod(playbackState, "updatePlaybackAnchor", new Class[]{long.class}, 12_345L);
        invokeMethod(playbackState, "setPlayEpoch", new Class[]{long.class}, 7L);
        invokeMethod(playbackState, "setStateVersion", new Class[]{long.class}, 9L);
        invokeMethod(playbackState, "setLastHotActivityAt", new Class[]{long.class}, 0L);

        musicPlayerService.evictColdRooms();

        MusicPlayerService.RoomPlayerSession rehydrated = musicPlayerService.getSession(roomId);
        var state = rehydrated.getCurrentPlayerState();

        assertThat(rehydrated.getQueueManager().getQueueSnapshot()).containsExactly(queueItem);
        assertThat(rehydrated.getQueueManager().getHistorySnapshot()).containsExactly(historyMusic);
        assertThat(state.nowPlaying()).isNotNull();
        assertThat(state.nowPlaying().music().id()).isEqualTo("now-1");
        assertThat(state.nowPlaying().currentPosition()).isEqualTo(12_345L);
        assertThat(state.nowPlaying().enqueuedById()).isEqualTo(owner.getPublicId());
        assertThat(state.nowPlaying().enqueuedByName()).isEqualTo(owner.getName());
        assertThat(state.queue()).containsExactly(queueItem);
        assertThat(state.isPaused()).isTrue();
        assertThat(state.playEpoch()).isEqualTo(7L);
        assertThat(state.stateVersion()).isEqualTo(9L);
        assertThat(chatService.getHistoryFull(roomId)).containsExactly(chatMessage);
    }

    private Object getFieldValue(Object target, String fieldName) throws Exception {
        Field field = target.getClass().getDeclaredField(fieldName);
        field.setAccessible(true);
        return field.get(target);
    }

    private void invokeMethod(Object target, String methodName, Class<?>[] parameterTypes, Object... args) throws Exception {
        Method method = target.getClass().getDeclaredMethod(methodName, parameterTypes);
        method.setAccessible(true);
        method.invoke(target, args);
    }
}
