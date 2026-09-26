package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestWebSocketPlaylistCommandsRoundTrip(t *testing.T) {
	server, _ := websocketTestServer(t)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	initial := readUntilType(t, connection, "player.state").Payload.(map[string]any)
	require.Contains(t, initial, "historyCursor")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	send := func(frame string) { require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(frame))) }

	send(`{"type":"playlist.create","requestId":"p1","payload":{"scope":"user","name":"我的歌单","mutationId":"pm1"}}`)
	ack := readUntilType(t, connection, "playlist.ack")
	require.Equal(t, "p1", ack.RequestID)
	created := ack.Payload.(map[string]any)
	require.Equal(t, "create", created["operation"])
	require.Equal(t, "pm1", created["mutationId"])
	playlist := created["playlist"].(map[string]any)
	require.Equal(t, "我的歌单", playlist["name"])
	playlistID := playlist["id"].(string)

	items := `[{"id":"s1","name":"Song","artists":["A"],"duration":1000,"platform":"local","coverUrl":""},{"id":"s2","name":"Song 2","artists":[],"duration":2000,"platform":"local","coverUrl":""}]`
	send(`{"type":"playlist.add-items","payload":{"scope":"user","playlistId":"` + playlistID + `","mutationId":"pm2","items":` + items + `}}`)
	added := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.Equal(t, "add-items", added["operation"])
	require.EqualValues(t, 2, added["addedCount"])
	require.EqualValues(t, 0, added["skippedCount"])

	send(`{"type":"playlist.add-items","payload":{"scope":"user","playlistId":"` + playlistID + `","mutationId":"pm3","items":[{"id":"s1","name":"Song","artists":["A"],"duration":1000,"platform":"local","coverUrl":""}]}}`)
	dedup := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.EqualValues(t, 0, dedup["addedCount"])
	require.EqualValues(t, 1, dedup["skippedCount"])

	send(`{"type":"playlist.get","payload":{"scope":"user","playlistId":"` + playlistID + `"}}`)
	data := readUntilType(t, connection, "playlist.data").Payload.(map[string]any)
	require.Equal(t, "get", data["operation"])
	require.Equal(t, playlistID, data["playlistId"])
	tracks := data["tracks"].([]any)
	require.Len(t, tracks, 2)
	trackID := tracks[0].(map[string]any)["id"].(string)

	send(`{"type":"playlist.list","payload":{"scope":"user"}}`)
	listed := readUntilType(t, connection, "playlist.data").Payload.(map[string]any)
	require.Equal(t, "list", listed["operation"])
	playlists := listed["playlists"].([]any)
	require.Len(t, playlists, 2)
	// The personal liked list is listed first and stays read-only, the same row
	// a room like writes into.
	likedList := playlists[0].(map[string]any)
	require.Equal(t, "liked-songs", likedList["systemKey"])
	require.Equal(t, "喜欢的歌曲", likedList["name"])
	require.EqualValues(t, 0, likedList["trackCount"])
	require.Equal(t, playlistID, playlists[1].(map[string]any)["id"])

	send(`{"type":"playlist.enqueue","payload":{"scope":"user","playlistId":"` + playlistID + `","mutationId":"pm4"}}`)
	enqueued := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.Equal(t, "enqueue", enqueued["operation"])
	require.EqualValues(t, 2, enqueued["count"])

	send(`{"type":"playlist.remove-items","payload":{"scope":"user","playlistId":"` + playlistID + `","trackIds":["` + trackID + `","missing"],"mutationId":"pm5"}}`)
	removed := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.EqualValues(t, 1, removed["removedCount"])

	send(`{"type":"playlist.rename","payload":{"scope":"user","playlistId":"` + playlistID + `","name":"改名","mutationId":"pm6"}}`)
	renamed := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.Equal(t, "改名", renamed["playlist"].(map[string]any)["name"])

	send(`{"type":"playlist.delete","payload":{"scope":"user","playlistId":"` + playlistID + `","mutationId":"pm7"}}`)
	require.Equal(t, "delete", readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)["operation"])
	send(`{"type":"playlist.get","payload":{"scope":"user","playlistId":"` + playlistID + `"}}`)
	gone := readUntilType(t, connection, "playlist.nack").Payload.(map[string]any)
	require.Equal(t, "PLAYLIST_NOT_FOUND", gone["reason"])

	send(`{"type":"playlist.list","payload":{"scope":"global"}}`)
	require.Equal(t, "SCOPE_INVALID", readUntilType(t, connection, "playlist.nack").Payload.(map[string]any)["reason"])
	send(`{"type":"playlist.create","payload":{"scope":"user","name":"","mutationId":"pm8"}}`)
	require.Equal(t, "PAYLOAD_INVALID", readUntilType(t, connection, "playlist.nack").Payload.(map[string]any)["reason"])
	send(`{"type":"playlist.create","payload":{"scope":"user","name":"无 mutation"}}`)
	require.Equal(t, "PAYLOAD_INVALID", readUntilType(t, connection, "playlist.nack").Payload.(map[string]any)["reason"])

	send(`{"type":"playlist.create","payload":{"scope":"room","name":"房间歌单","mutationId":"pm9"}}`)
	roomCreated := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.Equal(t, "lounge", roomCreated["playlist"].(map[string]any)["roomId"])
}

func TestWebSocketPlaylistLikedSongsAllowsRemovalOnly(t *testing.T) {
	server, _, store := websocketTestServerWithStore(t)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	readUntilType(t, connection, "player.state")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	send := func(frame string) { require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(frame))) }

	repository := storesqlite.NewUserPlaylistRepository(store, time.Now, nil)
	background := context.Background()
	playlist, err := repository.CreateSystemPlaylist(background, "member", realtime.LikedSongsName, realtime.LikedSongsSystemKey)
	require.NoError(t, err)
	added, err := repository.AddTracksIfAbsent(background, "member", playlist.ID, []storesqlite.Music{
		{ID: "keep", Name: "Keep", Artists: []string{"A"}, Duration: 1000, Platform: "local"},
		{ID: "drop", Name: "Drop", Artists: []string{"B"}, Duration: 2000, Platform: "local"},
	})
	require.NoError(t, err)
	require.Len(t, added, 2)
	dropID := added[1].ID

	send(`{"type":"playlist.remove-items","payload":{"scope":"user","playlistId":"` + playlist.ID + `","trackIds":["` + dropID + `","missing"],"mutationId":"lm1"}}`)
	removed := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.Equal(t, "remove-items", removed["operation"])
	require.EqualValues(t, 1, removed["removedCount"], "the liked list is the one system playlist a member may take rows out of")
	require.Equal(t, playlist.ID, removed["playlistId"])

	send(`{"type":"playlist.get","payload":{"scope":"user","playlistId":"` + playlist.ID + `"}}`)
	tracks := readUntilType(t, connection, "playlist.data").Payload.(map[string]any)["tracks"].([]any)
	require.Len(t, tracks, 1)
	require.Equal(t, "keep", tracks[0].(map[string]any)["music"].(map[string]any)["id"])

	send(`{"type":"playlist.remove-items","payload":{"scope":"user","playlistId":"` + playlist.ID + `","trackIds":["` + dropID + `"],"mutationId":"lm2"}}`)
	again := readUntilType(t, connection, "playlist.ack").Payload.(map[string]any)
	require.EqualValues(t, 0, again["removedCount"], "removing twice is an ack with nothing removed, not a nack")

	for _, frame := range []string{
		`{"type":"playlist.add-items","payload":{"scope":"user","playlistId":"` + playlist.ID + `","mutationId":"lm3","items":[{"id":"third","name":"Third","artists":[],"duration":1000,"platform":"local","coverUrl":""}]}}`,
		`{"type":"playlist.rename","payload":{"scope":"user","playlistId":"` + playlist.ID + `","name":"改名","mutationId":"lm4"}}`,
		`{"type":"playlist.delete","payload":{"scope":"user","playlistId":"` + playlist.ID + `","mutationId":"lm5"}}`,
	} {
		send(frame)
		nacked := readUntilType(t, connection, "playlist.nack").Payload.(map[string]any)
		require.Equal(t, "SYSTEM_READONLY", nacked["reason"], frame)
	}
}
