package httpapi

import (
	"context"
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

func TestRoomAccessChangesCloseExistingSockets(t *testing.T) {
	for _, change := range []string{"password", "visibility", "remove-member", "owner-transfer", "owner-demotion"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			store := httpStore(t)
			for _, user := range []struct{ id, role string }{{"owner", "MEMBER"}, {"member", "MEMBER"}, {"admin", "PLATFORM_ADMIN"}} {
				insertStage5User(t, store, user.id+"-token", user.id, user.role)
			}
			insertStage5Room(t, store, "room", "PUBLIC", 0)
			insertStage5Room(t, store, "other", "PUBLIC", 0)
			members := storesqlite.NewRoomAccessRepository(store)
			for _, id := range []string{"owner", "member"} {
				role := "MEMBER"
				if id == "owner" || change == "owner-demotion" {
					role = "OWNER"
				}
				require.NoError(t, members.UpsertMembership(ctx, storesqlite.RoomMembership{RoomID: "room", PublicID: id, Role: role, CreatedAt: 1, UpdatedAt: 1}))
			}
			accounts := account.New(store)
			rooms := roomdomain.New(store, accounts)
			cfg := testConfig(t)
			proof := ""
			if change == "password" {
				private := true
				_, err := rooms.UpdateDetails(ctx, "room", "owner-token", roomdomain.UpdateInput{Name: "Room", Private: &private, Password: "old-password"})
				require.NoError(t, err)
				metadata, err := rooms.Access(ctx, "room")
				require.NoError(t, err)
				proof = signRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), "room", "member", time.Now().Add(time.Minute).UnixMilli(), metadata.PasswordVersion)
			}
			hub := wsruntime.NewHub(128)
			manager := realtime.NewManager(store, 32, time.Second, time.Hour, hub)
			api := NewWebSocketAPI(cfg, store, accounts, rooms, hub, manager, NewPlatformAPI(cfg, fixturePlatform{}))
			roomAPI := NewRoomAPI(rooms, cfg)
			roomAPI.SetAuthService(accounts)
			handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api, roomAPI)
			server := httptest.NewServer(handler)
			t.Cleanup(func() { api.Close(); server.Close(); manager.Close() })
			target := "member"
			if change == "owner-transfer" || change == "owner-demotion" {
				target = "owner"
			}
			dialTarget := func() *websocket.Conn {
				header := http.Header{"Origin": {server.URL}, "Cookie": {SessionCookieName + "=" + target + "-token; " + RoomAccessCookieName + "=" + proof}}
				connection, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/ws?room-id=room", &websocket.DialOptions{HTTPHeader: header})
				require.NoError(t, err)
				t.Cleanup(func() { _ = connection.CloseNow() })
				return connection
			}
			connection := dialTarget()
			readUntilType(t, connection, "player.state")
			other := dialWebSocket(t, server, target+"-token", "other")
			readUntilType(t, other, "player.state")
			admin := dialWebSocket(t, server, "admin-token", "room")
			readUntilType(t, admin, "player.state")
			var response *httptest.ResponseRecorder
			switch change {
			case "password", "visibility":
				response = batch2Request(handler, http.MethodPut, "/api/rooms/room", `{"name":"Room","isPrivate":true,"password":"new-password"}`, "owner-token", "")
			case "remove-member":
				response = batch2Request(handler, http.MethodDelete, "/api/rooms/room/members/member", "", "admin-token", "")
			case "owner-transfer":
				response = batch2Request(handler, http.MethodPost, "/api/rooms/room/owners/member", "", "admin-token", "")
			case "owner-demotion":
				response = batch2Request(handler, http.MethodDelete, "/api/rooms/room/owners/owner", "", "admin-token", "")
			}
			expectedStatus := http.StatusNoContent
			if change == "password" || change == "visibility" {
				expectedStatus = http.StatusOK
			}
			require.Equal(t, expectedStatus, response.Code, response.Body.String())
			readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			for {
				_, _, err := connection.Read(readCtx)
				if err != nil {
					require.Equal(t, websocket.StatusCode(4403), websocket.CloseStatus(err))
					break
				}
			}
			for _, unaffected := range []*websocket.Conn{other, admin} {
				require.NoError(t, unaffected.Write(readCtx, websocket.MessageText, []byte(`{"type":"sync.ping","payload":{"pingId":"still-open","clientSendTime":1}}`)))
				require.Equal(t, "still-open", readUntilType(t, unaffected, "sync.pong").Payload.(map[string]any)["pingId"])
			}
			if change == "password" || change == "visibility" {
				denied := dialTarget()
				_, _, err := denied.Read(readCtx)
				require.Equal(t, websocket.StatusPolicyViolation, websocket.CloseStatus(err), "reconnecting with the old room proof remains forbidden")
			}
		})
	}
}
