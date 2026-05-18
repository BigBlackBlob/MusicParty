package org.thornex.musicparty.service;

import org.springframework.context.ApplicationEventPublisher;
import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.server.ResponseStatusException;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.RoomPlaylist;
import org.thornex.musicparty.dto.RoomPlaylistTrack;
import org.thornex.musicparty.event.RoomPlaylistUpdateEvent;
import org.thornex.musicparty.persistence.RoomPlaylistRepository;
import org.thornex.musicparty.service.api.IMusicApiService;
import org.thornex.musicparty.service.api.SubsonicMusicApiService;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.function.Function;
import java.util.stream.Collectors;

@Service
public class RoomPlaylistService {
    private final RoomPlaylistRepository repository;
    private final AppProperties appProperties;
    private final NavidromeAccessService navidromeAccessService;
    private final SubsonicMusicApiService subsonicMusicApiService;
    private final SubsonicSourceRegistry subsonicSourceRegistry;
    private final ApplicationEventPublisher eventPublisher;
    private final Map<String, IMusicApiService> apiServiceMap;

    public RoomPlaylistService(RoomPlaylistRepository repository,
                               AppProperties appProperties,
                               NavidromeAccessService navidromeAccessService,
                               SubsonicMusicApiService subsonicMusicApiService,
                               SubsonicSourceRegistry subsonicSourceRegistry,
                               ApplicationEventPublisher eventPublisher,
                               List<IMusicApiService> apiServices) {
        this.repository = repository;
        this.appProperties = appProperties;
        this.navidromeAccessService = navidromeAccessService;
        this.subsonicMusicApiService = subsonicMusicApiService;
        this.subsonicSourceRegistry = subsonicSourceRegistry;
        this.eventPublisher = eventPublisher;
        this.apiServiceMap = apiServices.stream().collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
    }

    public List<RoomPlaylist> listPlaylists(String roomId) {
        return repository.listPlaylists(normalizeRoom(roomId));
    }

    @Transactional
    public RoomPlaylist createPlaylist(String roomId, String name) {
        RoomPlaylist playlist = repository.createPlaylist(normalizeRoom(roomId), sanitizeName(name));
        publishUpdate(playlist.roomId());
        return playlist;
    }

    @Transactional
    public RoomPlaylist renamePlaylist(String roomId, String playlistId, String name) {
        String normalizedRoom = normalizeRoom(roomId);
        RoomPlaylist playlist = repository.renamePlaylist(normalizedRoom, playlistId, sanitizeName(name))
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND));
        publishUpdate(normalizedRoom);
        return playlist;
    }

    @Transactional
    public void deletePlaylist(String roomId, String playlistId) {
        String normalizedRoom = normalizeRoom(roomId);
        if (!repository.deletePlaylist(normalizedRoom, playlistId)) {
            throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        }
        publishUpdate(normalizedRoom);
    }

    public List<RoomPlaylistTrack> listTracks(String roomId, String playlistId, int offset, int limit) {
        return repository.listTracks(normalizeRoom(roomId), playlistId, offset, limit);
    }

    @Transactional
    public RoomPlaylistTrack addTrack(String roomId, String playlistId, Music music) {
        String normalizedRoom = normalizeRoom(roomId);
        RoomPlaylistTrack track = repository.addTrack(normalizedRoom, playlistId, music)
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND));
        publishUpdate(normalizedRoom);
        return track;
    }

    @Transactional
    public void deleteTrack(String roomId, String playlistId, String trackId) {
        String normalizedRoom = normalizeRoom(roomId);
        if (!repository.deleteTrack(normalizedRoom, playlistId, trackId)) {
            throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        }
        publishUpdate(normalizedRoom);
    }

    @Transactional
    public void reorderTracks(String roomId, String playlistId, List<String> trackIds) {
        String normalizedRoom = normalizeRoom(roomId);
        repository.reorderTracks(normalizedRoom, playlistId, trackIds == null ? List.of() : trackIds);
        publishUpdate(normalizedRoom);
    }

    public Mono<List<RoomPlaylistTrack>> importPlaylist(String roomId, String playlistId, String platform, String externalPlaylistId, String token) {
        String normalizedRoom = normalizeRoom(roomId);
        assertImportAllowed(normalizedRoom, platform, token);
        int limit = Math.max(1, appProperties.getPlayer().getMaxPlaylistImportSize());
        return getService(platform, normalizedRoom).getPlaylistMusics(externalPlaylistId, 0, limit)
                .map(musics -> {
                    List<RoomPlaylistTrack> tracks = musics.stream()
                            .limit(limit)
                            .map(music -> repository.addTrack(normalizedRoom, playlistId, music))
                            .flatMap(Optional::stream)
                            .toList();
                    publishUpdate(normalizedRoom);
                    return tracks;
                });
    }

    public List<Music> getPlaylistMusics(String roomId, String playlistId) {
        return repository.listTracks(normalizeRoom(roomId), playlistId, 0, appProperties.getPlayer().getMaxPlaylistImportSize())
                .stream()
                .map(RoomPlaylistTrack::music)
                .toList();
    }

    private void assertImportAllowed(String roomId, String platform, String token) {
        if ("navidrome".equals(platform) && (token == null || !navidromeAccessService.canUseBySessionToken(token))) {
            throw new ResponseStatusException(HttpStatus.FORBIDDEN);
        }
        if (subsonicMusicApiService.supports(platform)) {
            RoomSubsonicSource source = subsonicSourceRegistry.findRoomSourceByPlatformId(roomId, platform)
                    .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND));
            boolean allowed = source.allowedUsers() != null && source.allowedUsers().contains("*")
                    || token != null && navidromeAccessService.canUseBySessionToken(token, source.allowedUsers());
            if (!allowed) throw new ResponseStatusException(HttpStatus.FORBIDDEN);
        }
    }

    private IMusicApiService getService(String platform, String roomId) {
        if (subsonicMusicApiService.supports(platform)) {
            return new IMusicApiService() {
                @Override public String getPlatformName() { return platform; }
                @Override public reactor.core.publisher.Mono<List<Music>> searchMusic(String keyword) { return searchMusic(keyword, 0, 20); }
                @Override public reactor.core.publisher.Mono<List<Music>> searchMusic(String keyword, int offset, int limit) { return subsonicMusicApiService.searchMusic(roomId, platform, keyword, offset, limit); }
                @Override public reactor.core.publisher.Mono<org.thornex.musicparty.dto.PlayableMusic> getPlayableMusic(String musicId) { return subsonicMusicApiService.getPlayableMusic(platform, musicId); }
                @Override public reactor.core.publisher.Mono<List<org.thornex.musicparty.dto.Playlist>> getUserPlaylists(String userId) { return Mono.just(List.of()); }
                @Override public reactor.core.publisher.Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) { return Mono.just(List.of()); }
                @Override public reactor.core.publisher.Mono<List<org.thornex.musicparty.dto.UserSearchResult>> searchUsers(String keyword) { return Mono.just(List.of()); }
                @Override public reactor.core.publisher.Mono<String> getLyric(String musicId) { return Mono.just(""); }
            };
        }
        IMusicApiService service = apiServiceMap.get(platform);
        if (service == null) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Platform not supported");
        return service;
    }

    private String sanitizeName(String name) {
        String value = name == null ? "" : name.trim();
        if (value.isEmpty()) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Playlist name is required");
        return value.length() > 80 ? value.substring(0, 80) : value;
    }

    private String normalizeRoom(String roomId) {
        return roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
    }

    private void publishUpdate(String roomId) {
        eventPublisher.publishEvent(new RoomPlaylistUpdateEvent(this, roomId));
    }
}
