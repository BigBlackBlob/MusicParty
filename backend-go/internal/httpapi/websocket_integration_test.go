package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func websocketTestServer(t *testing.T) (*httptest.Server, *wsruntime.Hub) {
	t.Helper()
	server, hub, _ := websocketTestServerWithStore(t)
	return server, hub
}

func websocketTestServerWithStore(t *testing.T) (*httptest.Server, *wsruntime.Hub, *storesqlite.Store) {
	t.Helper()
	store := httpStore(t)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	hub := wsruntime.NewHub(256)
	manager := realtime.NewManager(store, 100, 5*time.Second, time.Hour, hub)
	api := NewWebSocketAPI(cfg, store, accounts, rooms, hub, manager, NewPlatformAPI(cfg, fixturePlatform{}))
	server := httptest.NewServer(NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api))
	t.Cleanup(func() {
		server.Close()
		hub.Close()
		manager.Close()
	})
	return server, hub, store
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
	_, hasLegacyOnlineUsers := state.Payload.(map[string]any)["onlineUsers"]
	require.False(t, hasLegacyOnlineUsers)
	initialPresence := readUntilType(t, connection, "users.online")
	require.Equal(t, "lounge", initialPresence.RoomID)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"user.me","payload":{}}`)))
	user := readUntilType(t, connection, "user.me")
	userPayload := user.Payload.(map[string]any)
	require.Equal(t, "member", userPayload["publicId"])
	_, leaksSessionToken := userPayload["sessionToken"]
	require.False(t, leaksSessionToken)
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
	resyncPresence := readUntilType(t, connection, "users.online")
	require.Equal(t, "lounge", resyncPresence.RoomID)
	require.Equal(t, "lounge", resyncPresence.Payload.(map[string]any)["roomId"])
}

func TestWebSocketJoinBroadcastsVersionedPresence(t *testing.T) {
	server, _ := websocketTestServer(t)
	first := dialWebSocket(t, server, "member-token", "lounge")
	_ = readUntilType(t, first, "player.state")

	second := dialWebSocket(t, server, "member-token", "lounge")
	presence := readUntilType(t, second, "users.online")
	payload := presence.Payload.(map[string]any)
	require.Equal(t, "lounge", payload["roomId"])
	require.Equal(t, float64(2), payload["revision"])
	users := payload["users"].([]any)
	require.Len(t, users, 1, "multiple tabs for one account must remain one online user")
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

func TestWebSocketControlMutationReplay(t *testing.T) {
	server, _ := websocketTestServer(t)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	state := readUntilType(t, connection, "player.state").Payload.(map[string]any)
	scope := state["idempotencyScopeId"].(string)
	require.NotEmpty(t, scope)
	require.Equal(t, float64(60000), state["idempotencyTtlMs"])
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	send := func(kind, requestID, payload string) map[string]any {
		t.Helper()
		require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"`+kind+`","requestId":"`+requestID+`","payload":`+payload+`}`)))
		ack := readUntilType(t, connection, "control.ack")
		require.Equal(t, requestID, ack.RequestID)
		return ack.Payload.(map[string]any)
	}
	payload := `{"mutationId":"same","idempotencyScopeId":"` + scope + `"}`
	first := send("control.toggle-shuffle", "first", payload)
	require.Equal(t, "applied", first["outcome"])
	require.Equal(t, false, first["replayed"])
	replayed := send("control.toggle-shuffle", "retry", payload)
	require.Equal(t, true, replayed["replayed"])
	require.Equal(t, "same", replayed["mutationId"])
	require.Equal(t, first["committed"], replayed["committed"])
	require.Equal(t, "MUTATION_CONFLICT", send("control.next", "conflict", payload)["code"])
	require.Equal(t, "IDEMPOTENCY_SCOPE_MISMATCH", send("control.next", "scope", `{"mutationId":"new","idempotencyScopeId":"old"}`)["code"])
	for _, invalid := range []string{`{"mutationId":"x"}`, `{"idempotencyScopeId":"x"}`, `{"mutationId":null,"idempotencyScopeId":null}`, `{"mutationId":"","idempotencyScopeId":"x"}`} {
		require.Equal(t, "PAYLOAD_INVALID", send("control.next", "invalid", invalid)["code"])
	}
	payload = `{"mutationId":"noop","idempotencyScopeId":"` + scope + `"}`
	first = send("control.like", "noop", payload)
	require.Equal(t, "noop", first["outcome"])
	replayed = send("control.like", "noop-retry", payload)
	require.Equal(t, "noop", replayed["outcome"])
	require.Equal(t, true, replayed["replayed"])
	require.Equal(t, first["committed"], replayed["committed"])
	payload = `{"positionMs":0,"mutationId":"denied","idempotencyScopeId":"` + scope + `"}`
	first = send("control.seek", "denied", payload)
	require.Equal(t, "NO_CURRENT_TRACK", first["code"])
	replayed = send("control.seek", "denied-retry", payload)
	require.Equal(t, "rejected", replayed["outcome"])
	require.Equal(t, first["code"], replayed["code"])
	require.Equal(t, true, replayed["replayed"])
}

func TestWebSocketControlAckCorrelationAndPreconditions(t *testing.T) {
	server, _ := websocketTestServer(t)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	readUntilType(t, connection, "player.state")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"/enqueue","requestId":"r1","roomId":"lounge","payload":{"platform":"local","musicId":"track","mutationId":"m1"}}`)))
	readUntilType(t, connection, "enqueue.ack")

	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"control.seek","requestId":"s1","payload":{"positionMs":1000}}`)))
	ack := readUntilType(t, connection, "control.ack")
	require.Equal(t, "s1", ack.RequestID)
	require.Equal(t, "lounge", ack.RoomID)
	ackPayload := ack.Payload.(map[string]any)
	require.Equal(t, "applied", ackPayload["outcome"])
	committed := ackPayload["committed"].(map[string]any)
	require.Equal(t, float64(2), committed["playEpoch"])
	require.GreaterOrEqual(t, committed["stateVersion"].(float64), float64(1))
	require.GreaterOrEqual(t, committed["queueVersion"].(float64), float64(1))

	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"control.seek","requestId":"s2","payload":{"positionMs":1000,"expectedPlayEpoch":1}}`)))
	rejected := readUntilType(t, connection, "control.ack")
	require.Equal(t, "s2", rejected.RequestID)
	require.Equal(t, "lounge", rejected.RoomID)
	rejectedPayload := rejected.Payload.(map[string]any)
	require.Equal(t, "rejected", rejectedPayload["outcome"])
	require.Equal(t, "PRECONDITION_FAILED", rejectedPayload["code"])
	_, hasCommitted := rejectedPayload["committed"]
	require.False(t, hasCommitted)

	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"control.like","requestId":"s3","payload":{}}`)))
	firstLike := readUntilType(t, connection, "control.ack")
	require.Equal(t, "applied", firstLike.Payload.(map[string]any)["outcome"])
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"control.like","requestId":"s4","payload":{}}`)))
	secondLike := readUntilType(t, connection, "control.ack")
	require.Equal(t, "s4", secondLike.RequestID)
	require.Equal(t, "lounge", secondLike.RoomID)
	require.Equal(t, "noop", secondLike.Payload.(map[string]any)["outcome"])
	require.NotNil(t, secondLike.Payload.(map[string]any)["committed"])
	require.Equal(t, firstLike.Payload.(map[string]any)["committed"], secondLike.Payload.(map[string]any)["committed"])

	// A like records who liked this track, so the caller's own public id in
	// likedUserIds is the contract's answer to "did I like it"; unlike takes it back.
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"control.unlike","requestId":"s5","payload":{}}`)))
	unliked := readUntilType(t, connection, "control.ack")
	require.Equal(t, "s5", unliked.RequestID)
	require.Equal(t, "applied", unliked.Payload.(map[string]any)["outcome"])
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"control.unlike","requestId":"s6","payload":{}}`)))
	repeated := readUntilType(t, connection, "control.ack")
	require.Equal(t, "noop", repeated.Payload.(map[string]any)["outcome"])
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"player.resync","payload":{}}`)))
	resynced := readUntilType(t, connection, "player.state")
	nowPlaying := resynced.Payload.(map[string]any)["nowPlaying"].(map[string]any)
	require.NotContains(t, nowPlaying["likedUserIds"], "member")
}

func TestWebSocketSeekRequiresPositionAndAcceptsZero(t *testing.T) {
	server, _ := websocketTestServer(t)
	connection := dialWebSocket(t, server, "member-token", "lounge")
	readUntilType(t, connection, "player.state")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"/enqueue","requestId":"enqueue","payload":{"platform":"local","musicId":"track","mutationId":"seek-validation"}}`)))
	readUntilType(t, connection, "enqueue.ack")

	for _, test := range []struct {
		name    string
		payload string
	}{
		{name: "missing", payload: `{}`},
		{name: "null", payload: `{"positionMs":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			frame := `{"type":"control.seek","requestId":"` + test.name + `","payload":` + test.payload + `}`
			require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(frame)))
			ack := readUntilType(t, connection, "control.ack")
			require.Equal(t, test.name, ack.RequestID)
			require.Equal(t, map[string]any{"outcome": "rejected", "code": "PAYLOAD_INVALID"}, ack.Payload)
		})
	}

	// The rejected inputs must not advance the epoch, and explicit zero is valid.
	require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"control.seek","requestId":"zero","payload":{"positionMs":0,"expectedPlayEpoch":1}}`)))
	ack := readUntilType(t, connection, "control.ack")
	require.Equal(t, "zero", ack.RequestID)
	payload := ack.Payload.(map[string]any)
	require.Equal(t, "applied", payload["outcome"])
	require.Equal(t, float64(2), payload["committed"].(map[string]any)["playEpoch"])
}

func TestControlRejectionCodeMapping(t *testing.T) {
	locked := map[string]string{"control.toggle-pause": "PAUSE_LOCKED", "control.next": "SKIP_LOCKED", "control.previous": "SKIP_LOCKED", "control.toggle-shuffle": "SHUFFLE_LOCKED"}
	for kind, expected := range locked {
		code, ok := controlRejectionCode(kind, realtime.ErrControlLocked)
		require.True(t, ok)
		require.Equal(t, expected, code)
	}
	for _, pair := range []struct {
		err  error
		code string
	}{
		{realtime.ErrControlDenied, "NO_CURRENT_TRACK"},
		{realtime.ErrNoHistory, "NO_HISTORY"},
		{realtime.ErrSeekForbidden, "SEEK_FORBIDDEN"},
		{realtime.ErrPreconditionFailed, "PRECONDITION_FAILED"},
		{realtime.ErrCommandQueueFull, "COMMAND_REJECTED"},
	} {
		code, ok := controlRejectionCode("control.seek", pair.err)
		require.True(t, ok)
		require.Equal(t, pair.code, code)
	}
	for _, err := range []error{context.Canceled, errors.New("sqlite: disk I/O error")} {
		_, ok := controlRejectionCode("control.seek", err)
		require.False(t, ok)
	}
}

func TestDesktopWebSocketNegotiatesBeforeState(t *testing.T) {
	server, hub := websocketTestServer(t)
	for _, payload := range []string{
		`{"type":"client.hello","payload":{"apiVersion":"2026-01","clientVersion":"0.2.0"}}`,
		`{"type":"client.hello","payload":{"apiVersion":"2026-01","clientVersion":"0.1.9"}}`,
		`{"type":"client.hello","payload":{"apiVersion":"wrong","clientVersion":"0.2.0"}}`,
		`{"type":"control.next","payload":{}}`,
		`{"type":"client.hello","payload":null}`,
	} {
		t.Run(payload, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			header := http.Header{"Origin": {server.URL}, "Cookie": {SessionCookieName + "=member-token"}}
			conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/desktop/v1/ws?roomId=lounge", &websocket.DialOptions{HTTPHeader: header})
			require.NoError(t, err)
			defer conn.CloseNow()
			require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte(payload)))
			_, data, err := conn.Read(ctx)
			if strings.Contains(payload, `"apiVersion":"2026-01","clientVersion":"0.2.0"`) {
				require.NoError(t, err)
				var hello wsruntime.Envelope
				require.NoError(t, json.Unmarshal(data, &hello))
				require.Equal(t, "server.hello", hello.Type)
				require.Equal(t, "0.2.0", hello.Payload.(map[string]any)["minimumClientVersion"])
				_ = readUntilType(t, conn, "player.state")
				_ = conn.Close(websocket.StatusNormalClosure, "")
			} else {
				require.Equal(t, websocket.StatusPolicyViolation, websocket.CloseStatus(err))
				require.Contains(t, err.Error(), "version-incompatible")
			}
			require.Eventually(t, func() bool { return hub.Count() == 0 }, time.Second, time.Millisecond*10)
		})
	}
}

func TestDesktopWebSocketRequiresOriginSessionAndRoom(t *testing.T) {
	server, _ := websocketTestServer(t)
	for _, tc := range []struct {
		origin, token, room string
		upgrade             bool
	}{
		{"", "member-token", "lounge", false},
		{"https://untrusted.invalid", "member-token", "lounge", false},
		{server.URL, "", "lounge", true},
		{server.URL, "member-token", "missing", true},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		header := http.Header{"Origin": {tc.origin}, "Cookie": {SessionCookieName + "=" + tc.token}}
		conn, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/desktop/v1/ws?roomId="+tc.room, &websocket.DialOptions{HTTPHeader: header})
		if !tc.upgrade {
			require.Error(t, err)
			require.Equal(t, http.StatusForbidden, response.StatusCode)
		} else {
			require.NoError(t, err)
			_, _, err = conn.Read(ctx)
			require.Equal(t, websocket.StatusPolicyViolation, websocket.CloseStatus(err))
			_ = conn.CloseNow()
		}
		cancel()
	}
}

// Room creation moved into the room domain service so the desktop can create
// rooms over HTTP; this keeps the WebSocket path pinned to the same behaviour.
func TestWebSocketAdminRoomCreationAndFailures(t *testing.T) {
	store := httpStore(t)
	insertStage5User(t, store, "admin-token", "admin", "PLATFORM_ADMIN")
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	accounts := account.New(store)
	cfg := testConfig(t)
	hub := wsruntime.NewHub(256)
	manager := realtime.NewManager(store, 100, 5*time.Second, time.Hour, hub)
	api := NewWebSocketAPI(cfg, store, accounts, roomdomain.New(store, accounts), hub, manager, NewPlatformAPI(cfg, fixturePlatform{}))
	server := httptest.NewServer(NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api))
	t.Cleanup(func() {
		server.Close()
		hub.Close()
		manager.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	admin := dialWebSocket(t, server, "admin-token", "lounge")
	readUntilType(t, admin, "player.state")
	require.NoError(t, admin.Write(ctx, websocket.MessageText, []byte(`{"type":"rooms.create","payload":{"name":"WS Created","isPrivate":true,"password":"secret"}}`)))
	created := readUntilType(t, admin, "rooms.created")
	payload := created.Payload.(map[string]any)
	require.Equal(t, "WS Created", payload["name"])
	require.Equal(t, true, payload["privateRoom"])
	require.Equal(t, true, payload["accessGranted"])
	require.True(t, strings.HasPrefix(payload["roomId"].(string), "room-"))
	listed := readUntilType(t, admin, "rooms.list").Payload.([]any)
	names := make([]string, 0, len(listed))
	for _, item := range listed {
		names = append(names, item.(map[string]any)["name"].(string))
	}
	require.Contains(t, names, "WS Created")

	require.NoError(t, admin.Write(ctx, websocket.MessageText, []byte(`{"type":"rooms.create","payload":{"name":"ws created"}}`)))
	failed := readUntilType(t, admin, "player.events")
	event := failed.Payload.(map[string]any)
	require.Equal(t, "ROOM_CREATE_FAILED", event["code"])
	require.Equal(t, "Room name already exists", event["message"])

	member := dialWebSocket(t, server, "member-token", "lounge")
	readUntilType(t, member, "player.state")
	require.NoError(t, member.Write(ctx, websocket.MessageText, []byte(`{"type":"rooms.create","payload":{"name":"Member Room"}}`)))
	denied := readUntilType(t, member, "player.events")
	require.Equal(t, "CONTROL_DENIED", denied.Payload.(map[string]any)["code"])
}

// The creator set their own room's password, so the short-lived access ticket must not lock them
// out of it; everyone else still has to present one.
func TestRoomEntryExemptsTheOwnerFromTheAccessTicket(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	cfg := testConfig(t)
	cfg.Auth.RoomAccessTokenSecret = "contract-room-access-secret-32-bytes"
	insertStage5User(t, store, "creator-token", "owner", "MEMBER")
	insertStage5User(t, store, "stranger-token", "stranger", "MEMBER")
	insertStage5Room(t, store, "owned-private", "PRIVATE", 1)
	api := NewWebSocketAPI(cfg, store, accounts, nil, nil, nil, nil)
	request := func() *http.Request {
		return httptest.NewRequest(http.MethodGet, "/api/desktop/v1/ws?roomId=owned-private", nil)
	}
	creator, err := accounts.Resolve(context.Background(), "creator-token")
	require.NoError(t, err)
	stranger, err := accounts.Resolve(context.Background(), "stranger-token")
	require.NoError(t, err)
	require.True(t, api.canEnterRoom(request(), "owned-private", creator))
	require.False(t, api.canEnterRoom(request(), "owned-private", stranger), "a non-owner still needs a valid room ticket")
}

// The lobby's room history has to reach the whole session, not the 100-entry window the runtime
// keeps for control.previous, and a member who joins midway must be able to read what played
// before they arrived. Driven over a real socket so the assertions are on the wire shape.
func TestWebSocketRoomHistoryPagesTheWholeSession(t *testing.T) {
	server, _, store := websocketTestServerWithStore(t)
	insertStage5User(t, store, "latecomer-token", "latecomer", "MEMBER")
	repository := storesqlite.NewQueueRepository(store, nil)
	const played = 250
	seed := context.Background()
	for index := 0; index < played; index++ {
		var requester *string
		if index%10 != 0 {
			publicID := "member"
			requester = &publicID
		}
		require.NoError(t, repository.AppendHistory(seed, storesqlite.HistoryEntry{
			ID:               fmt.Sprintf("hist-%03d", index),
			RoomID:           "lounge",
			Music:            storesqlite.Music{ID: fmt.Sprintf("track-%03d", index), Name: fmt.Sprintf("Track %d", index), Artists: []string{"Artist"}, Duration: 60_000, Platform: "local"},
			EnqueuerPublicID: requester,
			PlayedAt:         int64(1_000_000 + index*1_000),
		}))
	}

	readHistory := func(t *testing.T, connection *websocket.Conn, body string) map[string]any {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"history.list","payload":`+body+`}`)))
		payload, ok := readUntilType(t, connection, "history.page").Payload.(map[string]any)
		require.True(t, ok, "history.page must carry an object payload")
		return payload
	}
	readItems := func(payload map[string]any) []map[string]any {
		raw, ok := payload["items"].([]any)
		require.True(t, ok, "items must always be an array, never null")
		items := make([]map[string]any, 0, len(raw))
		for _, value := range raw {
			items = append(items, value.(map[string]any))
		}
		return items
	}

	latecomer := dialWebSocket(t, server, "latecomer-token", "lounge")
	// A full 200-item page is about 34 KB, well past the client library's 32 KB default.
	latecomer.SetReadLimit(1 << 20)
	state := readUntilType(t, latecomer, "player.state")
	cursor := state.Payload.(map[string]any)["historyCursor"].(float64)
	require.Equal(t, float64(100), cursor, "the in-memory window control.previous walks back over is still capped")

	// A request id comes back on the page, so two pages in flight stay distinguishable; a request
	// without one keeps the field null rather than inventing an id.
	correlation, cancelCorrelation := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelCorrelation()
	require.NoError(t, latecomer.Write(correlation, websocket.MessageText, []byte(`{"type":"history.list","requestId":"page-42","payload":{"offset":0,"limit":2}}`)))
	correlated := readUntilType(t, latecomer, "history.page")
	require.Equal(t, "page-42", correlated.RequestID)
	require.Equal(t, "lounge", correlated.RoomID)

	newest := readHistory(t, latecomer, `{"offset":0,"limit":50}`)
	require.Equal(t, "lounge", newest["roomId"])
	require.Equal(t, float64(played), newest["total"], "total is the whole session, not the window")
	require.Equal(t, float64(0), newest["offset"])
	first := readItems(newest)
	require.Len(t, first, 50)
	require.Equal(t, "hist-249", first[0]["id"], "newest first")
	require.Equal(t, float64(1_249_000), first[0]["playedAt"])
	require.Equal(t, "member", first[0]["enqueuerName"], "the requester name comes from the server, not from who happens to be online")
	require.Equal(t, "member", first[0]["enqueuerPublicId"])
	require.Equal(t, "Track 249", first[0]["music"].(map[string]any)["name"])
	require.Equal(t, "hist-240", first[9]["id"])
	require.Nil(t, first[9]["enqueuerName"], "a requester whose profile is gone stays null instead of guessing")
	require.Nil(t, first[9]["enqueuerPublicId"])

	beyond := readItems(readHistory(t, latecomer, `{"offset":200,"limit":60}`))
	require.Len(t, beyond, 50)
	require.Equal(t, "hist-049", beyond[0]["id"], "paging must reach entries the memory window never held")
	require.Equal(t, "hist-000", beyond[49]["id"])

	require.Len(t, readItems(readHistory(t, latecomer, `{"limit":9999}`)), 200, "limit is clamped")
	require.Len(t, readItems(readHistory(t, latecomer, `{}`)), 50, "a missing limit is one page")
	overflow := readHistory(t, latecomer, `{"offset":900}`)
	require.Empty(t, readItems(overflow), "past the end is an empty array, not null and not an error")
	require.Equal(t, float64(played), overflow["total"])

	// The list and control.previous must agree on order, or the button would resume from a track
	// the page never showed.
	window, err := repository.LoadHistory(context.Background(), "lounge", 100)
	require.NoError(t, err)
	aligned := readItems(readHistory(t, latecomer, `{"limit":100}`))
	require.Len(t, aligned, len(window))
	for index := range window {
		require.Equal(t, window[index].ID, aligned[index]["id"])
	}
}
