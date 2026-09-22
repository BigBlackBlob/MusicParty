package httpapi

import (
	"context"
	"testing"
	"time"

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
	require.Len(t, listed["playlists"].([]any), 1)

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
