package org.thornex.musicparty.service;

import org.springframework.http.HttpStatus;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.util.StringUtils;
import org.springframework.web.server.ResponseStatusException;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlaylistWriteResult;
import org.thornex.musicparty.dto.UserPlaylist;
import org.thornex.musicparty.dto.UserPlaylistTrack;
import org.thornex.musicparty.persistence.UserPlaylistRepository;
import org.thornex.musicparty.persistence.UserProfileRepository;
import org.thornex.musicparty.service.api.IMusicApiService;
import reactor.core.publisher.Mono;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;

@Service
public class UserPlaylistService {
    public static final String LIKED_SONGS_SYSTEM_KEY = "liked-songs";
    private static final String LIKED_SONGS_NAME = "喜欢的歌曲";
    private final UserPlaylistRepository repository;
    private final UserProfileRepository userProfileRepository;
    private final UserService userService;
    private final MusicPlayerService musicPlayerService;
    private final AppProperties appProperties;
    private final PlaylistExportService playlistExportService;
    private final Map<String, IMusicApiService> apiServiceMap;

    public UserPlaylistService(UserPlaylistRepository repository,
                               UserProfileRepository userProfileRepository,
                               UserService userService,
                               MusicPlayerService musicPlayerService,
                               AppProperties appProperties,
                               PlaylistExportService playlistExportService,
                               List<IMusicApiService> apiServices) {
        this.repository = repository;
        this.userProfileRepository = userProfileRepository;
        this.userService = userService;
        this.musicPlayerService = musicPlayerService;
        this.appProperties = appProperties;
        this.playlistExportService = playlistExportService;
        this.apiServiceMap = apiServices.stream().collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
    }

    public List<UserPlaylist> listPlaylists(String sessionToken) {
        String owner = requireNamedPublicId(sessionToken);
        ensureLikedPlaylist(owner);
        return repository.listPlaylists(owner);
    }

    @Transactional
    public UserPlaylist createPlaylist(String sessionToken, String name) {
        return repository.createPlaylist(requireNamedPublicId(sessionToken), sanitizeName(name));
    }

    @Transactional
    public UserPlaylist renamePlaylist(String sessionToken, String playlistId, String name) {
        String owner = requireNamedPublicId(sessionToken);
        assertMutablePlaylist(owner, playlistId);
        return repository.renamePlaylist(owner, playlistId, sanitizeName(name))
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND));
    }

    @Transactional
    public void deletePlaylist(String sessionToken, String playlistId) {
        String owner = requireNamedPublicId(sessionToken);
        assertMutablePlaylist(owner, playlistId);
        if (!repository.deletePlaylist(owner, playlistId)) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
    }

    public List<UserPlaylistTrack> listTracks(String sessionToken, String playlistId, int offset, int limit) {
        String owner = requireNamedPublicId(sessionToken);
        if (repository.findPlaylist(owner, playlistId).isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        return repository.listTracks(owner, playlistId, offset, limit);
    }

    @Transactional
    public PlaylistWriteResult addTracks(String sessionToken, String playlistId, List<Music> musics) {
        String owner = requireNamedPublicId(sessionToken);
        if (repository.findPlaylist(owner, playlistId).isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        int skipped = 0;
        java.util.ArrayList<UserPlaylistTrack> added = new java.util.ArrayList<>();
        for (Music music : musics == null ? List.<Music>of() : musics) {
            var track = repository.addTrackIfAbsent(owner, playlistId, music);
            if (track.isPresent()) added.add(track.get());
            else skipped++;
        }
        return new PlaylistWriteResult(added.size(), skipped, added);
    }

    @Transactional
    public List<UserPlaylistTrack> listLikedSongs(String sessionToken) {
        String owner = requireNamedPublicId(sessionToken);
        UserPlaylist playlist = ensureLikedPlaylist(owner);
        return repository.listTracks(owner, playlist.id(), 0, 500);
    }

    @Transactional
    public PlaylistWriteResult addLikedSong(String sessionToken, String platform, String musicId, Music music) {
        String owner = requireNamedPublicId(sessionToken);
        Music normalized = normalizeLikedMusic(platform, musicId, music);
        UserPlaylist playlist = ensureLikedPlaylist(owner);
        return repository.addTrackIfAbsent(owner, playlist.id(), normalized)
                .map(track -> new PlaylistWriteResult(1, 0, List.of(track)))
                .orElseGet(() -> new PlaylistWriteResult(0, 1, List.of()));
    }

    @Transactional
    public void deleteLikedSong(String sessionToken, String platform, String musicId) {
        String owner = requireNamedPublicId(sessionToken);
        UserPlaylist playlist = ensureLikedPlaylist(owner);
        repository.deleteTrackByMusicKey(owner, playlist.id(), musicKey(platform, musicId));
    }

    @Transactional
    public void deleteTrack(String sessionToken, String playlistId, String trackId) {
        String owner = requireNamedPublicId(sessionToken);
        if (!repository.deleteTrack(owner, playlistId, trackId)) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
    }

    @Transactional
    public void reorderTracks(String sessionToken, String playlistId, List<String> trackIds) {
        String owner = requireNamedPublicId(sessionToken);
        if (repository.findPlaylist(owner, playlistId).isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        repository.reorderTracks(owner, playlistId, trackIds == null ? List.of() : trackIds);
    }

    public Mono<PlaylistWriteResult> importPlaylist(String sessionToken, String playlistId, String platform, String externalPlaylistId) {
        String owner = requireNamedPublicId(sessionToken);
        if (repository.findPlaylist(owner, playlistId).isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        if (!StringUtils.hasText(externalPlaylistId)) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Playlist id is required");
        if (!StringUtils.hasText(platform)) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Platform is required");
        IMusicApiService service = apiServiceMap.get(platform);
        if (service == null) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Platform not supported");
        int limit = Math.max(1, appProperties.getPlayer().getMaxPlaylistImportSize());
        return service.getPlaylistMusics(externalPlaylistId, 0, limit)
                .map(musics -> addTracks(sessionToken, playlistId, musics.stream().limit(limit).toList()));
    }

    public Mono<PlaylistWriteResult> importNetease(String sessionToken, String playlistId, String externalPlaylistId) {
        return importPlaylist(sessionToken, playlistId, "netease", externalPlaylistId);
    }

    public String exportPlaylist(String sessionToken, String playlistId, String format) {
        String owner = requireNamedPublicId(sessionToken);
        if (repository.findPlaylist(owner, playlistId).isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        List<Music> musics = repository.listTracks(owner, playlistId, 0, 500).stream().map(UserPlaylistTrack::music).toList();
        return playlistExportService.format(musics, format);
    }

    public void enqueue(String sessionToken, String playlistId) {
        String owner = requireNamedPublicId(sessionToken);
        if (repository.findPlaylist(owner, playlistId).isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        List<Music> musics = repository.listTracks(owner, playlistId, 0, appProperties.getPlayer().getMaxPlaylistImportSize())
                .stream()
                .map(UserPlaylistTrack::music)
                .toList();
        String sessionId = userService.getUserBySessionToken(sessionToken)
                .map(org.thornex.musicparty.dto.User::getSessionId)
                .filter(StringUtils::hasText)
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.UNAUTHORIZED));
        musicPlayerService.enqueueSavedPlaylist(musics, sessionId);
    }

    private String requireNamedPublicId(String sessionToken) {
        if (!StringUtils.hasText(sessionToken)) throw new ResponseStatusException(HttpStatus.UNAUTHORIZED);
        var session = userProfileRepository.findSessionByHash(hashSessionToken(sessionToken))
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.UNAUTHORIZED));
        var profile = userProfileRepository.findByPublicId(session.publicId())
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.UNAUTHORIZED));
        if (profile.guest()) throw new ResponseStatusException(HttpStatus.FORBIDDEN);
        return profile.publicId();
    }

    private String sanitizeName(String name) {
        String value = name == null ? "" : name.trim();
        if (value.isEmpty()) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Playlist name is required");
        return value.length() > 80 ? value.substring(0, 80) : value;
    }

    private UserPlaylist ensureLikedPlaylist(String owner) {
        return repository.findSystemPlaylist(owner, LIKED_SONGS_SYSTEM_KEY)
                .orElseGet(() -> repository.createSystemPlaylist(owner, LIKED_SONGS_NAME, LIKED_SONGS_SYSTEM_KEY));
    }

    private void assertMutablePlaylist(String owner, String playlistId) {
        UserPlaylist playlist = repository.findPlaylist(owner, playlistId)
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND));
        if (StringUtils.hasText(playlist.systemKey())) {
            throw new ResponseStatusException(HttpStatus.FORBIDDEN, "System playlist cannot be modified");
        }
    }

    private Music normalizeLikedMusic(String platform, String musicId, Music music) {
        if (!StringUtils.hasText(platform) || !StringUtils.hasText(musicId)) {
            throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Music key is required");
        }
        if (music == null) return new Music(musicId, musicId, List.of(), 0, platform, "");
        return new Music(musicId, music.name(), music.artists(), music.duration(), platform, music.coverUrl());
    }

    private String musicKey(String platform, String musicId) {
        return String.valueOf(platform) + ":" + String.valueOf(musicId);
    }


    private String hashSessionToken(String sessionToken) {
        try {
            byte[] bytes = MessageDigest.getInstance("SHA-256").digest(sessionToken.getBytes(StandardCharsets.UTF_8));
            StringBuilder builder = new StringBuilder(bytes.length * 2);
            for (byte value : bytes) builder.append(String.format("%02x", value));
            return builder.toString();
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is unavailable", e);
        }
    }
}
