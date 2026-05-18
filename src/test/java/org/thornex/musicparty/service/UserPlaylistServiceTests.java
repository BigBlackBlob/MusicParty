package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.web.server.ResponseStatusException;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.persistence.InMemoryUserPlaylistRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.PersistedSession;
import org.thornex.musicparty.persistence.PersistedUserProfile;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.List;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.mock;

class UserPlaylistServiceTests {

    @Test
    void guestUsersCannotCreatePlaylists() {
        TestContext context = new TestContext();
        context.persistUser("token-guest", "u_guest", "游客", true);

        assertThatThrownBy(() -> context.service.createPlaylist("token-guest", "Mine"))
                .isInstanceOf(ResponseStatusException.class)
                .extracting(error -> ((ResponseStatusException) error).getStatusCode())
                .isEqualTo(HttpStatus.FORBIDDEN);
    }

    @Test
    void batchAddSkipsDuplicateMusicKeysAndKeepsOrder() {
        TestContext context = new TestContext();
        context.persistUser("token-a", "u_a", "Alice", false);
        var playlist = context.service.createPlaylist("token-a", "Mine");
        Music first = song("1", "First");
        Music duplicate = song("1", "First again");
        Music second = song("2", "Second");

        var result = context.service.addTracks("token-a", playlist.id(), List.of(first, duplicate, second));

        assertThat(result.addedCount()).isEqualTo(2);
        assertThat(result.skippedCount()).isEqualTo(1);
        assertThat(result.tracks()).extracting(track -> track.music().name()).containsExactly("First", "Second");
    }

    @Test
    void ownerPublicIdScopesPlaylistReadsAndWrites() {
        TestContext context = new TestContext();
        context.persistUser("token-a", "u_a", "Alice", false);
        context.persistUser("token-b", "u_b", "Bob", false);
        var playlist = context.service.createPlaylist("token-a", "Mine");

        assertThatThrownBy(() -> context.service.listTracks("token-b", playlist.id(), 0, 20))
                .isInstanceOf(ResponseStatusException.class)
                .extracting(error -> ((ResponseStatusException) error).getStatusCode())
                .isEqualTo(HttpStatus.NOT_FOUND);
    }

    @Test
    void likedSongsUseProtectedSystemPlaylist() {
        TestContext context = new TestContext();
        context.persistUser("token-a", "u_a", "Alice", false);

        context.service.addLikedSong("token-a", "netease", "1", song("1", "First"));
        context.service.addLikedSong("token-a", "netease", "1", song("1", "Duplicate"));

        var liked = context.service.listLikedSongs("token-a");
        assertThat(liked).hasSize(1);
        var playlist = context.service.listPlaylists("token-a").stream()
                .filter(item -> UserPlaylistService.LIKED_SONGS_SYSTEM_KEY.equals(item.systemKey()))
                .findFirst()
                .orElseThrow();
        assertThat(playlist.trackCount()).isEqualTo(1);
        assertThatThrownBy(() -> context.service.renamePlaylist("token-a", playlist.id(), "Nope"))
                .isInstanceOf(ResponseStatusException.class)
                .extracting(error -> ((ResponseStatusException) error).getStatusCode())
                .isEqualTo(HttpStatus.FORBIDDEN);

        context.service.deleteLikedSong("token-a", "netease", "1");
        assertThat(context.service.listLikedSongs("token-a")).isEmpty();
    }

    @Test
    void exportsPlaylistAsCsvWithStableColumns() {
        TestContext context = new TestContext();
        context.persistUser("token-a", "u_a", "Alice", false);
        var playlist = context.service.createPlaylist("token-a", "Mine");
        context.service.addTracks("token-a", playlist.id(), List.of(song("1", "First")));

        String csv = context.service.exportPlaylist("token-a", playlist.id(), "csv");

        assertThat(csv).startsWith("platform,id,name,artists,duration,coverUrl,externalUrl");
        assertThat(csv).contains("\"netease\",\"1\",\"First\",\"Artist\",\"1000\",\"\",\"https://music.163.com/#/song?id=1\"");
    }

    private static Music song(String id, String name) {
        return new Music(id, name, List.of("Artist"), 1000, "netease", "");
    }

    private static class TestContext {
        private final InMemoryUserProfileRepository users = new InMemoryUserProfileRepository();
        private final UserPlaylistService service = new UserPlaylistService(
                new InMemoryUserPlaylistRepository(),
                users,
                mock(UserService.class),
                mock(MusicPlayerService.class),
                new org.thornex.musicparty.config.AppProperties(),
                new PlaylistExportService(),
                List.of()
        );

        private void persistUser(String sessionToken, String publicId, String name, boolean guest) {
            users.upsertProfile(new PersistedUserProfile(publicId, name, guest, RoomService.DEFAULT_ROOM_ID, 1, 1));
            users.upsertSession(new PersistedSession(hash(sessionToken), publicId, 1, 1));
        }

        private String hash(String sessionToken) {
            try {
                byte[] bytes = MessageDigest.getInstance("SHA-256")
                        .digest(sessionToken.getBytes(StandardCharsets.UTF_8));
                StringBuilder builder = new StringBuilder(bytes.length * 2);
                for (byte value : bytes) builder.append(String.format("%02x", value));
                return builder.toString();
            } catch (Exception e) {
                throw new IllegalStateException(e);
            }
        }
    }
}
