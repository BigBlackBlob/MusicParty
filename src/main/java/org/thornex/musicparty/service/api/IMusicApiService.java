package org.thornex.musicparty.service.api;

import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.LyricResponse;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.dto.Album;
import reactor.core.publisher.Mono;

import java.util.List;

public interface IMusicApiService {
    String getPlatformName();
    Mono<List<Music>> searchMusic(String keyword);
    default Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
        return searchMusic(keyword); // Default fallback for platforms not yet supporting pagination
    }
    Mono<PlayableMusic> getPlayableMusic(String musicId);
    Mono<List<Playlist>> getUserPlaylists(String userId);
    Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit);
    default Mono<List<Album>> searchAlbums(String keyword) {
        return Mono.just(List.of());
    }
    default Mono<List<Music>> getAlbumMusics(String albumId) {
        return Mono.just(List.of());
    }
    Mono<List<UserSearchResult>> searchUsers(String keyword);
    Mono<String> getLyric(String musicId);
    default Mono<LyricResponse> getLyricDetail(String musicId) {
        return getLyric(musicId).map(lyric -> new LyricResponse(lyric, "", ""));
    }
    default void prefetchMusic(String musicId) {};
}
