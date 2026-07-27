package org.thornex.musicparty.service.api;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.UserSearchResult;
import reactor.core.publisher.Mono;

import java.util.List;

/** Deterministic local-only source used by the performance acceptance harness. */
@Service
@ConditionalOnProperty(prefix = "app.music-api.fixture", name = "enabled", havingValue = "true")
public class FixtureMusicApiService implements IMusicApiService {
    @Override public String getPlatformName() { return "fixture"; }

    @Override public Mono<List<Music>> searchMusic(String keyword) {
        return Mono.just(List.of(music(keyword == null || keyword.isBlank() ? "fixture-1" : keyword)));
    }

    @Override public Mono<PlayableMusic> getPlayableMusic(String musicId) {
        Music music = music(musicId);
        return Mono.just(new PlayableMusic(music.id(), music.name(), music.artists(), music.duration(),
                music.platform(), "https://fixture.invalid/audio", "", false));
    }

    @Override public Mono<List<Playlist>> getUserPlaylists(String userId) { return Mono.just(List.of()); }
    @Override public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) { return Mono.just(List.of()); }
    @Override public Mono<List<UserSearchResult>> searchUsers(String keyword) { return Mono.just(List.of()); }
    @Override public Mono<String> getLyric(String musicId) { return Mono.just(""); }

    private Music music(String id) {
        String normalized = id == null || id.isBlank() ? "fixture-1" : id;
        return new Music(normalized, "Fixture " + normalized, List.of("Performance Fixture"), 180_000L, "fixture", "");
    }
}
