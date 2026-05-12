package org.thornex.musicparty.dto;

public record SyncPingRequest(
        String pingId,
        long clientSendTime
) {}
