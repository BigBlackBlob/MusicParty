package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.RoomPlaylist;
import org.thornex.musicparty.dto.RoomPlaylistTrack;

import java.util.List;
import java.util.Optional;

public interface RoomPlaylistRepository {
    List<RoomPlaylist> listPlaylists(String roomId);
    Optional<RoomPlaylist> findPlaylist(String roomId, String playlistId);
    RoomPlaylist createPlaylist(String roomId, String name);
    Optional<RoomPlaylist> renamePlaylist(String roomId, String playlistId, String name);
    boolean deletePlaylist(String roomId, String playlistId);
    List<RoomPlaylistTrack> listTracks(String roomId, String playlistId, int offset, int limit);
    Optional<RoomPlaylistTrack> addTrack(String roomId, String playlistId, Music music);
    boolean deleteTrack(String roomId, String playlistId, String trackId);
    void reorderTracks(String roomId, String playlistId, List<String> orderedTrackIds);
    void deleteRoomData(String roomId);
}
