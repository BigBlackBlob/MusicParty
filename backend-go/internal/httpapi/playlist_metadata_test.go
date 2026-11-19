package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/stretchr/testify/require"
)

func TestPlaylistDescriptionUpdates(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	cfg := testConfig(t)
	insertStage5User(t, store, "owner-token", "owner", "MEMBER")
	insertStage5User(t, store, "other-token", "other", "MEMBER")
	insertStage5Room(t, store, "private-room", "PRIVATE", 3)
	insertStage5Room(t, store, "other-room", "PUBLIC", 0)
	api := NewPlaylistAPI(cfg, store, accounts, roomdomain.New(store, accounts), NewPlatformAPI(cfg))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	personal, err := api.users.CreatePlaylist(t.Context(), "owner", "Personal")
	require.NoError(t, err)
	shared, err := api.roomPlaylists.CreatePlaylist(t.Context(), "private-room", "Shared")
	require.NoError(t, err)
	proof := signRoomAccessToken(api.roomSecret, "private-room", "other", time.Now().Add(time.Minute).UnixMilli(), 3)

	for _, tc := range []struct {
		name, listPath, playlistID, token, proof, deniedToken string
		deniedStatus                                          int
	}{
		{"personal", "/api/me/playlists", personal.ID, "owner-token", "", "other-token", http.StatusNotFound},
		{"room", "/api/rooms/private-room/playlists", shared.ID, "other-token", proof, "other-token", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := func(method, path, body, token, proof string, status int) []byte {
				t.Helper()
				response := batch2Request(handler, method, path, body, token, proof)
				require.Equal(t, status, response.Code, response.Body.String())
				return response.Body.Bytes()
			}
			path := tc.listPath + "/" + tc.playlistID + "/description"
			description := "<b>Mix</b>\n" + strings.Repeat("\U0001f3b5", 489)
			body, err := json.Marshal(map[string]string{"description": description})
			require.NoError(t, err)
			request(http.MethodPatch, path, string(body), tc.token, tc.proof, http.StatusOK)
			request(http.MethodPatch, path, `{"description":""}`, tc.deniedToken, "", tc.deniedStatus)
			tooLong, err := json.Marshal(map[string]string{"description": description + "!"})
			require.NoError(t, err)
			request(http.MethodPatch, path, string(tooLong), tc.token, tc.proof, http.StatusBadRequest)
			request(http.MethodPatch, path, `{"description":null}`, tc.token, tc.proof, http.StatusBadRequest)
			readDescription := func() string {
				var playlists []playlistView
				require.NoError(t, json.Unmarshal(request(http.MethodGet, tc.listPath, "", tc.token, tc.proof, http.StatusOK), &playlists))
				for _, playlist := range playlists {
					if playlist.ID == tc.playlistID {
						return playlist.Description
					}
				}
				t.Fatalf("playlist %s missing from list", tc.playlistID)
				return ""
			}
			require.Equal(t, description, readDescription())
			request(http.MethodPatch, path, `{"description":""}`, tc.token, tc.proof, http.StatusOK)
			require.Empty(t, readDescription())
		})
	}
	wrongRoom := batch2Request(handler, http.MethodPatch, "/api/rooms/other-room/playlists/"+shared.ID+"/description", `{"description":"Wrong room"}`, "owner-token", "")
	require.Equal(t, http.StatusNotFound, wrongRoom.Code, wrongRoom.Body.String())
}
