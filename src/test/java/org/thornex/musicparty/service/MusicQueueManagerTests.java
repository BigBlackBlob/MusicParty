package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.QueueItemStatus;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class MusicQueueManagerTests {

    @Test
    void duplicateCheckIncludesPlatform() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        assertThat(manager.add(new Music("same-id", "Netease Song", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY))
                .isNotNull();
        assertThat(manager.add(new Music("same-id", "Bilibili Song", List.of("B"), 1000, "bilibili", ""), user, QueueItemStatus.READY))
                .isNotNull();
        assertThat(manager.add(new Music("same-id", "Duplicate Netease Song", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY))
                .isNull();
    }

    @Test
    void reorderByQueueIdMovesItemRelativeToCurrentSnapshot() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        MusicQueueItem first = manager.add(new Music("a", "A", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY);
        MusicQueueItem second = manager.add(new Music("b", "B", List.of("B"), 1000, "netease", ""), user, QueueItemStatus.READY);
        MusicQueueItem third = manager.add(new Music("c", "C", List.of("C"), 1000, "netease", ""), user, QueueItemStatus.READY);

        manager.reorderByQueueId(third.queueId(), first.queueId(), "before");

        assertThat(manager.getQueueSnapshot())
                .extracting(MusicQueueItem::queueId)
                .containsExactly(third.queueId(), first.queueId(), second.queueId());
    }
}

