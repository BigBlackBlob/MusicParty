package org.thornex.musicparty.persistence;

import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.RoomPlaylist;
import org.thornex.musicparty.dto.RoomPlaylistTrack;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryRoomPlaylistRepository implements RoomPlaylistRepository {
    private final Map<String, StoredPlaylist> playlists = new ConcurrentHashMap<>();
    private final Map<String, List<RoomPlaylistTrack>> tracks = new ConcurrentHashMap<>();

    @Override
    public synchronized List<RoomPlaylist> listPlaylists(String roomId) {
        return playlists.values().stream()
                .filter(playlist -> playlist.roomId.equals(roomId))
                .sorted(Comparator.comparingLong(StoredPlaylist::createdAt))
                .map(this::toDto)
                .toList();
    }

    @Override
    public synchronized Optional<RoomPlaylist> findPlaylist(String roomId, String playlistId) {
        StoredPlaylist playlist = playlists.get(playlistId);
        if (playlist == null || !playlist.roomId.equals(roomId)) return Optional.empty();
        return Optional.of(toDto(playlist));
    }

    @Override
    public synchronized RoomPlaylist createPlaylist(String roomId, String name) {
        long now = System.currentTimeMillis();
        StoredPlaylist playlist = new StoredPlaylist(UUID.randomUUID().toString(), roomId, name, now, now);
        playlists.put(playlist.id, playlist);
        tracks.put(playlist.id, new ArrayList<>());
        return toDto(playlist);
    }

    @Override
    public synchronized Optional<RoomPlaylist> renamePlaylist(String roomId, String playlistId, String name) {
        StoredPlaylist current = playlists.get(playlistId);
        if (current == null || !current.roomId.equals(roomId)) return Optional.empty();
        StoredPlaylist renamed = new StoredPlaylist(current.id, current.roomId, name, current.createdAt, System.currentTimeMillis());
        playlists.put(playlistId, renamed);
        return Optional.of(toDto(renamed));
    }

    @Override
    public synchronized boolean deletePlaylist(String roomId, String playlistId) {
        StoredPlaylist current = playlists.get(playlistId);
        if (current == null || !current.roomId.equals(roomId)) return false;
        playlists.remove(playlistId);
        tracks.remove(playlistId);
        return true;
    }

    @Override
    public synchronized List<RoomPlaylistTrack> listTracks(String roomId, String playlistId, int offset, int limit) {
        if (findPlaylist(roomId, playlistId).isEmpty()) return List.of();
        int safeOffset = Math.max(0, offset);
        int safeLimit = Math.max(1, Math.min(500, limit));
        return tracks.getOrDefault(playlistId, List.of()).stream()
                .sorted(Comparator.comparingInt(RoomPlaylistTrack::sortOrder))
                .skip(safeOffset)
                .limit(safeLimit)
                .toList();
    }

    @Override
    public synchronized Optional<RoomPlaylistTrack> addTrack(String roomId, String playlistId, Music music) {
        if (music == null || findPlaylist(roomId, playlistId).isEmpty()) return Optional.empty();
        List<RoomPlaylistTrack> list = tracks.computeIfAbsent(playlistId, ignored -> new ArrayList<>());
        RoomPlaylistTrack track = new RoomPlaylistTrack(UUID.randomUUID().toString(), playlistId, music, list.size(), System.currentTimeMillis());
        list.add(track);
        touch(playlistId);
        return Optional.of(track);
    }

    @Override
    public synchronized boolean deleteTrack(String roomId, String playlistId, String trackId) {
        if (findPlaylist(roomId, playlistId).isEmpty()) return false;
        List<RoomPlaylistTrack> list = tracks.getOrDefault(playlistId, new ArrayList<>());
        boolean removed = list.removeIf(track -> track.id().equals(trackId));
        if (removed) {
            rewriteOrder(playlistId, list);
            touch(playlistId);
        }
        return removed;
    }

    @Override
    public synchronized void reorderTracks(String roomId, String playlistId, List<String> orderedTrackIds) {
        if (findPlaylist(roomId, playlistId).isEmpty() || orderedTrackIds == null) return;
        Map<String, RoomPlaylistTrack> byId = new LinkedHashMap<>();
        tracks.getOrDefault(playlistId, List.of()).forEach(track -> byId.put(track.id(), track));
        List<RoomPlaylistTrack> next = new ArrayList<>();
        for (String id : orderedTrackIds) {
            RoomPlaylistTrack track = byId.remove(id);
            if (track != null) next.add(track);
        }
        next.addAll(byId.values());
        rewriteOrder(playlistId, next);
        touch(playlistId);
    }

    @Override
    public synchronized void deleteRoomData(String roomId) {
        List<String> ids = playlists.values().stream()
                .filter(playlist -> playlist.roomId.equals(roomId))
                .map(StoredPlaylist::id)
                .toList();
        ids.forEach(id -> {
            playlists.remove(id);
            tracks.remove(id);
        });
    }

    private RoomPlaylist toDto(StoredPlaylist playlist) {
        return new RoomPlaylist(playlist.id, playlist.roomId, playlist.name,
                tracks.getOrDefault(playlist.id, List.of()).size(), playlist.createdAt, playlist.updatedAt);
    }

    private void rewriteOrder(String playlistId, List<RoomPlaylistTrack> list) {
        List<RoomPlaylistTrack> ordered = new ArrayList<>();
        for (int i = 0; i < list.size(); i++) {
            RoomPlaylistTrack track = list.get(i);
            ordered.add(new RoomPlaylistTrack(track.id(), track.playlistId(), track.music(), i, track.createdAt()));
        }
        tracks.put(playlistId, ordered);
    }

    private void touch(String playlistId) {
        StoredPlaylist current = playlists.get(playlistId);
        if (current != null) {
            playlists.put(playlistId, new StoredPlaylist(current.id, current.roomId, current.name, current.createdAt, System.currentTimeMillis()));
        }
    }

    private record StoredPlaylist(String id, String roomId, String name, long createdAt, long updatedAt) {}
}
