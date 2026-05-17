package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.support.AbstractPlatformTransactionManager;
import org.springframework.transaction.support.DefaultTransactionStatus;
import org.sqlite.SQLiteDataSource;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.PlayerState;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.config.SqliteSchemaInitializer;
import org.thornex.musicparty.enums.MessageType;
import org.thornex.musicparty.enums.PlayerAction;
import org.thornex.musicparty.enums.QueueItemStatus;
import org.thornex.musicparty.event.PlayerStateEvent;
import org.thornex.musicparty.event.QueueUpdateEvent;
import org.thornex.musicparty.event.SystemMessageEvent;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.JdbcMigrationStateRepository;
import org.thornex.musicparty.persistence.JdbcPlaybackStateRepository;
import org.thornex.musicparty.persistence.JdbcQueueRepository;
import org.thornex.musicparty.persistence.JdbcRoomRepository;
import org.thornex.musicparty.persistence.PersistedPlaybackState;
import org.thornex.musicparty.persistence.PlaybackStateRepository;

import java.util.ArrayList;
import java.util.List;
import java.nio.file.Files;
import java.nio.file.Path;

import static org.assertj.core.api.Assertions.assertThat;

class PlaybackTransitionServiceTests {

    @Test
    void applyPersistsQueueAndPlaybackAndPublishesEventsInOrder() {
        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        InMemoryPlaybackStateRepository playbackStateRepository = new InMemoryPlaybackStateRepository();
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                queueRepository,
                new org.thornex.musicparty.persistence.InMemoryChatRepository(),
                playbackStateRepository
        );
        List<Object> publishedEvents = new ArrayList<>();
        PlaybackTransitionService transitionService = new PlaybackTransitionService(
                persistenceService,
                new RoomStateMutationService(new TestTransactionManager()),
                publishedEvents::add
        );

        MusicQueueItem item = new MusicQueueItem(
                "queue-1",
                new Music("music-1", "Song", List.of("Artist"), 120_000L, "netease", "cover"),
                new UserSummary("user-1", "Alice", false),
                QueueItemStatus.READY
        );
        PersistedPlaybackState playbackState = new PersistedPlaybackState(
                "room-1",
                null,
                null,
                null,
                0L,
                0L,
                0L,
                false,
                false,
                false,
                false,
                false,
                true,
                java.util.Set.of("user-1"),
                java.util.List.of(123L),
                2L,
                3L,
                4L
        );
        QueueUpdateEvent queueEvent = new QueueUpdateEvent(this, "room-1", List.of(item));
        PlayerStateEvent playerStateEvent = new PlayerStateEvent(this, "room-1", new PlayerState(
                null,
                List.of(item),
                false,
                List.of(),
                false,
                false,
                false,
                false,
                true,
                0,
                10L,
                3L,
                2L
        ));
        SystemMessageEvent systemMessageEvent = new SystemMessageEvent(
                this,
                SystemMessageEvent.Level.INFO,
                PlayerAction.PLAY_START,
                "user-1",
                "Song",
                "room-1"
        );

        transitionService.apply(new PlaybackTransitionService.PlaybackTransition(
                "room-1",
                List.of(item),
                playbackState,
                queueEvent,
                playerStateEvent,
                systemMessageEvent
        ));

        assertThat(queueRepository.loadQueue("room-1")).containsExactly(item);
        assertThat(playbackStateRepository.findByRoomId("room-1")).contains(playbackState);
        assertThat(publishedEvents).containsExactly(queueEvent, playerStateEvent, systemMessageEvent);
    }

    @Test
    void applyRethrowsCommitFailureAfterSynchronousEventPublication() {
        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        InMemoryPlaybackStateRepository playbackStateRepository = new InMemoryPlaybackStateRepository();
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                queueRepository,
                new org.thornex.musicparty.persistence.InMemoryChatRepository(),
                playbackStateRepository
        );
        List<Object> publishedEvents = new ArrayList<>();
        PlaybackTransitionService transitionService = new PlaybackTransitionService(
                persistenceService,
                new RoomStateMutationService(new FailingCommitTransactionManager()),
                publishedEvents::add
        );

        MusicQueueItem item = new MusicQueueItem(
                "queue-1",
                new Music("music-1", "Song", List.of("Artist"), 120_000L, "netease", "cover"),
                new UserSummary("user-1", "Alice", false),
                QueueItemStatus.READY
        );
        PersistedPlaybackState playbackState = new PersistedPlaybackState(
                "room-1",
                null,
                null,
                null,
                0L,
                0L,
                0L,
                false,
                false,
                false,
                false,
                false,
                true,
                java.util.Set.of("user-1"),
                java.util.List.of(123L),
                2L,
                3L,
                4L
        );
        QueueUpdateEvent queueEvent = new QueueUpdateEvent(this, "room-1", List.of(item));
        PlayerStateEvent playerStateEvent = new PlayerStateEvent(this, "room-1", new PlayerState(
                null,
                List.of(item),
                false,
                List.of(),
                false,
                false,
                false,
                false,
                true,
                0,
                10L,
                3L,
                2L
        ));
        SystemMessageEvent systemMessageEvent = new SystemMessageEvent(
                this,
                SystemMessageEvent.Level.INFO,
                PlayerAction.PLAY_START,
                "user-1",
                "Song",
                "room-1"
        );

        try {
            transitionService.apply(new PlaybackTransitionService.PlaybackTransition(
                    "room-1",
                    List.of(item),
                    playbackState,
                    queueEvent,
                    playerStateEvent,
                    systemMessageEvent
            ));
        } catch (IllegalStateException ex) {
            assertThat(ex).hasMessage("commit failed");
        }

        assertThat(publishedEvents).containsExactly(queueEvent, playerStateEvent, systemMessageEvent);
    }

    @Test
    void applyPersistsQueueAndPlaybackOnRealSqliteAndPublishesEventsInOrder() throws Exception {
        SqlitePlaybackContext context = createSqlitePlaybackContext("playback-success.db", false);
        MusicQueueItem item = queueItem();
        PersistedPlaybackState playbackState = playbackState(RoomService.DEFAULT_ROOM_ID);
        QueueUpdateEvent queueEvent = queueEvent(item, RoomService.DEFAULT_ROOM_ID);
        PlayerStateEvent playerStateEvent = playerStateEvent(item, RoomService.DEFAULT_ROOM_ID);
        SystemMessageEvent systemMessageEvent = systemMessageEvent(RoomService.DEFAULT_ROOM_ID);

        context.transitionService.apply(new PlaybackTransitionService.PlaybackTransition(
                RoomService.DEFAULT_ROOM_ID,
                List.of(item),
                playbackState,
                queueEvent,
                playerStateEvent,
                systemMessageEvent
        ));

        assertThat(context.queueRepository.loadQueue(RoomService.DEFAULT_ROOM_ID)).containsExactly(item);
        assertThat(context.playbackStateRepository.findByRoomId(RoomService.DEFAULT_ROOM_ID)).contains(playbackState);
        assertThat(context.publishedEvents).containsExactly(queueEvent, playerStateEvent, systemMessageEvent);
    }

    @Test
    void applyRollsBackRealSqliteStateAndDoesNotPublishEventsWhenPlaybackPersistenceFails() throws Exception {
        SqlitePlaybackContext context = createSqlitePlaybackContext("playback-failure.db", true);
        MusicQueueItem item = queueItem();
        PersistedPlaybackState playbackState = playbackState(RoomService.DEFAULT_ROOM_ID);
        QueueUpdateEvent queueEvent = queueEvent(item, RoomService.DEFAULT_ROOM_ID);
        PlayerStateEvent playerStateEvent = playerStateEvent(item, RoomService.DEFAULT_ROOM_ID);
        SystemMessageEvent systemMessageEvent = systemMessageEvent(RoomService.DEFAULT_ROOM_ID);

        try {
            context.transitionService.apply(new PlaybackTransitionService.PlaybackTransition(
                    RoomService.DEFAULT_ROOM_ID,
                    List.of(item),
                    playbackState,
                    queueEvent,
                    playerStateEvent,
                    systemMessageEvent
            ));
        } catch (IllegalStateException ex) {
            assertThat(ex).hasMessage("playback write failed");
        }

        assertThat(context.queueRepository.loadQueue(RoomService.DEFAULT_ROOM_ID)).isEmpty();
        assertThat(context.playbackStateRepository.findByRoomId(RoomService.DEFAULT_ROOM_ID)).isEmpty();
        assertThat(context.publishedEvents).isEmpty();
    }

    private SqlitePlaybackContext createSqlitePlaybackContext(String fileName, boolean failPlaybackPersist) throws Exception {
        SQLiteDataSource dataSource = createSqliteDataSource(fileName);
        JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
        ObjectMapper objectMapper = new ObjectMapper();
        RoomService roomService = new RoomService(
                objectMapper,
                event -> {},
                new AppProperties(),
                new JdbcRoomRepository(jdbcTemplate),
                new JdbcMigrationStateRepository(jdbcTemplate)
        );
        roomService.init();
        JdbcQueueRepository queueRepository = new JdbcQueueRepository(jdbcTemplate, objectMapper);
        JdbcPlaybackStateRepository playbackStateRepository = new JdbcPlaybackStateRepository(jdbcTemplate, objectMapper);
        PlaybackStateRepository effectivePlaybackRepository = failPlaybackPersist
                ? new FailingPlaybackStateRepository(playbackStateRepository)
                : playbackStateRepository;
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                queueRepository,
                new InMemoryChatRepository(),
                effectivePlaybackRepository
        );
        List<Object> publishedEvents = new ArrayList<>();
        return new SqlitePlaybackContext(
                new PlaybackTransitionService(
                        persistenceService,
                        new RoomStateMutationService(new DataSourceTransactionManager(dataSource)),
                        publishedEvents::add
                ),
                queueRepository,
                playbackStateRepository,
                publishedEvents
        );
    }

    private SQLiteDataSource createSqliteDataSource(String fileName) throws Exception {
        Path tempDir = Path.of("target", "tmp", "playback-transition-tests");
        Files.createDirectories(tempDir);
        Path dbPath = tempDir.resolve(fileName);
        Files.deleteIfExists(dbPath);
        SQLiteDataSource dataSource = new SQLiteDataSource();
        dataSource.setUrl("jdbc:sqlite:" + dbPath.toAbsolutePath());
        ResourceDatabasePopulator populator = new ResourceDatabasePopulator(
                new org.springframework.core.io.ClassPathResource("db/schema.sql")
        );
        new SqliteSchemaInitializer(dataSource, populator).initialize();
        return dataSource;
    }

    private MusicQueueItem queueItem() {
        return new MusicQueueItem(
                "queue-1",
                new Music("music-1", "Song", List.of("Artist"), 120_000L, "netease", "cover"),
                new UserSummary("user-1", "Alice", false),
                QueueItemStatus.READY
        );
    }

    private PersistedPlaybackState playbackState(String roomId) {
        return new PersistedPlaybackState(
                roomId,
                null,
                null,
                null,
                0L,
                0L,
                0L,
                false,
                false,
                false,
                false,
                false,
                true,
                java.util.Set.of("user-1"),
                java.util.List.of(123L),
                2L,
                3L,
                4L
        );
    }

    private QueueUpdateEvent queueEvent(MusicQueueItem item, String roomId) {
        return new QueueUpdateEvent(this, roomId, List.of(item));
    }

    private PlayerStateEvent playerStateEvent(MusicQueueItem item, String roomId) {
        return new PlayerStateEvent(this, roomId, new PlayerState(
                null,
                List.of(item),
                false,
                List.of(),
                false,
                false,
                false,
                false,
                true,
                0,
                10L,
                3L,
                2L
        ));
    }

    private SystemMessageEvent systemMessageEvent(String roomId) {
        return new SystemMessageEvent(
                this,
                SystemMessageEvent.Level.INFO,
                PlayerAction.PLAY_START,
                "user-1",
                "Song",
                roomId
        );
    }

    private static final class FailingCommitTransactionManager extends AbstractPlatformTransactionManager {
        @Override
        protected Object doGetTransaction() {
            return new Object();
        }

        @Override
        protected void doBegin(Object transaction, TransactionDefinition definition) {
            // no-op
        }

        @Override
        protected void doCommit(DefaultTransactionStatus status) {
            throw new IllegalStateException("commit failed");
        }

        @Override
        protected void doRollback(DefaultTransactionStatus status) {
            // no-op
        }
    }

    private record SqlitePlaybackContext(
            PlaybackTransitionService transitionService,
            JdbcQueueRepository queueRepository,
            JdbcPlaybackStateRepository playbackStateRepository,
            List<Object> publishedEvents
    ) {
    }

    private static final class FailingPlaybackStateRepository implements PlaybackStateRepository {
        private final JdbcPlaybackStateRepository delegate;

        private FailingPlaybackStateRepository(JdbcPlaybackStateRepository delegate) {
            this.delegate = delegate;
        }

        @Override
        public java.util.Optional<PersistedPlaybackState> findByRoomId(String roomId) {
            return delegate.findByRoomId(roomId);
        }

        @Override
        public void upsert(PersistedPlaybackState state) {
            throw new IllegalStateException("playback write failed");
        }

        @Override
        public void delete(String roomId) {
            delegate.delete(roomId);
        }
    }
}
