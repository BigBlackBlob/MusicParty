package org.thornex.musicparty.dto;

import java.util.List;

public record QueueBatchActionRequest(List<String> queueIds) {}
