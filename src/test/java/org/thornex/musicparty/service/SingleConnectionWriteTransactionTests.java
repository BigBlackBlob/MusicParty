package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.core.io.ClassPathResource;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.thornex.musicparty.config.SqliteSchemaInitializer;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.QueueItemStatus;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryRoomPlaylistRepository;
import org.thornex.musicparty.persistence.JdbcPlaybackStateRepository;
import org.thornex.musicparty.persistence.JdbcQueueRepository;

import java.nio.file.Path;
import java.time.Duration;
import java.util.List;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;

import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertTimeoutPreemptively;

class SingleConnectionWriteTransactionTests {

    @TempDir
    Path tempDir;

    @Test
    void nestedPersistenceUsesWriterOwnedTransactionWithOneConnection() {
        HikariConfig hikariConfig = new HikariConfig();
        hikariConfig.setJdbcUrl("jdbc:sqlite:" + tempDir.resolve("single-writer.db").toAbsolutePath());
        hikariConfig.setMaximumPoolSize(1);
        hikariConfig.setMinimumIdle(1);
        hikariConfig.setConnectionTimeout(1_000);

        try (HikariDataSource dataSource = new HikariDataSource(hikariConfig)) {
            new SqliteSchemaInitializer(
                    dataSource,
                    new ResourceDatabasePopulator(new ClassPathResource("db/schema.sql"))
            ).initialize();

            JdbcTemplate jdbcTemplate = new JdbcTemplate(dataSource);
            JdbcQueueRepository queueRepository = new JdbcQueueRepository(jdbcTemplate, new ObjectMapper());
            JdbcPlaybackStateRepository playbackRepository = new JdbcPlaybackStateRepository(jdbcTemplate, new ObjectMapper());
            ThreadPoolExecutor writer = new ThreadPoolExecutor(
                    1, 1, 0, TimeUnit.MILLISECONDS,
                    new ArrayBlockingQueue<>(8),
                    runnable -> new Thread(runnable, "mp-db-write-test"),
                    new ThreadPoolExecutor.AbortPolicy()
            );

            try {
                DatabaseWriteExecutor writeExecutor = new DatabaseWriteExecutor(writer);
                RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                        queueRepository,
                        new InMemoryChatRepository(),
                        playbackRepository,
                        new InMemoryRoomPlaylistRepository(),
                        writeExecutor,
                        new RoomStateWriteTransactionService(queueRepository, playbackRepository)
                );
                RoomStateMutationService mutationService = new RoomStateMutationService(
                        new DataSourceTransactionManager(dataSource),
                        writeExecutor
                );
                MusicQueueItem item = new MusicQueueItem(
                        "queue-1",
                        new Music("music-1", "Song", List.of("Artist"), 120_000L, "netease", "cover"),
                        new UserSummary("user-1", "Alice", false),
                        QueueItemStatus.READY
                );

                assertTimeoutPreemptively(Duration.ofSeconds(2), () ->
                        mutationService.runInTransaction(() ->
                                persistenceService.persistQueueSnapshot(RoomService.DEFAULT_ROOM_ID, List.of(item))));

                assertThat(queueRepository.loadQueue(RoomService.DEFAULT_ROOM_ID))
                        .extracting(MusicQueueItem::queueId)
                        .containsExactly("queue-1");
            } finally {
                writer.shutdownNow();
            }
        }
    }
}
