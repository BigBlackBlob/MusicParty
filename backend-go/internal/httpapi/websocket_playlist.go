package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
)

const (
	playlistMaxItems  = 200
	playlistMaxTracks = 500
	playlistMaxName   = 64
)

type playlistCommandPayload struct {
	Scope      string          `json:"scope"`
	PlaylistID string          `json:"playlistId"`
	Name       string          `json:"name"`
	Offset     *int            `json:"offset"`
	Limit      *int            `json:"limit"`
	MutationID string          `json:"mutationId"`
	Items      json.RawMessage `json:"items"`
	TrackIDs   []string        `json:"trackIds"`
}

var playlistWriteOperations = map[string]struct{}{
	"playlist.create": {}, "playlist.rename": {}, "playlist.delete": {},
	"playlist.add-items": {}, "playlist.remove-items": {}, "playlist.enqueue": {},
}

// playlistCommand answers the desktop WebSocket playlist surface. Scopes are
// "room" (owner = the attached room, membership proven at connect time) and
// "user" (owner = the session public ID); playlist IDs and mutation IDs are
// correlation data only, never an ownership claim.
func (api *WebSocketAPI) playlistCommand(ctx context.Context, client *wsruntime.Client, runtime *realtime.RoomRuntime, envelope inboundEnvelope, kind string, session account.Session) {
	request, decodeErr := decodePayload[playlistCommandPayload](envelope.Payload)
	respond := func(typ string, payload map[string]any) {
		value := wsruntime.Envelope{Type: typ, RoomID: client.RoomID, Payload: payload}
		if envelope.RequestID != "" {
			value.RequestID = envelope.RequestID
		}
		_ = client.Direct(value)
	}
	nack := func(reason string) {
		payload := map[string]any{"operation": strings.TrimPrefix(kind, "playlist."), "reason": reason}
		if request.MutationID != "" {
			payload["mutationId"] = request.MutationID
		}
		respond("playlist.nack", payload)
	}
	if decodeErr != nil {
		nack("PAYLOAD_INVALID")
		return
	}
	owner, roomID := session.PublicID, client.RoomID
	switch request.Scope {
	case "user":
	case "room":
		if roomID == "" {
			nack("ROOM_REQUIRED")
			return
		}
	default:
		nack("SCOPE_INVALID")
		return
	}
	if _, write := playlistWriteOperations[kind]; write && (request.MutationID == "" || len(request.MutationID) > 128) {
		nack("PAYLOAD_INVALID")
		return
	}
	operation := strings.TrimPrefix(kind, "playlist.")
	find := func() (*storesqlite.Playlist, error) {
		if request.Scope == "user" {
			return api.userPlaylists.FindPlaylist(ctx, owner, request.PlaylistID)
		}
		return api.roomPlaylists.FindPlaylist(ctx, roomID, request.PlaylistID)
	}
	writable := func(playlist *storesqlite.Playlist) bool { return playlist.SystemKey == nil }
	view := func(playlist storesqlite.Playlist) playlistView {
		if request.Scope == "user" {
			return userPlaylist(playlist)
		}
		return roomPlaylist(playlist)
	}
	tracks := func(offset, limit int) ([]storesqlite.PlaylistTrack, error) {
		if request.Scope == "user" {
			return api.userPlaylists.ListTracks(ctx, owner, request.PlaylistID, offset, limit)
		}
		return api.roomPlaylists.ListTracks(ctx, roomID, request.PlaylistID, offset, limit)
	}
	notFound := func(err error) bool { return errors.Is(err, sql.ErrNoRows) }

	switch kind {
	case "playlist.list":
		var values []storesqlite.Playlist
		var err error
		if request.Scope == "user" {
			values, err = api.userPlaylists.ListPlaylists(ctx, owner)
		} else {
			values, err = api.roomPlaylists.ListPlaylists(ctx, roomID)
		}
		if err != nil {
			nack("PERSISTENCE_FAILED")
			return
		}
		playlists := make([]playlistView, 0, len(values))
		for _, value := range values {
			playlists = append(playlists, view(value))
		}
		respond("playlist.data", map[string]any{"operation": "list", "scope": request.Scope, "playlists": playlists})
	case "playlist.get":
		playlist, err := find()
		if err != nil {
			nack(nackForPlaylistError(err, notFound))
			return
		}
		values, err := tracks(playlistOffset(request.Offset), playlistLimit(request.Limit))
		if err != nil {
			nack("PERSISTENCE_FAILED")
			return
		}
		items := make([]trackView, 0, len(values))
		for _, value := range values {
			items = append(items, track(value))
		}
		respond("playlist.data", map[string]any{"operation": "get", "scope": request.Scope, "playlistId": playlist.ID, "tracks": items})
	case "playlist.create":
		name, ok := playlistName(request.Name)
		if !ok {
			nack("PAYLOAD_INVALID")
			return
		}
		var playlist storesqlite.Playlist
		var err error
		if request.Scope == "user" {
			playlist, err = api.userPlaylists.CreatePlaylist(ctx, owner, name)
		} else {
			playlist, err = api.roomPlaylists.CreatePlaylist(ctx, roomID, name)
		}
		if err != nil {
			nack("PERSISTENCE_FAILED")
			return
		}
		respond("playlist.ack", map[string]any{"operation": operation, "scope": request.Scope, "mutationId": request.MutationID, "playlist": view(playlist)})
	case "playlist.rename":
		name, ok := playlistName(request.Name)
		if !ok {
			nack("PAYLOAD_INVALID")
			return
		}
		playlist, err := find()
		if err != nil {
			nack(nackForPlaylistError(err, notFound))
			return
		}
		if !writable(playlist) {
			nack("SYSTEM_READONLY")
			return
		}
		if request.Scope == "user" {
			playlist, err = api.userPlaylists.RenamePlaylist(ctx, owner, playlist.ID, name)
		} else {
			playlist, err = api.roomPlaylists.RenamePlaylist(ctx, roomID, playlist.ID, name)
		}
		if err != nil {
			nack("PERSISTENCE_FAILED")
			return
		}
		if playlist == nil {
			nack("PLAYLIST_NOT_FOUND")
			return
		}
		respond("playlist.ack", map[string]any{"operation": operation, "scope": request.Scope, "mutationId": request.MutationID, "playlist": view(*playlist)})
	case "playlist.delete":
		playlist, err := find()
		if err != nil {
			nack(nackForPlaylistError(err, notFound))
			return
		}
		if !writable(playlist) {
			nack("SYSTEM_READONLY")
			return
		}
		var deleted bool
		if request.Scope == "user" {
			deleted, err = api.userPlaylists.DeletePlaylist(ctx, owner, playlist.ID)
		} else {
			deleted, err = api.roomPlaylists.DeletePlaylist(ctx, roomID, playlist.ID)
		}
		if err != nil {
			nack("PERSISTENCE_FAILED")
			return
		}
		if !deleted {
			nack("PLAYLIST_NOT_FOUND")
			return
		}
		respond("playlist.ack", map[string]any{"operation": operation, "scope": request.Scope, "mutationId": request.MutationID})
	case "playlist.add-items":
		playlist, err := find()
		if err != nil {
			nack(nackForPlaylistError(err, notFound))
			return
		}
		if !writable(playlist) {
			nack("SYSTEM_READONLY")
			return
		}
		var musics []storesqlite.Music
		if err := json.Unmarshal(request.Items, &musics); err != nil || len(musics) == 0 || len(musics) > playlistMaxItems {
			nack("PAYLOAD_INVALID")
			return
		}
		for _, music := range musics {
			if !validPlaylistMusic(music) {
				nack("PAYLOAD_INVALID")
				return
			}
		}
		var added []storesqlite.PlaylistTrack
		if request.Scope == "user" {
			added, err = api.userPlaylists.AddTracksIfAbsent(ctx, owner, playlist.ID, musics)
		} else {
			added, err = api.roomPlaylists.AddTracks(ctx, roomID, playlist.ID, musics)
		}
		if err != nil {
			nack("PERSISTENCE_FAILED")
			return
		}
		respond("playlist.ack", map[string]any{"operation": operation, "scope": request.Scope, "mutationId": request.MutationID, "playlistId": playlist.ID, "addedCount": len(added), "skippedCount": max(0, len(musics)-len(added))})
	case "playlist.remove-items":
		playlist, err := find()
		if err != nil {
			nack(nackForPlaylistError(err, notFound))
			return
		}
		if !writable(playlist) {
			nack("SYSTEM_READONLY")
			return
		}
		if len(request.TrackIDs) == 0 || len(request.TrackIDs) > playlistMaxItems {
			nack("PAYLOAD_INVALID")
			return
		}
		removed := 0
		for _, trackID := range request.TrackIDs {
			if trackID == "" || len(trackID) > 128 {
				nack("PAYLOAD_INVALID")
				return
			}
			var changed bool
			if request.Scope == "user" {
				changed, err = api.userPlaylists.DeleteTrack(ctx, owner, playlist.ID, trackID)
			} else {
				changed, err = api.roomPlaylists.DeleteTrack(ctx, roomID, playlist.ID, trackID)
			}
			if err != nil {
				nack("PERSISTENCE_FAILED")
				return
			}
			if changed {
				removed++
			}
		}
		respond("playlist.ack", map[string]any{"operation": operation, "scope": request.Scope, "mutationId": request.MutationID, "playlistId": playlist.ID, "removedCount": removed})
	case "playlist.enqueue":
		playlist, err := find()
		if err != nil {
			nack(nackForPlaylistError(err, notFound))
			return
		}
		values, err := tracks(0, playlistMaxTracks)
		if err != nil || len(values) == 0 {
			nack("PERSISTENCE_FAILED")
			return
		}
		musics := make([]storesqlite.Music, len(values))
		for index := range values {
			musics[index] = values[index].Music
		}
		items, err := runtime.EnqueueMany(ctx, session, musics, request.MutationID)
		if err != nil {
			if errors.Is(err, realtime.ErrDuplicate) {
				nack("DUPLICATE")
			} else {
				nack("PERSISTENCE_FAILED")
			}
			return
		}
		respond("playlist.ack", map[string]any{"operation": operation, "scope": request.Scope, "mutationId": request.MutationID, "playlistId": playlist.ID, "count": len(items)})
	}
}

func nackForPlaylistError(err error, notFound func(error) bool) string {
	if notFound(err) {
		return "PLAYLIST_NOT_FOUND"
	}
	return "PERSISTENCE_FAILED"
}

func playlistName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	return value, value != "" && len([]rune(value)) <= playlistMaxName
}

func playlistOffset(value *int) int {
	if value == nil {
		return 0
	}
	return max(0, *value)
}

func playlistLimit(value *int) int {
	if value == nil || *value <= 0 {
		return 100
	}
	return min(playlistMaxTracks, *value)
}

func validPlaylistMusic(music storesqlite.Music) bool {
	bounded := func(value string, min, max int) bool {
		length := len([]rune(value))
		return min <= length && length <= max
	}
	if !bounded(music.ID, 1, 128) || !bounded(music.Platform, 1, 64) || !bounded(music.Name, 1, 256) || !bounded(music.CoverURL, 0, 512) {
		return false
	}
	if music.Duration < 0 || music.Duration > 86_400_000 || len(music.Artists) > 8 {
		return false
	}
	for _, artist := range music.Artists {
		if !bounded(artist, 0, 128) {
			return false
		}
	}
	return strings.TrimSpace(music.ID) != "" && strings.TrimSpace(music.Name) != ""
}
