package org.thornex.musicparty.dto;

import java.util.List;

public final class UserPlaylistRequests {
    private UserPlaylistRequests() {
    }

    public record CreatePlaylistRequest(String name) {
    }

    public record UpdatePlaylistRequest(String name) {
    }

    public record AddUserPlaylistTracksRequest(List<Music> musics) {
    }

    public record ReorderTracksRequest(List<String> trackIds) {
    }

    public record ImportPlaylistRequest(String platform, String playlistId) {
    }

    public record ImportNeteasePlaylistRequest(String playlistId) {
    }
}
