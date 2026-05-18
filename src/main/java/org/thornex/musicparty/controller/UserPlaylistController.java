package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.*;
import org.thornex.musicparty.dto.PlaylistWriteResult;
import org.thornex.musicparty.dto.UserPlaylist;
import org.thornex.musicparty.dto.UserPlaylistRequests;
import org.thornex.musicparty.dto.UserPlaylistTrack;
import org.thornex.musicparty.service.UserPlaylistService;
import reactor.core.publisher.Mono;

import java.util.List;

@RestController
@RequestMapping("/api/me/playlists")
@RequiredArgsConstructor
public class UserPlaylistController {
    private final UserPlaylistService userPlaylistService;

    @GetMapping
    public List<UserPlaylist> list(@RequestParam String sessionToken) {
        return userPlaylistService.listPlaylists(sessionToken);
    }

    @PostMapping
    public UserPlaylist create(@RequestParam String sessionToken, @RequestBody UserPlaylistRequests.CreatePlaylistRequest request) {
        return userPlaylistService.createPlaylist(sessionToken, request.name());
    }

    @PatchMapping("/{playlistId}")
    public UserPlaylist rename(@RequestParam String sessionToken,
                               @PathVariable String playlistId,
                               @RequestBody UserPlaylistRequests.UpdatePlaylistRequest request) {
        return userPlaylistService.renamePlaylist(sessionToken, playlistId, request.name());
    }

    @DeleteMapping("/{playlistId}")
    public void delete(@RequestParam String sessionToken, @PathVariable String playlistId) {
        userPlaylistService.deletePlaylist(sessionToken, playlistId);
    }

    @GetMapping("/{playlistId}/tracks")
    public List<UserPlaylistTrack> tracks(@RequestParam String sessionToken,
                                          @PathVariable String playlistId,
                                          @RequestParam(defaultValue = "0") int offset,
                                          @RequestParam(defaultValue = "100") int limit) {
        return userPlaylistService.listTracks(sessionToken, playlistId, offset, limit);
    }

    @PostMapping("/{playlistId}/tracks/batch")
    public PlaylistWriteResult addTracks(@RequestParam String sessionToken,
                                         @PathVariable String playlistId,
                                         @RequestBody UserPlaylistRequests.AddUserPlaylistTracksRequest request) {
        return userPlaylistService.addTracks(sessionToken, playlistId, request.musics());
    }

    @DeleteMapping("/{playlistId}/tracks/{trackId}")
    public void deleteTrack(@RequestParam String sessionToken, @PathVariable String playlistId, @PathVariable String trackId) {
        userPlaylistService.deleteTrack(sessionToken, playlistId, trackId);
    }

    @PostMapping("/{playlistId}/tracks/reorder")
    public void reorder(@RequestParam String sessionToken,
                        @PathVariable String playlistId,
                        @RequestBody UserPlaylistRequests.ReorderTracksRequest request) {
        userPlaylistService.reorderTracks(sessionToken, playlistId, request.trackIds());
    }

    @PostMapping("/{playlistId}/import/netease")
    public Mono<PlaylistWriteResult> importNetease(@RequestParam String sessionToken,
                                                   @PathVariable String playlistId,
                                                   @RequestBody UserPlaylistRequests.ImportNeteasePlaylistRequest request) {
        return userPlaylistService.importNetease(sessionToken, playlistId, request.playlistId());
    }

    @PostMapping("/{playlistId}/enqueue")
    public void enqueue(@RequestParam String sessionToken, @PathVariable String playlistId) {
        userPlaylistService.enqueue(sessionToken, playlistId);
    }
}
