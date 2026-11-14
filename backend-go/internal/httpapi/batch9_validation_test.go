package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

type batch9Platform struct {
	fixturePlatform
	calls atomic.Int32
}

func (f *batch9Platform) Search(context.Context, string, int, int) ([]platform.Music, error) {
	f.calls.Add(1)
	return []platform.Music{}, nil
}
func (f *batch9Platform) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	f.calls.Add(1)
	return []platform.Music{}, nil
}

func TestBatch9PlatformInputRejectedBeforeProvider(t *testing.T) {
	cfg := testConfig(t)
	provider := &batch9Platform{}
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewPlatformAPI(cfg, provider))
	for _, path := range []string{"/api/search/local/song", "/api/playlist/songs/local/list"} {
		for _, query := range []string{"offset=-1", "offset=10001", "offset=oops", "offset=9223372036854775808", "limit=-1", "limit=0", "limit=501", "limit=nope"} {
			t.Run(path+query, func(t *testing.T) {
				before := provider.calls.Load()
				response := batch2Request(handler, http.MethodGet, path+"?"+query, "", "", "")
				require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
				require.JSONEq(t, `{"status":400,"code":"bad-request","message":"Invalid pagination parameters"}`, response.Body.String())
				require.Equal(t, before, provider.calls.Load())
			})
		}
	}
	for _, path := range []string{"/api/search/local/" + strings.Repeat("x", 129), "/api/search/local/song?limit=101"} {
		response := batch2Request(handler, http.MethodGet, path, "", "", "")
		require.Equal(t, http.StatusBadRequest, response.Code)
	}
	for _, path := range []string{"/api/search/local/song?offset=10000&limit=100", "/api/playlist/songs/local/list?offset=10000&limit=500"} {
		response := batch2Request(handler, http.MethodGet, path, "", "", "")
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	}
}

func TestBatch9RESTAndWSRejectInvalidMusicWithoutPartialWrites(t *testing.T) {
	server, _, store := websocketTestServerWithStore(t)
	accounts := account.New(store)
	cfg := testConfig(t)
	api := NewPlaylistAPI(cfg, store, accounts, roomdomain.New(store, accounts), NewPlatformAPI(cfg))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	users := storesqlite.NewUserPlaylistRepository(store, time.Now, nil)
	rooms := storesqlite.NewRoomPlaylistRepository(store, time.Now, nil)
	personal, err := users.CreatePlaylist(context.Background(), "member", "Personal")
	require.NoError(t, err)
	room, err := rooms.CreatePlaylist(context.Background(), "lounge", "Room")
	require.NoError(t, err)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	readUntilType(t, connection, "player.state")
	valid := storesqlite.Music{ID: "valid", Name: "Song", Platform: "local", Artists: []string{"Artist"}, Duration: 1000}
	invalid := []storesqlite.Music{{}, valid, valid, valid, valid, valid, valid, valid}
	invalid[1].Name = strings.Repeat("字", 257)
	invalid[2].Duration = -1
	invalid[3].CoverURL = strings.Repeat("x", 513)
	invalid[4].Artists = []string{strings.Repeat("x", 129)}
	invalid[5].Artists = make([]string, 9)
	invalid[6].ID = strings.Repeat("x", 129)
	invalid[7].Platform = strings.Repeat("x", 65)
	for _, music := range invalid {
		batch := []storesqlite.Music{valid, music}
		body, err := json.Marshal(map[string]any{"musics": batch})
		require.NoError(t, err)
		response := batch2Request(handler, http.MethodPost, "/api/me/playlists/"+personal.ID+"/tracks/batch", string(body), "member-token", "")
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		body, err = json.Marshal(map[string]any{"music": music})
		require.NoError(t, err)
		response = batch2Request(handler, http.MethodPost, "/api/rooms/lounge/playlists/"+room.ID+"/tracks", string(body), "member-token", "")
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		frame, err := json.Marshal(map[string]any{"type": "playlist.add-items", "payload": map[string]any{"scope": "user", "playlistId": personal.ID, "mutationId": "invalid", "items": batch}})
		require.NoError(t, err)
		require.NoError(t, connection.Write(context.Background(), websocket.MessageText, frame))
		require.Equal(t, "PAYLOAD_INVALID", readUntilType(t, connection, "playlist.nack").Payload.(map[string]any)["reason"])
	}
	for _, size := range []int{0, 201} {
		musics := make([]storesqlite.Music, size)
		for i := range musics {
			musics[i] = valid
		}
		body, err := json.Marshal(map[string]any{"musics": musics})
		require.NoError(t, err)
		response := batch2Request(handler, http.MethodPost, "/api/me/playlists/"+personal.ID+"/tracks/batch", string(body), "member-token", "")
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		frame, err := json.Marshal(map[string]any{"type": "playlist.add-items", "payload": map[string]any{"scope": "user", "playlistId": personal.ID, "mutationId": "oversize", "items": musics}})
		require.NoError(t, err)
		require.NoError(t, connection.Write(context.Background(), websocket.MessageText, frame))
		require.Equal(t, "PAYLOAD_INVALID", readUntilType(t, connection, "playlist.nack").Payload.(map[string]any)["reason"])
	}
	for _, body := range []string{`null`, `[]`, `{"musics":null}`, `{"musics":[{"id":"s","name":"Song","platform":"local","duration":"1"}]}`, `{"musics":[`} {
		response := batch2Request(handler, http.MethodPost, "/api/me/playlists/"+personal.ID+"/tracks/batch", body, "member-token", "")
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
	}
	response := batch2Request(handler, http.MethodPut, "/api/me/liked-songs/local/song", `{}`, "member-token", "")
	require.Equal(t, http.StatusBadRequest, response.Code)
	tracks, err := users.ListTracks(context.Background(), "member", personal.ID, 0, 500)
	require.NoError(t, err)
	require.Empty(t, tracks)
	tracks, err = rooms.ListTracks(context.Background(), "lounge", room.ID, 0, 500)
	require.NoError(t, err)
	require.Empty(t, tracks)
	// The exact batch and name boundaries are accepted, including multibyte names.
	musics := make([]storesqlite.Music, 200)
	for i := range musics {
		musics[i] = valid
		musics[i].ID = strconv.Itoa(i)
		musics[i].Name = strings.Repeat("字", 256)
	}
	body, err := json.Marshal(map[string]any{"musics": musics})
	require.NoError(t, err)
	response = batch2Request(handler, http.MethodPost, "/api/me/playlists/"+personal.ID+"/tracks/batch", string(body), "member-token", "")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	tracks, err = users.ListTracks(context.Background(), "member", personal.ID, 0, 500)
	require.NoError(t, err)
	require.Len(t, tracks, 200)
}
