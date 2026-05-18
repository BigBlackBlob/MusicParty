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
    private final UserPlaylistRepository repository;
    private final UserProfileRepository userProfileRepository;
    private final UserService userService;
    private final MusicPlayerService musicPlayerService;
    private final AppProperties appProperties;
    private final Map<String, IMusicApiService> apiServiceMap;

    public UserPlaylistService(UserPlaylistRepository repository,
                               UserProfileRepository userProfileRepository,
                               UserService userService,
                               MusicPlayerService musicPlayerService,
                               AppProperties appProperties,
                               List<IMusicApiService> apiServices) {
        this.repository = repository;
        this.userProfileRepository = userProfileRepository;
        this.userService = userService;
        this.musicPlayerService = musicPlayerService;
        this.appProperties = appProperties;
        this.apiServiceMap = apiServices.stream().collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
    }

    public List<UserPlaylist> listPlaylists(String sessionToken) {
        return repository.listPlaylists(requireNamedPublicId(sessionToken));
    }

    @Transactional
    public UserPlaylist createPlaylist(String sessionToken, String name) {
        return repository.createPlaylist(requireNamedPublicId(sessionToken), sanitizeName(name));
    }

    @Transactional
    public UserPlaylist renamePlaylist(String sessionToken, String playlistId, String name) {
        String owner = requireNamedPublicId(sessionToken);
        return repository.renamePlaylist(owner, playlistId, sanitizeName(name))
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND));
    }

    @Transactional
    public void deletePlaylist(String sessionToken, String playlistId) {
        String owner = requireNamedPublicId(sessionToken);
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

    public Mono<PlaylistWriteResult> importNetease(String sessionToken, String playlistId, String externalPlaylistId) {
        String owner = requireNamedPublicId(sessionToken);
        if (repository.findPlaylist(owner, playlistId).isEmpty()) throw new ResponseStatusException(HttpStatus.NOT_FOUND);
        if (!StringUtils.hasText(externalPlaylistId)) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Playlist id is required");
        IMusicApiService service = apiServiceMap.get("netease");
        if (service == null) throw new ResponseStatusException(HttpStatus.BAD_REQUEST, "Platform not supported");
        int limit = Math.max(1, appProperties.getPlayer().getMaxPlaylistImportSize());
        return service.getPlaylistMusics(externalPlaylistId, 0, limit)
                .map(musics -> addTracks(sessionToken, playlistId, musics.stream().limit(limit).toList()));
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
