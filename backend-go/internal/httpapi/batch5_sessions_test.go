package httpapi

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http/httptest"
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

func TestBatch5PasswordChangeClosesAllAccountConnections(t *testing.T) {
	ctx := context.Background()
	store := httpStore(t)
	accounts := account.New(store)
	require.NoError(t, accounts.Bootstrap(ctx, "admin", "old-password"))
	first, err := accounts.Login(ctx, "admin", "old-password")
	require.NoError(t, err)
	second, err := accounts.Login(ctx, "admin", "old-password")
	require.NoError(t, err)
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	cfg := testConfig(t)
	hub := wsruntime.NewHub(64)
	defer hub.Close()
	accounts.SetSessionsRevokedHook(hub.CloseSessionHashes)
	manager := realtime.NewManager(store, 10, time.Second, time.Hour, hub)
	defer manager.Close()
	api := NewWebSocketAPI(cfg, store, accounts, roomdomain.New(store, accounts), hub, manager, NewPlatformAPI(cfg))
	server := httptest.NewServer(NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api))
	defer server.Close()
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	closed := make(chan error, 2)
	for _, token := range []string{first.SessionToken, second.SessionToken} {
		conn := dialWebSocket(t, server, token, "lounge")
		defer conn.CloseNow()
		readUntilType(t, conn, "player.state")
		go func(conn *websocket.Conn) {
			for {
				_, _, err := conn.Read(readCtx)
				if err != nil {
					closed <- err
					return
				}
			}
		}(conn)
	}
	require.Equal(t, 2, hub.Count())
	require.NoError(t, accounts.ChangePassword(ctx, first.SessionToken, "old-password", "new-password"))
	require.Error(t, <-closed)
	require.Error(t, <-closed)
	require.NoError(t, readCtx.Err())
	require.Equal(t, 0, hub.Count())
	_, err = accounts.Resolve(ctx, second.SessionToken)
	require.ErrorIs(t, err, account.ErrUnknownSession)
}

func TestBatch5WebSocketExpiresWithoutIncomingCommands(t *testing.T) {
	server, hub, store := websocketTestServerWithStore(t)
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "update user_account set role='ADMIN' where public_id='member'"); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "update user_session set created_at=?", time.Now().Add(-account.AdminSessionLifetime+2*time.Second).UnixMilli())
		return err
	}))
	conn := dialWebSocket(t, server, "member-token", "lounge")
	defer conn.CloseNow()
	readUntilType(t, conn, "player.state")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}
	require.NoError(t, ctx.Err(), "server must close the idle socket at expiry")
	require.Eventually(t, func() bool { return hub.Count() == 0 }, time.Second, time.Millisecond)
	expired := dialWebSocket(t, server, "member-token", "lounge")
	defer expired.CloseNow()
	_, _, err := expired.Read(ctx)
	require.Error(t, err)
	require.NoError(t, ctx.Err())
	require.Zero(t, hub.Count())
}
