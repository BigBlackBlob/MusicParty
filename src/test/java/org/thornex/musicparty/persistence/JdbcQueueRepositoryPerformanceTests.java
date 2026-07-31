package org.thornex.musicparty.persistence;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.sqlite.SQLiteDataSource;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.QueueItemStatus;

import java.nio.file.Path;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class JdbcQueueRepositoryPerformanceTests {

    @TempDir
    Path temporaryDirectory;

    @Test
    void moveUsesSparseKeyAndLeavesUnmovedRowsAtTheirExistingKeys() {
        SQLiteDataSource dataSource = new SQLiteDataSource();
        dataSource.setUrl("jdbc:sqlite:" + temporaryDirectory.resolve("queue.db"));
        new ResourceDatabasePopulator(new org.springframework.core.io.ClassPathResource("db/schema.sql"))
                .execute(dataSource);
        JdbcTemplate jdbc = new JdbcTemplate(dataSource);
        JdbcQueueRepository repository = new JdbcQueueRepository(jdbc, new ObjectMapper());

        MusicQueueItem first = item("first");
        MusicQueueItem second = item("second");
        MusicQueueItem third = item("third");
        repository.synchronizeQueue("lounge", List.of(first, second, third));

        repository.synchronizeQueue("lounge", List.of(first, third, second));

        assertThat(jdbc.queryForObject("select sort_order from room_queue where id = 'first'", Long.class)).isEqualTo(1024L);
        assertThat(jdbc.queryForObject("select sort_order from room_queue where id = 'third'", Long.class)).isEqualTo(1536L);
        assertThat(jdbc.queryForObject("select sort_order from room_queue where id = 'second'", Long.class)).isEqualTo(2048L);
        assertThat(repository.loadQueue("lounge")).extracting(MusicQueueItem::queueId)
                .containsExactly("first", "third", "second");
    }

    private MusicQueueItem item(String id) {
        return new MusicQueueItem(id, new Music(id, id, List.of("artist"), 1000, "test", null),
                new UserSummary("user", "User", false), QueueItemStatus.READY);
    }
}
