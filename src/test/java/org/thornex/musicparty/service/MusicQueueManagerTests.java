package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.QueueItemStatus;

import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.Callable;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

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

        boolean changed = manager.reorderByQueueId(third.queueId(), first.queueId(), "before");

        assertThat(changed).isTrue();
        assertThat(manager.getQueueSnapshot())
                .extracting(MusicQueueItem::queueId)
                .containsExactly(third.queueId(), first.queueId(), second.queueId());
    }

    @Test
    void reorderByQueueIdMovesItemAfterTargetWhenDraggingDownward() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        MusicQueueItem first = manager.add(new Music("a", "A", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY);
        MusicQueueItem second = manager.add(new Music("b", "B", List.of("B"), 1000, "netease", ""), user, QueueItemStatus.READY);
        MusicQueueItem third = manager.add(new Music("c", "C", List.of("C"), 1000, "netease", ""), user, QueueItemStatus.READY);

        boolean changed = manager.reorderByQueueId(first.queueId(), third.queueId(), "after");

        assertThat(changed).isTrue();
        assertThat(manager.getQueueSnapshot())
                .extracting(MusicQueueItem::queueId)
                .containsExactly(second.queueId(), third.queueId(), first.queueId());
    }

    @Test
    void reorderByQueueIdReturnsFalseForInvalidOrUnchangedRequests() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        MusicQueueItem first = manager.add(new Music("a", "A", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY);
        MusicQueueItem second = manager.add(new Music("b", "B", List.of("B"), 1000, "netease", ""), user, QueueItemStatus.READY);

        assertThat(manager.reorderByQueueId(first.queueId(), "missing", "before")).isFalse();
        assertThat(manager.reorderByQueueId(first.queueId(), first.queueId(), "before")).isFalse();
        assertThat(manager.reorderByQueueId(first.queueId(), second.queueId(), "before")).isFalse();
        assertThat(manager.reorder(0, 0)).isFalse();

        assertThat(manager.getQueueSnapshot())
                .extracting(MusicQueueItem::queueId)
                .containsExactly(first.queueId(), second.queueId());
    }

    @Test
    void sequentialPollUsesPhysicalOrderWhenToppedItemWasDraggedBehindNormalItem() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        Music normal = new Music("a", "A", List.of("A"), 1000, "netease", "");
        Music toppedMusic = new Music("b", "B", List.of("B"), 1000, "netease", "");
        MusicQueueItem first = manager.add(normal, user, QueueItemStatus.READY);
        MusicQueueItem second = manager.add(toppedMusic, user, QueueItemStatus.READY);

        manager.top(second.queueId(), false);
        MusicQueueItem topped = manager.getQueueSnapshot().get(0);
        manager.reorderByQueueId(topped.queueId(), first.queueId(), "after");

        MusicQueueItem next = manager.pollNext(false, Map.of(
                MusicQueueManager.musicKey(normal), QueueItemStatus.READY,
                MusicQueueManager.musicKey(toppedMusic), QueueItemStatus.READY
        ), Set.of());

        assertThat(next.music()).isEqualTo(normal);
    }

    @Test
    void topStillMovesSongToFrontInSequentialMode() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        MusicQueueItem first = manager.add(new Music("a", "A", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY);
        MusicQueueItem second = manager.add(new Music("b", "B", List.of("B"), 1000, "netease", ""), user, QueueItemStatus.READY);

        manager.top(second.queueId(), false);

        assertThat(manager.getQueueSnapshot())
                .extracting(MusicQueueItem::queueId)
                .containsExactly("TOP-" + second.queueId(), first.queueId());
    }

    @Test
    void shufflePollStillPrioritizesGlobalTop() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        Music normal = new Music("a", "A", List.of("A"), 1000, "netease", "");
        Music toppedMusic = new Music("b", "B", List.of("B"), 1000, "netease", "");
        MusicQueueItem first = manager.add(normal, user, QueueItemStatus.READY);
        MusicQueueItem second = manager.add(toppedMusic, user, QueueItemStatus.READY);

        manager.top(second.queueId(), false);
        MusicQueueItem topped = manager.getQueueSnapshot().get(0);
        manager.reorderByQueueId(topped.queueId(), first.queueId(), "after");

        MusicQueueItem next = manager.pollNext(true, Map.of(
                MusicQueueManager.musicKey(normal), QueueItemStatus.READY,
                MusicQueueManager.musicKey(toppedMusic), QueueItemStatus.READY
        ), Set.of());

        assertThat(next.music()).isEqualTo(toppedMusic);
    }

    @Test
    void snapshotsAreImmutableAndReusedUntilTheNextMutation() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);

        List<MusicQueueItem> empty = manager.getQueueSnapshot();
        assertThat(manager.getQueueSnapshot()).isSameAs(empty);
        assertThatThrownBy(() -> empty.add(null)).isInstanceOf(UnsupportedOperationException.class);

        manager.add(new Music("a", "A", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY);
        List<MusicQueueItem> afterAdd = manager.getQueueSnapshot();
        assertThat(afterAdd).isNotSameAs(empty);
        assertThat(manager.getQueueSnapshot()).isSameAs(afterAdd);
    }

    @Test
    void prefixedQueueIdsRemainIndexedAfterTopAndRemoval() {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);
        MusicQueueItem item = manager.add(new Music("a", "A", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY);

        manager.top(item.queueId(), false);
        assertThat(manager.remove(item.queueId())).isPresent();
        assertThat(manager.getQueueSnapshot()).isEmpty();
    }

    @Test
    void concurrentAddsKeepSnapshotAndDuplicateIndexConsistent() throws Exception {
        MusicQueueManager manager = new MusicQueueManager(new AppProperties());
        UserSummary user = new UserSummary("public-id", "User", false);
        try (var executor = Executors.newFixedThreadPool(4)) {
            List<Callable<MusicQueueItem>> tasks = java.util.stream.IntStream.range(0, 40)
                    .<Callable<MusicQueueItem>>mapToObj(index -> () -> manager.add(
                            new Music("id-" + (index % 20), "Song", List.of("A"), 1000, "netease", ""), user, QueueItemStatus.READY))
                    .toList();
            List<Future<MusicQueueItem>> futures = executor.invokeAll(tasks);

            assertThat(futures.stream().filter(future -> {
                try {
                    return future.get() != null;
                } catch (Exception exception) {
                    throw new AssertionError(exception);
                }
            })).hasSize(20);
            assertThat(manager.getQueueSnapshot()).hasSize(20);
            assertThat(manager.getQueueSnapshot()).extracting(item -> MusicQueueManager.musicKey(item.music())).doesNotHaveDuplicates();
        }
    }
}

