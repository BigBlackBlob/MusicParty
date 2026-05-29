package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.springframework.web.server.ResponseStatusException;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomPlaylistRepository;
import org.thornex.musicparty.persistence.PersistedHistoryEntry;
import org.thornex.musicparty.service.api.SubsonicMusicApiService;

import java.util.List;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class RoomPlaylistServiceTests {

    @Test
    void listsReadOnlyHistoryPlaylistAndReturnsDeduplicatedTracksByRecentPlay() {
        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        RoomPlaylistService service = createService(queueRepository);
        Music oldSong = new Music("old", "Old", List.of("Artist"), 1000, "netease", "");
        Music newerSong = new Music("new", "New", List.of("Artist"), 1000, "bilibili", "");
        append(queueRepository, "room-a", oldSong, 1000);
        append(queueRepository, "room-a", newerSong, 2000);
        append(queueRepository, "room-a", oldSong, 3000);
        append(queueRepository, "room-b", oldSong, 4000);

        assertThat(service.listPlaylists("room-a"))
                .first()
                .satisfies(playlist -> {
                    assertThat(playlist.id()).isEqualTo(RoomPlaylistService.HISTORY_PLAYLIST_ID);
                    assertThat(playlist.systemKey()).isEqualTo(RoomPlaylistService.HISTORY_SYSTEM_KEY);
                    assertThat(playlist.trackCount()).isEqualTo(2);
                });
        assertThat(service.listTracks("room-a", RoomPlaylistService.HISTORY_PLAYLIST_ID, 0, 20))
                .extracting(track -> track.music().id())
                .containsExactly("old", "new");
        assertThat(service.listPlaylists("room-b").getFirst().trackCount()).isEqualTo(1);
    }

    @Test
    void historyPlaylistWriteOperationsAreForbidden() {
        RoomPlaylistService service = createService(new InMemoryQueueRepository());

        assertThatThrownBy(() -> service.renamePlaylist("room-a", RoomPlaylistService.HISTORY_PLAYLIST_ID, "Next"))
                .isInstanceOf(ResponseStatusException.class)
                .hasMessageContaining("403");
        assertThatThrownBy(() -> service.deletePlaylist("room-a", RoomPlaylistService.HISTORY_PLAYLIST_ID))
                .isInstanceOf(ResponseStatusException.class)
                .hasMessageContaining("403");
    }

    private RoomPlaylistService createService(InMemoryQueueRepository queueRepository) {
        SubsonicMusicApiService subsonicMusicApiService = mock(SubsonicMusicApiService.class);
        when(subsonicMusicApiService.supports(org.mockito.ArgumentMatchers.anyString())).thenReturn(false);
        return new RoomPlaylistService(
                new InMemoryRoomPlaylistRepository(),
                queueRepository,
                new AppProperties(),
                mock(NavidromeAccessService.class),
                subsonicMusicApiService,
                mock(SubsonicSourceRegistry.class),
                event -> {},
                new PlaylistExportService(),
                List.of()
        );
    }

    private void append(InMemoryQueueRepository queueRepository, String roomId, Music music, long playedAt) {
        queueRepository.appendHistory(new PersistedHistoryEntry(UUID.randomUUID().toString(), roomId, music, "user", playedAt));
    }
}
