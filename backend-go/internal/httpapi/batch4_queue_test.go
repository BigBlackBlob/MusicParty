package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestBatch4InvalidPingDoesNotPanic(t *testing.T) {
	store := httpStore(t)
	require.NoError(t, storesqlite.NewRoomRepository(store).Upsert(context.Background(), storesqlite.Room{ID: "lounge", Name: "Lounge", Visibility: "PUBLIC"}))
	manager := realtime.NewManager(store, 10, time.Second, time.Hour, wsruntime.NewHub(64))
	defer manager.Close()
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	insertStage5User(t, store, "user-token", "user", "MEMBER")
	api := &WebSocketAPI{accounts: account.New(store)}
	client := wsruntime.NewHub(64).Register(nil, "lounge", account.Session{PublicID: "user", SessionToken: "user-token"})
	for _, payload := range []string{"null", "[]", "1", "{}", `{"pingId":"p","clientSendTime":"bad"}`} {
		require.NotPanics(t, func() {
			api.dispatch(context.Background(), client, runtime, inboundEnvelope{Type: "sync.ping", Payload: json.RawMessage(payload)})
		}, payload)
	}
}

func TestBatch4HTTPAndWebSocketShareCapacityAndRebuiltSnapshots(t *testing.T) {
	ctx := context.Background()
	store := httpStore(t)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	hub := wsruntime.NewHub(256)
	manager := realtime.NewManager(store, 100, time.Second, time.Hour, hub, realtime.QueueLimits{MaxSize: 1, MaxUserSongs: 1, HistorySize: 2})
	defer manager.Close()
	defer hub.Close()
	platforms := NewPlatformAPI(cfg, fixturePlatform{})
	playlists := NewPlaylistAPI(cfg, store, accounts, rooms, platforms)
	playlists.SetRealtime(manager)
	playlist, err := playlists.users.CreatePlaylist(ctx, "member", "Import")
	require.NoError(t, err)
	_, err = playlists.users.AddTracksIfAbsent(ctx, "member", playlist.ID, []storesqlite.Music{{ID: "a", Platform: "local"}, {ID: "b", Platform: "local"}, {ID: "c", Platform: "local"}})
	require.NoError(t, err)
	ws := NewWebSocketAPI(cfg, store, accounts, rooms, hub, manager, platforms)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), playlists, ws)
	response := batch2Request(handler, http.MethodPost, "/api/me/playlists/"+playlist.ID+"/enqueue", "{}", "member-token", "")
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "QUEUE_CAPACITY")
	server := httptest.NewServer(handler)
	defer server.Close()
	connection := dialWebSocket(t, server, "member-token", "lounge")
	defer connection.CloseNow()
	initial := readUntilType(t, connection, "player.state").Payload.(map[string]any)
	require.Empty(t, initial["queue"], "failed HTTP batch must leave no partial queue")
	for _, id := range []string{"first", "pending"} {
		require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"enqueue","payload":{"platform":"local","musicId":"`+id+`","mutationId":"`+id+`"}}`)))
		readUntilType(t, connection, "enqueue.ack")
	}
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"enqueue","payload":{"platform":"local","musicId":"overflow","mutationId":"overflow"}}`)))
	nack := readUntilType(t, connection, "enqueue.nack")
	require.Equal(t, "QUEUE_CAPACITY", nack.Payload.(map[string]any)["reason"])
	require.NoError(t, connection.CloseNow())
	require.Eventually(t, func() bool { return hub.Count() == 0 }, time.Second, time.Millisecond)
	manager.CloseRoom("lounge")
	reconnected := dialWebSocket(t, server, "member-token", "lounge")
	defer reconnected.CloseNow()
	restored := readUntilType(t, reconnected, "player.state").Payload.(map[string]any)
	require.NotEqual(t, initial["queueEpoch"], restored["queueEpoch"])
	require.Greater(t, restored["queueVersion"].(float64), float64(0))
	require.Len(t, restored["queue"], 1)
	require.NoError(t, reconnected.Write(ctx, websocket.MessageText, []byte(`{"type":"player.resync","payload":{}}`)))
	resynced := readUntilType(t, reconnected, "player.state").Payload.(map[string]any)
	require.Equal(t, restored["queueEpoch"], resynced["queueEpoch"])
	require.Equal(t, restored["queue"], resynced["queue"])
	require.Equal(t, restored["queueVersion"], resynced["queueVersion"])
}

func TestBatch4WebSocketRejectsMalformedMutationsAndReplaysAck(t *testing.T) {
	server, hub, store := websocketTestServerWithStore(t)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	defer connection.CloseNow()
	initial := readUntilType(t, connection, "player.state").Payload.(map[string]any)
	require.NotEmpty(t, initial["queueEpoch"])
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, payload := range []string{"null", "[]", `{"pingId":"p"}`} {
		require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"sync.ping","payload":`+payload+`}`)))
		event := readUntilType(t, connection, "player.events")
		require.Equal(t, "PAYLOAD_INVALID", event.Payload.(map[string]any)["code"])
	}
	for _, kind := range []string{"enqueue", "enqueue.playlist", "enqueue.album", "enqueue.room-playlist", "queue.top", "queue.remove", "queue.batch-top", "queue.batch-remove", "queue.reorder", "queue.clear"} {
		require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"`+kind+`","payload":{}}`)))
		wanted := "queue.mutation.nack"
		if kind == "enqueue" || kind == "enqueue.playlist" || kind == "enqueue.album" || kind == "enqueue.room-playlist" {
			wanted = "enqueue.nack"
		}
		if kind == "queue.reorder" {
			wanted = "queue.reorder.nack"
		}
		nack := readUntilType(t, connection, wanted)
		require.Equal(t, "PAYLOAD_INVALID", nack.Payload.(map[string]any)["reason"])
	}
	command := []byte(`{"type":"enqueue","payload":{"platform":"local","musicId":"track","mutationId":"once"}}`)
	require.NoError(t, connection.Write(ctx, websocket.MessageText, command))
	first := readUntilType(t, connection, "enqueue.ack")
	require.NoError(t, connection.Write(ctx, websocket.MessageText, command))
	second := readUntilType(t, connection, "enqueue.ack")
	require.Equal(t, first.Payload, second.Payload)
	var count int
	require.NoError(t, store.Reader().QueryRow("select count(*) from room_queue where room_id='lounge'").Scan(&count))
	require.Zero(t, count, "retry must not enqueue a second copy of the playing track")
	require.NoError(t, connection.CloseNow())
	require.Eventually(t, func() bool { return hub.Count() == 0 }, time.Second, time.Millisecond)
}
