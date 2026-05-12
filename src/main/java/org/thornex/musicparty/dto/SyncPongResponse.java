package org.thornex.musicparty.dto;

public record SyncPongResponse(
        String pingId,
        long clientSendTime,
        long serverReceiveTime,
        long serverSendTime
) {}
