package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func websocketTestServer(t *testing.T) (*httptest.Server, *wsruntime.Hub) {
	t.Helper()
	store := httpStore(t)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	hub := wsruntime.NewHub(256)
	manager := realtime.NewManager(store, 100, 5*time.Second, time.Hour, hub, hub.Online)
	api := NewWebSocketAPI(cfg, store, accounts, rooms, hub, manager, NewPlatformAPI(cfg, fixturePlatform{}))
	server := httptest.NewServer(NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api))
	t.Cleanup(func() {
		server.Close()
		hub.Close()
		manager.Close()
	})
	return server, hub
}

func dialWebSocket(t *testing.T, server *httptest.Server, token, roomID string) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("Origin", server.URL)
	if token != "" {
		header.Set("Cookie", SessionCookieName+"="+token)
	}
	connection, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http")+"/ws?room-id="+roomID, &websocket.DialOptions{HTTPHeader: header})
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close(websocket.StatusNormalClosure, "") })
	return connection
}

func readUntilType(t *testing.T, connection *websocket.Conn, wanted string) wsruntime.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, data, err := connection.Read(ctx)
		require.NoError(t, err)
		var envelope wsruntime.Envelope
		require.NoError(t, json.Unmarshal(data, &envelope))
		if envelope.Type == wanted {
			return envelope
		}
	}
}

func TestWebSocketInitialStateAliasEnqueueAckAndResync(t *testing.T) {
	server, hub := websocketTestServer(t)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	state := readUntilType(t, connection, "player.state")
	require.Nil(t, state.RoomID)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"user.me","payload":{}}`)))
	user := readUntilType(t, connection, "user.me")
	userPayload := user.Payload.(map[string]any)
	require.Equal(t, "member", userPayload["publicId"])
	require.Equal(t, "member-token", userPayload["sessionToken"])
	require.Eventually(t, func() bool { return hub.Count() == 1 }, time.Second, 10*time.Millisecond)

	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"/enqueue","requestId":"r1","roomId":"lounge","payload":{"platform":"local","musicId":"track","mutationId":"m1"}}`)))
	ack := readUntilType(t, connection, "enqueue.ack")
	require.Equal(t, "m1", ack.Payload.(map[string]any)["mutationId"])
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"player.resync","requestId":"resync","roomId":"lounge","payload":{}}`)))
	resync := readUntilType(t, connection, "player.state")
	require.Nil(t, resync.RequestID)
	require.Equal(t, "lounge", resync.RoomID)
	nowPlaying := resync.Payload.(map[string]any)["nowPlaying"].(map[string]any)
	music := nowPlaying["music"].(map[string]any)
	require.Equal(t, "Resolved Track", music["name"])
	require.Equal(t, []any{"Resolved Artist"}, music["artists"])
	require.Equal(t, float64(60_000), music["duration"])
	require.Equal(t, "/api/local/media/track", music["url"])
}

func TestWebSocketRejectsUnknownSessionWithPolicyViolation(t *testing.T) {
	server, _ := websocketTestServer(t)
	connection := dialWebSocket(t, server, "", "lounge")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := connection.Read(ctx)
	require.Error(t, err)
	require.Equal(t, websocket.StatusPolicyViolation, websocket.CloseStatus(err))
}
