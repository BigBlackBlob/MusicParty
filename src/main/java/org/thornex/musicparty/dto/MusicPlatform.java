package org.thornex.musicparty.dto;

public record MusicPlatform(
    String id,
    String label,
    boolean supportsAlbumSearch,
    boolean subsonic
) {
    public MusicPlatform(String id, String label, boolean supportsAlbumSearch) {
        this(id, label, supportsAlbumSearch, false);
    }
}
