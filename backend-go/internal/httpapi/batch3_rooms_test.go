package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/stretchr/testify/require"
)

func TestBatch3RenameKeepsPrivateAccess(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "owner-token", "owner", "MEMBER")
	service := roomdomain.New(store, accounts)
	created, err := service.Create(context.Background(), "owner-token", roomdomain.CreateInput{Name: "Private", Private: true, Password: "secret"})
	require.NoError(t, err)
	before, err := service.Access(context.Background(), created.RoomID)
	require.NoError(t, err)
	cfg := testConfig(t)
	api := NewRoomAPI(service, cfg)
	api.SetAuthService(accounts)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	renamed := batch2Request(handler, http.MethodPut, "/api/rooms/"+created.RoomID, `{"name":"Renamed"}`, "owner-token", "")
	require.Equal(t, http.StatusOK, renamed.Code, renamed.Body.String())
	after, err := service.Access(context.Background(), created.RoomID)
	require.NoError(t, err)
	require.Equal(t, before, after, "omitted visibility must preserve both the password and its version")
	public := batch2Request(handler, http.MethodPut, "/api/rooms/"+created.RoomID, `{"name":"Renamed","isPrivate":false}`, "owner-token", "")
	require.Equal(t, http.StatusOK, public.Code, public.Body.String())
	after, err = service.Access(context.Background(), created.RoomID)
	require.NoError(t, err)
	require.False(t, after.Private)
	require.Equal(t, before.PasswordVersion+1, after.PasswordVersion)
}

func TestBatch3DeletingRoomDisconnectsWebSocket(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "owner-token", "owner", "MEMBER")
	rooms := roomdomain.New(store, accounts)
	created, err := rooms.Create(context.Background(), "owner-token", roomdomain.CreateInput{Name: "Disposable"})
	require.NoError(t, err)
	cfg := testConfig(t)
	hub := wsruntime.NewHub(64)
	unrelated := hub.Register(nil, "lounge", account.Session{PublicID: "other"})
	manager := realtime.NewManager(store, 10, time.Second, time.Hour, hub)
	defer manager.Close()
	defer hub.Close()
	rooms.SetRoomDeletedHook(func(roomID string) {
		manager.CloseRoom(roomID)
		hub.CloseRoom(roomID)
	})
	api := NewWebSocketAPI(cfg, store, accounts, rooms, hub, manager, NewPlatformAPI(cfg))
	server := httptest.NewServer(NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api))
	defer server.Close()
	connection := dialWebSocket(t, server, "owner-token", created.RoomID)
	defer connection.CloseNow()
	readUntilType(t, connection, "player.state")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	closed := make(chan error, 1)
	go func() {
		for {
			_, _, readErr := connection.Read(ctx)
			if readErr != nil {
				closed <- readErr
				return
			}
		}
	}()
	deleted, err := rooms.Delete(context.Background(), created.RoomID, "owner-token")
	require.NoError(t, err)
	require.True(t, deleted)
	require.Equal(t, 1, hub.Count(), "only the deleted room must lose its presence")
	require.Empty(t, hub.Presence(created.RoomID).Users)
	require.NoError(t, unrelated.Send("test", "lounge", nil), "other rooms must stay connected")
	require.Error(t, <-closed)
	require.NoError(t, ctx.Err(), "server must close the socket instead of waiting for the client to time out")
}
