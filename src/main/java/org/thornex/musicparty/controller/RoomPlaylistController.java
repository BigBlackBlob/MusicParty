package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
import org.thornex.musicparty.dto.RoomPlaylist;
import org.thornex.musicparty.dto.RoomPlaylistRequests.AddTrackRequest;
import org.thornex.musicparty.dto.RoomPlaylistRequests.CreatePlaylistRequest;
import org.thornex.musicparty.dto.RoomPlaylistRequests.ImportPlaylistRequest;
import org.thornex.musicparty.dto.RoomPlaylistRequests.ReorderTracksRequest;
import org.thornex.musicparty.dto.RoomPlaylistRequests.UpdatePlaylistRequest;
import org.thornex.musicparty.dto.RoomPlaylistTrack;
import org.thornex.musicparty.service.RoomAccessService;
import org.thornex.musicparty.service.RoomPlaylistService;
import org.thornex.musicparty.service.UserService;
import reactor.core.publisher.Mono;

import java.util.List;

@RestController
@RequestMapping("/api/rooms/{roomId}/playlists")
@RequiredArgsConstructor
public class RoomPlaylistController {
    private final RoomPlaylistService roomPlaylistService;
    private final UserService userService;
    private final RoomAccessService roomAccessService;

    @GetMapping
    public List<RoomPlaylist> list(@PathVariable String roomId) {
        return roomPlaylistService.listPlaylists(roomId);
    }

    @PostMapping
    public RoomPlaylist create(@PathVariable String roomId,
                               @RequestBody CreatePlaylistRequest request,
                               @RequestParam(required = false) String sessionToken,
                               @RequestParam(required = false) String roomAccessToken) {
        requireRoomAccess(roomId, sessionToken, roomAccessToken);
        return roomPlaylistService.createPlaylist(roomId, request.name());
    }

    @PatchMapping("/{playlistId}")
    public RoomPlaylist rename(@PathVariable String roomId,
                               @PathVariable String playlistId,
                               @RequestBody UpdatePlaylistRequest request,
                               @RequestParam(required = false) String sessionToken,
                               @RequestParam(required = false) String roomAccessToken) {
        requireRoomAccess(roomId, sessionToken, roomAccessToken);
        return roomPlaylistService.renamePlaylist(roomId, playlistId, request.name());
    }

    @DeleteMapping("/{playlistId}")
    public void delete(@PathVariable String roomId,
                       @PathVariable String playlistId,
                       @RequestParam(required = false) String sessionToken,
                       @RequestParam(required = false) String roomAccessToken) {
        requireRoomAccess(roomId, sessionToken, roomAccessToken);
        roomPlaylistService.deletePlaylist(roomId, playlistId);
    }

    @GetMapping("/{playlistId}/tracks")
    public List<RoomPlaylistTrack> tracks(@PathVariable String roomId,
                                          @PathVariable String playlistId,
                                          @RequestParam(defaultValue = "0") int offset,
                                          @RequestParam(defaultValue = "100") int limit) {
        return roomPlaylistService.listTracks(roomId, playlistId, offset, limit);
    }

    @PostMapping("/{playlistId}/tracks")
    public RoomPlaylistTrack addTrack(@PathVariable String roomId,
                                      @PathVariable String playlistId,
                                      @RequestBody AddTrackRequest request,
                                      @RequestParam(required = false) String sessionToken,
                                      @RequestParam(required = false) String roomAccessToken) {
        requireRoomAccess(roomId, sessionToken, roomAccessToken);
        return roomPlaylistService.addTrack(roomId, playlistId, request.music());
    }

    @DeleteMapping("/{playlistId}/tracks/{trackId}")
    public void deleteTrack(@PathVariable String roomId,
                            @PathVariable String playlistId,
                            @PathVariable String trackId,
                            @RequestParam(required = false) String sessionToken,
                            @RequestParam(required = false) String roomAccessToken) {
        requireRoomAccess(roomId, sessionToken, roomAccessToken);
        roomPlaylistService.deleteTrack(roomId, playlistId, trackId);
    }

    @PostMapping("/{playlistId}/tracks/reorder")
    public void reorder(@PathVariable String roomId,
                        @PathVariable String playlistId,
                        @RequestBody ReorderTracksRequest request,
                        @RequestParam(required = false) String sessionToken,
                        @RequestParam(required = false) String roomAccessToken) {
        requireRoomAccess(roomId, sessionToken, roomAccessToken);
        roomPlaylistService.reorderTracks(roomId, playlistId, request.trackIds());
    }

    @PostMapping("/{playlistId}/import")
    public Mono<List<RoomPlaylistTrack>> importPlaylist(@PathVariable String roomId,
                                                        @PathVariable String playlistId,
                                                        @RequestBody ImportPlaylistRequest request,
                                                        @RequestParam(required = false) String token,
                                                        @RequestParam(required = false) String sessionToken,
                                                        @RequestParam(required = false) String roomAccessToken) {
        requireRoomAccess(roomId, sessionToken, roomAccessToken);
        return roomPlaylistService.importPlaylist(roomId, playlistId, request.platform(), request.playlistId(), token);
    }

    @GetMapping(value = "/{playlistId}/export", produces = MediaType.TEXT_PLAIN_VALUE)
    public String export(@PathVariable String roomId,
                         @PathVariable String playlistId,
                         @RequestParam(defaultValue = "txt") String format) {
        return roomPlaylistService.exportPlaylist(roomId, playlistId, format);
    }

    private void requireRoomAccess(String roomId, String sessionToken, String roomAccessToken) {
        String publicId = userService.resolvePublicIdBySessionToken(sessionToken)
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.UNAUTHORIZED, "Unknown session token"));
        if (!roomAccessService.validateAccessToken(roomId, publicId, roomAccessToken)) {
            throw new ResponseStatusException(HttpStatus.FORBIDDEN, "Room access denied");
        }
    }
}
