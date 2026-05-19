package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.enums.LocalTrackStatus;
import org.thornex.musicparty.persistence.InMemoryLocalTrackRepository;
import org.thornex.musicparty.service.api.LocalMusicApiService;

import java.nio.file.Path;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

class LocalMusicApiServiceTests {

    @TempDir
    Path tempDir;

    @Test
    void searchAndPlayableUseLocalPlatformAndProxyUrls() {
        AppProperties properties = new AppProperties();
        properties.getLocalLibrary().setPath(tempDir.toString());
        InMemoryLocalTrackRepository repository = new InMemoryLocalTrackRepository();
        long now = System.currentTimeMillis();
        repository.upsert(new LocalTrack("track-1", "hash", "song.mp3", null, "audio/mpeg", 3,
                "Local Song", List.of("Alice"), "Album", 1234, null, null, "audio/track-1.ogg",
                LocalTrackStatus.COMPLETED, null, "Transcode completed", 100, "u", now, now, now, now));
        LocalMusicApiService service = new LocalMusicApiService(new LocalLibraryService(properties, repository, mock(LocalTranscodeService.class)));

        assertThat(service.searchMusic("local").block())
                .singleElement()
                .satisfies(music -> {
                    assertThat(music.platform()).isEqualTo("local");
                    assertThat(music.id()).isEqualTo("track-1");
                });

        assertThat(service.getPlayableMusic("track-1").block())
                .satisfies(playable -> {
                    assertThat(playable.platform()).isEqualTo("local");
                    assertThat(playable.url()).isEqualTo("/api/local/media/track-1");
                    assertThat(playable.needsProxy()).isFalse();
                });
    }
}
