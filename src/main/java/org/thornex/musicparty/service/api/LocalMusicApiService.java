package org.thornex.musicparty.service.api;

import org.springframework.stereotype.Service;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.service.LocalLibraryService;
import reactor.core.publisher.Mono;

import java.util.List;

@Service
public class LocalMusicApiService implements IMusicApiService {
    private final LocalLibraryService localLibraryService;

    public LocalMusicApiService(LocalLibraryService localLibraryService) {
        this.localLibraryService = localLibraryService;
    }

    @Override
    public String getPlatformName() {
        return "local";
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword) {
        return searchMusic(keyword, 0, 20);
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
        return Mono.fromSupplier(() -> localLibraryService.search(keyword, offset, limit).stream().map(LocalTrack::toMusic).toList());
    }

    @Override
    public Mono<PlayableMusic> getPlayableMusic(String musicId) {
        return Mono.fromSupplier(() -> localLibraryService.getPlayableTrack(musicId).toPlayableMusic());
    }

    @Override
    public Mono<List<Playlist>> getUserPlaylists(String userId) {
        return Mono.just(List.of());
    }

    @Override
    public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) {
        return Mono.just(List.of());
    }

    @Override
    public Mono<List<UserSearchResult>> searchUsers(String keyword) {
        return Mono.just(List.of());
    }

    @Override
    public Mono<String> getLyric(String musicId) {
        return Mono.just("");
    }

    @Override
    public Mono<LyricResponse> getLyricDetail(String musicId) {
        return Mono.just(new LyricResponse("", "", ""));
    }
}
