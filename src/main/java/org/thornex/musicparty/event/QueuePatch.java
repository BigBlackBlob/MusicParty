package org.thornex.musicparty.event;

import org.thornex.musicparty.dto.MusicQueueItem;

import java.util.List;

public record QueuePatch(
        String operation,
        List<MusicQueueItem> items,
        List<String> queueIds,
        String queueId,
        String targetQueueId,
        String position
) {
    public static QueuePatch snapshot() {
        return new QueuePatch("snapshot", List.of(), List.of(), null, null, null);
    }

    public static QueuePatch append(List<MusicQueueItem> items) {
        return new QueuePatch("append", items, List.of(), null, null, null);
    }

    public static QueuePatch remove(List<String> queueIds) {
        return new QueuePatch("remove", List.of(), queueIds, null, null, null);
    }

    public static QueuePatch move(String queueId, String targetQueueId, String position) {
        return new QueuePatch("move", List.of(), List.of(), queueId, targetQueueId, position);
    }
}
