package httpapi

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

// A token whose account has since been disabled must not open a new WebSocket
// session. The handshake is rejected with a policy violation, the same close code
// an unknown token gets, so existing clients see a terminal authentication failure.
func TestWebSocketRejectsDisabledAccount(t *testing.T) {
	server, _, store := websocketTestServerWithStore(t)
	insertStage5User(t, store, "ws-disabled-token", "ws-disabled", "MEMBER")
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update user_account set enabled=0 where public_id=?", "ws-disabled")
		return err
	}))

	connection := dialWebSocket(t, server, "ws-disabled-token", "lounge")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _, err := connection.Read(ctx)
	var closeError websocket.CloseError
	require.ErrorAs(t, err, &closeError)
	require.Equal(t, websocket.StatusPolicyViolation, closeError.Code)
}
