package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.UserPlaylist;
import org.thornex.musicparty.dto.UserPlaylistTrack;

import java.util.List;
import java.util.Optional;

public interface UserPlaylistRepository {
    List<UserPlaylist> listPlaylists(String ownerPublicId);
    Optional<UserPlaylist> findPlaylist(String ownerPublicId, String playlistId);
    UserPlaylist createPlaylist(String ownerPublicId, String name);
    Optional<UserPlaylist> renamePlaylist(String ownerPublicId, String playlistId, String name);
    boolean deletePlaylist(String ownerPublicId, String playlistId);
    List<UserPlaylistTrack> listTracks(String ownerPublicId, String playlistId, int offset, int limit);
    Optional<UserPlaylistTrack> addTrackIfAbsent(String ownerPublicId, String playlistId, Music music);
    boolean deleteTrack(String ownerPublicId, String playlistId, String trackId);
    void reorderTracks(String ownerPublicId, String playlistId, List<String> orderedTrackIds);
}
