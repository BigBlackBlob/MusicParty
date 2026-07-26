package org.thornex.musicparty.dto;

public record QueueActionRequest(String queueId, String mutationId) {
    public QueueActionRequest(String queueId) {
        this(queueId, null);
    }
}
