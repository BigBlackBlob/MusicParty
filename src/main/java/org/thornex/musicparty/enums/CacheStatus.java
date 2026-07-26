package org.thornex.musicparty.enums;

public enum CacheStatus {
    PENDING,
    RESOLVING,
    DOWNLOADING,
    TRANSCODING,
    COMPLETED,
    FAILED,
    REJECTED,
    CANCELLED,
    RETRY_WAIT
}
