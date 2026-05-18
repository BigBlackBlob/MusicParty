package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.UserPlaylist;
import org.thornex.musicparty.dto.UserPlaylistTrack;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryUserPlaylistRepository implements UserPlaylistRepository {
    private final Map<String, MutablePlaylist> playlists = new ConcurrentHashMap<>();
    private final Map<String, List<UserPlaylistTrack>> tracks = new ConcurrentHashMap<>();

    @Override
    public synchronized List<UserPlaylist> listPlaylists(String ownerPublicId) {
        return playlists.values().stream()
                .filter(playlist -> playlist.ownerPublicId.equals(ownerPublicId))
                .sorted(Comparator.comparingLong(playlist -> playlist.createdAt))
                .map(this::toDto)
                .toList();
    }

    @Override
    public synchronized Optional<UserPlaylist> findPlaylist(String ownerPublicId, String playlistId) {
        MutablePlaylist playlist = playlists.get(playlistId);
        if (playlist == null || !playlist.ownerPublicId.equals(ownerPublicId)) return Optional.empty();
        return Optional.of(toDto(playlist));
    }

    @Override
    public synchronized Optional<UserPlaylist> findSystemPlaylist(String ownerPublicId, String systemKey) {
        return playlists.values().stream()
                .filter(playlist -> playlist.ownerPublicId.equals(ownerPublicId))
                .filter(playlist -> java.util.Objects.equals(playlist.systemKey, systemKey))
                .findFirst()
                .map(this::toDto);
    }

    @Override
    public synchronized UserPlaylist createPlaylist(String ownerPublicId, String name) {
        long now = System.currentTimeMillis();
        MutablePlaylist playlist = new MutablePlaylist(UUID.randomUUID().toString(), ownerPublicId, name, null, now, now);
        playlists.put(playlist.id, playlist);
        tracks.put(playlist.id, new ArrayList<>());
        return toDto(playlist);
    }

    @Override
    public synchronized UserPlaylist createSystemPlaylist(String ownerPublicId, String name, String systemKey) {
        Optional<UserPlaylist> existing = findSystemPlaylist(ownerPublicId, systemKey);
        if (existing.isPresent()) return existing.get();
        long now = System.currentTimeMillis();
        MutablePlaylist playlist = new MutablePlaylist(UUID.randomUUID().toString(), ownerPublicId, name, systemKey, now, now);
        playlists.put(playlist.id, playlist);
        tracks.put(playlist.id, new ArrayList<>());
        return toDto(playlist);
    }

    @Override
    public synchronized Optional<UserPlaylist> renamePlaylist(String ownerPublicId, String playlistId, String name) {
        MutablePlaylist playlist = playlists.get(playlistId);
        if (playlist == null || !playlist.ownerPublicId.equals(ownerPublicId)) return Optional.empty();
        playlist.name = name;
        playlist.updatedAt = System.currentTimeMillis();
        return Optional.of(toDto(playlist));
    }

    @Override
    public synchronized boolean deletePlaylist(String ownerPublicId, String playlistId) {
        MutablePlaylist playlist = playlists.get(playlistId);
        if (playlist == null || !playlist.ownerPublicId.equals(ownerPublicId)) return false;
        playlists.remove(playlistId);
        tracks.remove(playlistId);
        return true;
    }

    @Override
    public synchronized List<UserPlaylistTrack> listTracks(String ownerPublicId, String playlistId, int offset, int limit) {
        if (findPlaylist(ownerPublicId, playlistId).isEmpty()) return List.of();
        int safeOffset = Math.max(0, offset);
        int safeLimit = Math.max(1, Math.min(500, limit));
        return tracks.getOrDefault(playlistId, List.of()).stream()
                .sorted(Comparator.comparingInt(UserPlaylistTrack::sortOrder))
                .skip(safeOffset)
                .limit(safeLimit)
                .toList();
    }

    @Override
    public synchronized Optional<UserPlaylistTrack> addTrackIfAbsent(String ownerPublicId, String playlistId, Music music) {
        if (music == null || findPlaylist(ownerPublicId, playlistId).isEmpty()) return Optional.empty();
        List<UserPlaylistTrack> list = tracks.computeIfAbsent(playlistId, key -> new ArrayList<>());
        String key = musicKey(music);
        if (list.stream().anyMatch(track -> musicKey(track.music()).equals(key))) return Optional.empty();
        long now = System.currentTimeMillis();
        UserPlaylistTrack track = new UserPlaylistTrack(UUID.randomUUID().toString(), playlistId, music, list.size(), now);
        list.add(track);
        touch(playlistId);
        return Optional.of(track);
    }

    @Override
    public synchronized boolean deleteTrack(String ownerPublicId, String playlistId, String trackId) {
        if (findPlaylist(ownerPublicId, playlistId).isEmpty()) return false;
        List<UserPlaylistTrack> list = new ArrayList<>(tracks.getOrDefault(playlistId, List.of()));
        boolean removed = list.removeIf(track -> track.id().equals(trackId));
        if (!removed) return false;
        tracks.put(playlistId, rewriteOrder(list));
        touch(playlistId);
        return true;
    }

    @Override
    public synchronized boolean deleteTrackByMusicKey(String ownerPublicId, String playlistId, String musicKey) {
        if (findPlaylist(ownerPublicId, playlistId).isEmpty()) return false;
        List<UserPlaylistTrack> list = new ArrayList<>(tracks.getOrDefault(playlistId, List.of()));
        boolean removed = list.removeIf(track -> musicKey(track.music()).equals(musicKey));
        if (!removed) return false;
        tracks.put(playlistId, rewriteOrder(list));
        touch(playlistId);
        return true;
    }

    @Override
    public synchronized void reorderTracks(String ownerPublicId, String playlistId, List<String> orderedTrackIds) {
        if (findPlaylist(ownerPublicId, playlistId).isEmpty() || orderedTrackIds == null) return;
        Map<String, UserPlaylistTrack> byId = new LinkedHashMap<>();
        for (UserPlaylistTrack track : tracks.getOrDefault(playlistId, List.of())) byId.put(track.id(), track);
        List<UserPlaylistTrack> reordered = new ArrayList<>();
        for (String id : orderedTrackIds) {
            UserPlaylistTrack track = byId.remove(id);
            if (track != null) reordered.add(track);
        }
        reordered.addAll(byId.values());
        tracks.put(playlistId, rewriteOrder(reordered));
        touch(playlistId);
    }

    private List<UserPlaylistTrack> rewriteOrder(List<UserPlaylistTrack> source) {
        List<UserPlaylistTrack> result = new ArrayList<>();
        for (int i = 0; i < source.size(); i++) {
            UserPlaylistTrack track = source.get(i);
            result.add(new UserPlaylistTrack(track.id(), track.playlistId(), track.music(), i, track.createdAt()));
        }
        return result;
    }

    private UserPlaylist toDto(MutablePlaylist playlist) {
        return new UserPlaylist(playlist.id, playlist.ownerPublicId, playlist.name, playlist.systemKey,
                tracks.getOrDefault(playlist.id, List.of()).size(), playlist.createdAt, playlist.updatedAt);
    }

    private void touch(String playlistId) {
        MutablePlaylist playlist = playlists.get(playlistId);
        if (playlist != null) playlist.updatedAt = System.currentTimeMillis();
    }

    private String musicKey(Music music) {
        return String.valueOf(music.platform()) + ":" + String.valueOf(music.id());
    }

    private static class MutablePlaylist {
        private final String id;
        private final String ownerPublicId;
        private final String systemKey;
        private String name;
        private final long createdAt;
        private long updatedAt;

        private MutablePlaylist(String id, String ownerPublicId, String name, String systemKey, long createdAt, long updatedAt) {
            this.id = id;
            this.ownerPublicId = ownerPublicId;
            this.name = name;
            this.systemKey = systemKey;
            this.createdAt = createdAt;
            this.updatedAt = updatedAt;
        }
    }
}
