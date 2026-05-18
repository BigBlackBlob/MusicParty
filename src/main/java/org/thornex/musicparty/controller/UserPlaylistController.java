package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlaylistWriteResult;
import org.thornex.musicparty.dto.UserPlaylist;
import org.thornex.musicparty.dto.UserPlaylistRequests;
import org.thornex.musicparty.dto.UserPlaylistTrack;
import org.thornex.musicparty.service.UserPlaylistService;
import reactor.core.publisher.Mono;

import java.util.List;

@RestController
@RequestMapping("/api/me")
@RequiredArgsConstructor
public class UserPlaylistController {
    private final UserPlaylistService userPlaylistService;

    @GetMapping("/playlists")
    public List<UserPlaylist> list(@RequestParam String sessionToken) {
        return userPlaylistService.listPlaylists(sessionToken);
    }

    @PostMapping("/playlists")
    public UserPlaylist create(@RequestParam String sessionToken, @RequestBody UserPlaylistRequests.CreatePlaylistRequest request) {
        return userPlaylistService.createPlaylist(sessionToken, request.name());
    }

    @PatchMapping("/playlists/{playlistId}")
    public UserPlaylist rename(@RequestParam String sessionToken,
                               @PathVariable String playlistId,
                               @RequestBody UserPlaylistRequests.UpdatePlaylistRequest request) {
        return userPlaylistService.renamePlaylist(sessionToken, playlistId, request.name());
    }

    @DeleteMapping("/playlists/{playlistId}")
    public void delete(@RequestParam String sessionToken, @PathVariable String playlistId) {
        userPlaylistService.deletePlaylist(sessionToken, playlistId);
    }

    @GetMapping("/playlists/{playlistId}/tracks")
    public List<UserPlaylistTrack> tracks(@RequestParam String sessionToken,
                                          @PathVariable String playlistId,
                                          @RequestParam(defaultValue = "0") int offset,
                                          @RequestParam(defaultValue = "100") int limit) {
        return userPlaylistService.listTracks(sessionToken, playlistId, offset, limit);
    }

    @PostMapping("/playlists/{playlistId}/tracks/batch")
    public PlaylistWriteResult addTracks(@RequestParam String sessionToken,
                                         @PathVariable String playlistId,
                                         @RequestBody UserPlaylistRequests.AddUserPlaylistTracksRequest request) {
        return userPlaylistService.addTracks(sessionToken, playlistId, request.musics());
    }

    @DeleteMapping("/playlists/{playlistId}/tracks/{trackId}")
    public void deleteTrack(@RequestParam String sessionToken, @PathVariable String playlistId, @PathVariable String trackId) {
        userPlaylistService.deleteTrack(sessionToken, playlistId, trackId);
    }

    @PostMapping("/playlists/{playlistId}/tracks/reorder")
    public void reorder(@RequestParam String sessionToken,
                        @PathVariable String playlistId,
                        @RequestBody UserPlaylistRequests.ReorderTracksRequest request) {
        userPlaylistService.reorderTracks(sessionToken, playlistId, request.trackIds());
    }

    @PostMapping("/playlists/{playlistId}/import/netease")
    public Mono<PlaylistWriteResult> importNetease(@RequestParam String sessionToken,
                                                   @PathVariable String playlistId,
                                                   @RequestBody UserPlaylistRequests.ImportNeteasePlaylistRequest request) {
        return userPlaylistService.importNetease(sessionToken, playlistId, request.playlistId());
    }

    @PostMapping("/playlists/{playlistId}/import")
    public Mono<PlaylistWriteResult> importPlaylist(@RequestParam String sessionToken,
                                                    @PathVariable String playlistId,
                                                    @RequestBody UserPlaylistRequests.ImportPlaylistRequest request) {
        return userPlaylistService.importPlaylist(sessionToken, playlistId, request.platform(), request.playlistId());
    }

    @GetMapping(value = "/playlists/{playlistId}/export", produces = MediaType.TEXT_PLAIN_VALUE)
    public String export(@RequestParam String sessionToken,
                         @PathVariable String playlistId,
                         @RequestParam(defaultValue = "txt") String format) {
        return userPlaylistService.exportPlaylist(sessionToken, playlistId, format);
    }

    @PostMapping("/playlists/{playlistId}/enqueue")
    public void enqueue(@RequestParam String sessionToken, @PathVariable String playlistId) {
        userPlaylistService.enqueue(sessionToken, playlistId);
    }

    @GetMapping("/liked-songs")
    public List<UserPlaylistTrack> likedSongs(@RequestParam String sessionToken) {
        return userPlaylistService.listLikedSongs(sessionToken);
    }

    @PutMapping("/liked-songs/{platform}/{musicId}")
    public PlaylistWriteResult likeSong(@RequestParam String sessionToken,
                                        @PathVariable String platform,
                                        @PathVariable String musicId,
                                        @RequestBody Music music) {
        return userPlaylistService.addLikedSong(sessionToken, platform, musicId, music);
    }

    @DeleteMapping("/liked-songs/{platform}/{musicId}")
    public void unlikeSong(@RequestParam String sessionToken,
                           @PathVariable String platform,
                           @PathVariable String musicId) {
        userPlaylistService.deleteLikedSong(sessionToken, platform, musicId);
    }
}
