package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.PersistedHistoryEntry;
import org.thornex.musicparty.persistence.RoomRepository;

import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class MusicPlayerServiceHistoryTests {

    @Test
    void playbackCompletionAppendsHistoryAndFlushRehydratesWithoutCollapsingEntries() throws Exception {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Music song = new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a");

        invokeMethod(session, "appendFinishedTrackToHistory", new Class[]{Music.class}, song);
        invokeMethod(session, "appendFinishedTrackToHistory", new Class[]{Music.class}, song);
        session.flushPersistentState();

        TestContext restarted = context.restart(queueRepository);

        assertThat(queueRepository.appendHistoryCalls).isEqualTo(2);
        assertThat(queueRepository.replaceHistoryCalls).isEqualTo(1);
        assertThat(session.getQueueManager().getHistorySnapshot()).containsExactly(song, song);
        assertThat(restarted.musicPlayerService().getSession(restarted.roomId()).getQueueManager().getHistorySnapshot())
                .containsExactly(song, song);
    }

    @Test
    void resetSystemClearsPersistedHistoryWithoutLeavingResidualEntriesAfterRehydrate() {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Music first = new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a");
        Music second = new Music("music-2", "Song B", List.of("Artist B"), 200_000L, "netease", "cover-b");

        session.getQueueManager().restore(List.of(), List.of(first, second));
        session.flushPersistentState();
        session.resetSystem(false, true);

        TestContext restarted = context.restart(queueRepository);

        assertThat(queueRepository.appendHistoryCalls).isZero();
        assertThat(queueRepository.replaceHistoryCalls).isEqualTo(2);
        assertThat(queueRepository.replacedHistorySnapshots.getLast()).isEmpty();
        assertThat(restarted.musicPlayerService().getSession(restarted.roomId()).getQueueManager().getHistorySnapshot()).isEmpty();
    }

    @Test
    void flushUsesReplaceOnlyForCurrentHistorySnapshot() {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Music first = new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a");
        Music second = new Music("music-2", "Song B", List.of("Artist B"), 200_000L, "netease", "cover-b");

        session.getQueueManager().restore(List.of(), List.of(first, second));
        session.flushPersistentState();

        assertThat(queueRepository.appendHistoryCalls).isZero();
        assertThat(queueRepository.replaceHistoryCalls).isEqualTo(1);
        assertThat(queueRepository.replacedHistorySnapshots).containsExactly(List.of(first, second));
    }

    @Test
    void playbackControlCooldownIsTrackedPerSession() throws Exception {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());

        boolean firstUserFirstControl = (boolean) invokeMethod(session, "isRateLimited", new Class[]{String.class}, "session-a");
        boolean firstUserSecondControl = (boolean) invokeMethod(session, "isRateLimited", new Class[]{String.class}, "session-a");
        boolean secondUserFirstControl = (boolean) invokeMethod(session, "isRateLimited", new Class[]{String.class}, "session-b");

        assertThat(firstUserFirstControl).isFalse();
        assertThat(firstUserSecondControl).isTrue();
        assertThat(secondUserFirstControl).isFalse();
    }

    private TestContext createContext(RecordingQueueRepository queueRepository) {
        AppProperties properties = new AppProperties();
        properties.getQueue().setHistorySize(50);
        RoomRepository roomRepository = new InMemoryRoomRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                event -> {},
                properties,
                roomRepository,
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        String roomId = roomService.createRoom("Focus", "owner-1", false, null).roomId();
        RoomSessionCoordinator roomSessionCoordinator = new RoomSessionCoordinator(roomService, event -> {});
        UserService userService = new UserService(event -> {}, roomService, roomSessionCoordinator, new InMemoryUserProfileRepository());
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                queueRepository,
                new InMemoryChatRepository(),
                new InMemoryPlaybackStateRepository()
        );
        RoomStateMutationService mutationService = new RoomStateMutationService(new TestTransactionManager());
        PlaybackTransitionService playbackTransitionService = new PlaybackTransitionService(
                persistenceService,
                mutationService,
                event -> {}
        );
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
                playbackTransitionService,
                null,
                null
        );
        return new TestContext(properties, roomRepository, roomId, musicPlayerService);
    }

    private Object invokeMethod(Object target, String methodName, Class<?>[] parameterTypes, Object... args) throws Exception {
        Method method = target.getClass().getDeclaredMethod(methodName, parameterTypes);
        method.setAccessible(true);
        return method.invoke(target, args);
    }

    private record TestContext(
            AppProperties properties,
            RoomRepository roomRepository,
            String roomId,
            MusicPlayerService musicPlayerService
    ) {
        private TestContext restart(RecordingQueueRepository queueRepository) {
            RoomService roomService = new RoomService(
                    new ObjectMapper(),
                    event -> {},
                    properties,
                    roomRepository,
                    new InMemoryMigrationStateRepository()
            );
            roomService.init();
            RoomSessionCoordinator roomSessionCoordinator = new RoomSessionCoordinator(roomService, event -> {});
            UserService userService = new UserService(event -> {}, roomService, roomSessionCoordinator, new InMemoryUserProfileRepository());
            RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                    queueRepository,
                    new InMemoryChatRepository(),
                    new InMemoryPlaybackStateRepository()
            );
            RoomStateMutationService mutationService = new RoomStateMutationService(new TestTransactionManager());
            PlaybackTransitionService playbackTransitionService = new PlaybackTransitionService(
                    persistenceService,
                    mutationService,
                    event -> {}
            );
            return new TestContext(
                    properties,
                    roomRepository,
                    roomId,
                    new MusicPlayerService(
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
                            playbackTransitionService,
                            null,
                            null
                    )
            );
        }
    }

    private static final class RecordingQueueRepository extends org.thornex.musicparty.persistence.InMemoryQueueRepository {
        private int appendHistoryCalls;
        private int replaceHistoryCalls;
        private final List<List<Music>> replacedHistorySnapshots = new ArrayList<>();

        @Override
        public void appendHistory(PersistedHistoryEntry historyEntry) {
            appendHistoryCalls++;
            super.appendHistory(historyEntry);
        }

        @Override
        public void replaceHistory(String roomId, List<Music> historyItems) {
            replaceHistoryCalls++;
            replacedHistorySnapshots.add(new ArrayList<>(historyItems));
            super.replaceHistory(roomId, historyItems);
        }
    }
}
