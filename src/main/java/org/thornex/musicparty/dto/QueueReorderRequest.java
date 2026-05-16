package org.thornex.musicparty.dto;

public record QueueReorderRequest(
        int oldIndex,
        int newIndex,
        String queueId,
        String targetQueueId,
        String position
) {}
