package org.thornex.musicparty.dto;

public record MusicPlatform(
    String id,
    String label,
    boolean supportsAlbumSearch
) {}