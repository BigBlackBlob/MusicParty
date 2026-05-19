package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.mock.web.MockMultipartFile;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.enums.LocalTrackStatus;
import org.thornex.musicparty.persistence.InMemoryLocalTrackRepository;

import java.nio.file.Path;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.ArgumentMatchers.isA;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;

class LocalLibraryServiceTests {

    @TempDir
    Path tempDir;

    @Test
    void uploadCreatesPendingTrackAndQueuesTranscode() throws Exception {
        AppProperties properties = new AppProperties();
        properties.getLocalLibrary().setPath(tempDir.toString());
        InMemoryLocalTrackRepository repository = new InMemoryLocalTrackRepository();
        LocalTranscodeService transcodeService = mock(LocalTranscodeService.class);
        LocalLibraryService service = new LocalLibraryService(properties, repository, transcodeService);

        LocalTrack track = service.upload(
                new MockMultipartFile("file", "song.mp3", "audio/mpeg", "abc".getBytes()),
                "alice",
                "Manual Title",
                "Alice, Bob",
                "Manual Album"
        ).track();

        assertThat(track.status()).isEqualTo(LocalTrackStatus.QUEUED);
        assertThat(track.originalHash()).isNotBlank();
        assertThat(track.originalFileName()).isEqualTo("song.mp3");
        assertThat(track.title()).isEqualTo("Manual Title");
        assertThat(track.artists()).containsExactly("Alice", "Bob");
        assertThat(repository.findById(track.id())).isPresent();
        verify(transcodeService).enqueue(eq(track.id()), isA(Path.class));
    }

    @Test
    void completedTracksAreSearchableOnlyAfterTranscode() {
        AppProperties properties = new AppProperties();
        properties.getLocalLibrary().setPath(tempDir.toString());
        InMemoryLocalTrackRepository repository = new InMemoryLocalTrackRepository();
        LocalLibraryService service = new LocalLibraryService(properties, repository, mock(LocalTranscodeService.class));
        long now = System.currentTimeMillis();
        repository.upsert(localTrack("pending", "Alpha Song", LocalTrackStatus.QUEUED, null, now));
        repository.upsert(localTrack("done", "Alpha Done", LocalTrackStatus.COMPLETED, "audio/done.ogg", now));

        assertThat(service.search("alpha", 0, 20))
                .extracting(LocalTrack::id)
                .containsExactly("done");
    }

    @Test
    void uploadReturnsDuplicateWhenOriginalHashAlreadyExists() throws Exception {
        AppProperties properties = new AppProperties();
        properties.getLocalLibrary().setPath(tempDir.toString());
        InMemoryLocalTrackRepository repository = new InMemoryLocalTrackRepository();
        LocalLibraryService service = new LocalLibraryService(properties, repository, mock(LocalTranscodeService.class));

        var first = service.upload(new MockMultipartFile("file", "a.mp3", "audio/mpeg", "abc".getBytes()), "alice", null, null, null);
        var duplicate = service.upload(new MockMultipartFile("file", "b.mp3", "audio/mpeg", "abc".getBytes()), "bob", null, null, null);

        assertThat(duplicate.duplicate()).isTrue();
        assertThat(duplicate.duplicateOf()).isEqualTo(first.track().id());
        assertThat(repository.findAll()).hasSize(1);
    }

    private static LocalTrack localTrack(String id, String title, LocalTrackStatus status, String oggPath, long now) {
        return new LocalTrack(id, id + "-hash", id + ".mp3", null, "audio/mpeg", 3,
                title, java.util.List.of("A"), "", 0, null, null, oggPath, status, null,
                null, status == LocalTrackStatus.COMPLETED ? 100 : 0, "u", now, now, null,
                status == LocalTrackStatus.COMPLETED ? now : null);
    }
}
