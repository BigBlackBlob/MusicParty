package org.thornex.musicparty.dto;

import java.util.List;

public final class RoomPlaylistRequests {
    private RoomPlaylistRequests() {}

    public record CreatePlaylistRequest(String name) {}
    public record UpdatePlaylistRequest(String name) {}
    public record AddTrackRequest(Music music) {}
    public record ReorderTracksRequest(List<String> trackIds) {}
    public record ImportPlaylistRequest(String platform, String playlistId) {}
    public record EnqueueRoomPlaylistRequest(String playlistId) {}
}
