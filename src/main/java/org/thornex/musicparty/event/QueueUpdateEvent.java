package org.thornex.musicparty.event;

import lombok.Getter;
import org.springframework.context.ApplicationEvent;
import org.thornex.musicparty.dto.MusicQueueItem;

import java.util.List;

/**
 * 当队列内容发生变化（增删改、排序、状态变更）时触发
 */
@Getter
public class QueueUpdateEvent extends ApplicationEvent {
    private final String roomId;
    private final List<MusicQueueItem> queue;
    private final long queueVersion;
    private final QueuePatch patch;

    public QueueUpdateEvent(Object source, List<MusicQueueItem> queue) {
        this(source, "lounge", queue, 0, QueuePatch.snapshot());
    }

    public QueueUpdateEvent(Object source, String roomId, List<MusicQueueItem> queue) {
        this(source, roomId, queue, 0, QueuePatch.snapshot());
    }

    public QueueUpdateEvent(Object source, String roomId, List<MusicQueueItem> queue, long queueVersion) {
        this(source, roomId, queue, queueVersion, QueuePatch.snapshot());
    }

    public QueueUpdateEvent(Object source, String roomId, List<MusicQueueItem> queue, long queueVersion, QueuePatch patch) {
        super(source);
        this.roomId = roomId;
        this.queue = queue;
        this.queueVersion = queueVersion;
        this.patch = patch == null ? QueuePatch.snapshot() : patch;
    }
}
