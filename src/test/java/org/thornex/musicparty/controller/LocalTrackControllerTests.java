package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.mock.web.MockMultipartFile;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.LocalTrack;
import org.thornex.musicparty.dto.LocalUploadResult;
import org.thornex.musicparty.enums.LocalTrackStatus;
import org.thornex.musicparty.persistence.InMemoryLocalTrackRepository;
import org.thornex.musicparty.service.LocalLibraryAccessService;
import org.thornex.musicparty.service.LocalLibraryService;
import org.thornex.musicparty.service.UserService;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class LocalTrackControllerTests {

    @Test
    void uploadRejectsUserOutsideAdminAndAllowlist() {
        AppProperties properties = new AppProperties();
        properties.setAdminPassword("secret");
        LocalLibraryAccessService accessService = new LocalLibraryAccessService(properties, mock(UserService.class), new InMemoryLocalTrackRepository());
        LocalTrackController controller = new LocalTrackController(mock(LocalLibraryService.class), accessService, new InternalStreamProxyToken());

        var response = controller.uploadTrack(
                new MockMultipartFile("file", "song.mp3", "audio/mpeg", "abc".getBytes()),
                null,
                "wrong",
                null,
                null,
                null
        );

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.FORBIDDEN);
    }

    @Test
    void adminUploadDelegatesToLibraryService() throws Exception {
        AppProperties properties = new AppProperties();
        properties.setAdminPassword("secret");
        LocalLibraryService libraryService = mock(LocalLibraryService.class);
        LocalTrack track = new LocalTrack("id", "hash", "song.mp3", null, "audio/mpeg", 3,
                "Song", List.of("Artist"), "", 0, null, null, null, LocalTrackStatus.QUEUED,
                null, "Queued for transcoding", 0, "admin", 1, 1, null, null);
        LocalUploadResult result = LocalUploadResult.created(track);
        when(libraryService.upload(any(), eq("admin"), eq("Song"), eq("Artist"), eq("Album"))).thenReturn(result);
        LocalLibraryAccessService accessService = new LocalLibraryAccessService(properties, mock(UserService.class), new InMemoryLocalTrackRepository());
        LocalTrackController controller = new LocalTrackController(libraryService, accessService, new InternalStreamProxyToken());

        var response = controller.uploadTrack(
                new MockMultipartFile("file", "song.mp3", "audio/mpeg", "abc".getBytes()),
                null,
                "secret",
                "Song",
                "Artist",
                "Album"
        );

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.OK);
        assertThat(response.getBody()).isEqualTo(result);
    }
}
