package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import reactor.core.publisher.Mono;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class NavidromeMusicApiServiceTests {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    void searchAlbumsMapsSubsonicAlbums() throws Exception {
        NavidromeSubsonicClient client = mock(NavidromeSubsonicClient.class);
        when(client.getJson(eq("search3.view"), eq(Map.of(
                "query", "blue",
                "songCount", "0",
                "artistCount", "0",
                "albumCount", "50"
        )))).thenReturn(Mono.just(MAPPER.readTree("""
                {"subsonic-response":{"status":"ok","searchResult3":{"album":[
                  {"id":"album-1","name":"Blue Album","artist":"Alice","coverArt":"cover-1","songCount":12}
                ]}}}
                """)));
        when(client.buildLocalProxyPath("/api/navidrome/cover", "cover-1"))
                .thenReturn("/api/navidrome/cover/cover-1");

        NavidromeMusicApiService service = new NavidromeMusicApiService(newProperties(), client);

        var albums = service.searchAlbums("blue").block();

        assertThat(albums).hasSize(1);
        assertThat(albums.getFirst().id()).isEqualTo("album-1");
        assertThat(albums.getFirst().name()).isEqualTo("Blue Album");
        assertThat(albums.getFirst().artistName()).isEqualTo("Alice");
        assertThat(albums.getFirst().coverUrl()).isEqualTo("/api/navidrome/cover/cover-1");
        assertThat(albums.getFirst().trackCount()).isEqualTo(12);
        assertThat(albums.getFirst().platform()).isEqualTo("navidrome");
    }

    @Test
    void getAlbumMusicsMapsSubsonicAlbumSongs() throws Exception {
        NavidromeSubsonicClient client = mock(NavidromeSubsonicClient.class);
        when(client.getJson(eq("getAlbum.view"), eq(Map.of("id", "album-1"))))
                .thenReturn(Mono.just(MAPPER.readTree("""
                {"subsonic-response":{"status":"ok","album":{"song":[
                  {"id":"song-1","title":"Track One","artist":"Alice","duration":180,"coverArt":"cover-1"}
                ]}}}
                """)));
        when(client.buildLocalProxyPath("/api/navidrome/cover", "cover-1"))
                .thenReturn("/api/navidrome/cover/cover-1");

        NavidromeMusicApiService service = new NavidromeMusicApiService(newProperties(), client);

        var songs = service.getAlbumMusics("album-1").block();

        assertThat(songs).hasSize(1);
        assertThat(songs.getFirst().id()).isEqualTo("song-1");
        assertThat(songs.getFirst().name()).isEqualTo("Track One");
        assertThat(songs.getFirst().artists()).containsExactly("Alice");
        assertThat(songs.getFirst().duration()).isEqualTo(180_000);
        assertThat(songs.getFirst().coverUrl()).isEqualTo("/api/navidrome/cover/cover-1");
        assertThat(songs.getFirst().platform()).isEqualTo("navidrome");
    }

    private AppProperties newProperties() {
        AppProperties properties = new AppProperties();
        AppProperties.NavidromeApiConfig navidrome = new AppProperties.NavidromeApiConfig();
        navidrome.setBaseUrl("http://127.0.0.1:4533");
        navidrome.setUsername("admin");
        navidrome.setPassword("secret");
        properties.setNavidrome(navidrome);
        return properties;
    }
}
