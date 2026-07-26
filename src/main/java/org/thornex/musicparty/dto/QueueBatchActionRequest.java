package org.thornex.musicparty.dto;

import java.util.List;

public record QueueBatchActionRequest(List<String> queueIds, String mutationId) {
    public QueueBatchActionRequest(List<String> queueIds) {
        this(queueIds, null);
    }
}
