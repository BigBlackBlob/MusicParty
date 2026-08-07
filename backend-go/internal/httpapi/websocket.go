package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type WebSocketAPI struct {
	cfg       config.Config
	store     *storesqlite.Store
	accounts  *account.Service
	rooms     *roomdomain.Service
	hub       *wsruntime.Hub
	runtimes  *realtime.Manager
	platforms *PlatformAPI
}

func NewWebSocketAPI(cfg config.Config, store *storesqlite.Store, accounts *account.Service, rooms *roomdomain.Service, hub *wsruntime.Hub, runtimes *realtime.Manager, platforms *PlatformAPI) *WebSocketAPI {
	return &WebSocketAPI{cfg: cfg, store: store, accounts: accounts, rooms: rooms, hub: hub, runtimes: runtimes, platforms: platforms}
}
func (api *WebSocketAPI) Routes(r chi.Router) { r.Get("/ws", api.handle) }

type inboundEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId"`
	RoomID    string          `json:"roomId"`
	Payload   json.RawMessage `json:"payload"`
}

func (api *WebSocketAPI) handle(w http.ResponseWriter, r *http.Request) {
	connection, err := wsruntime.Accept(w, r, api.cfg.Application.AllowedOrigins)
	if err != nil {
		return
	}
	token := sessionToken(r)
	session, err := api.accounts.Resolve(r.Context(), token)
	if err != nil {
		_ = connection.Close(websocket.StatusPolicyViolation, "Unknown session token")
		return
	}
	roomID := defaultValue(r.URL.Query().Get("room-id"), "lounge")
	if !api.canEnterRoom(r, roomID, session) {
		_ = connection.Close(websocket.StatusPolicyViolation, "Forbidden")
		return
	}
	client := api.hub.RegisterSession(connection, roomID, session, token)
	runtime, err := api.runtimes.Room(r.Context(), roomID)
	if err != nil {
		client.Close(websocket.StatusInternalError, "Room unavailable")
		api.hub.Unregister(client)
		return
	}
	if err := runtime.Attach(r.Context()); err != nil {
		client.Close(websocket.StatusInternalError, "Room unavailable")
		api.hub.Unregister(client)
		return
	}
	defer func() { _ = runtime.Detach(context.Background()) }()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	writeDone := make(chan error, 1)
	go func() { writeDone <- client.WriteLoop(ctx) }()
	api.sendInitial(ctx, client, runtime)
	api.hub.BroadcastRoom(roomID, "users.online", api.hub.Presence(roomID))
	for {
		var envelope inboundEnvelope
		err := wsruntime.ReadJSON(ctx, connection, &envelope)
		if err != nil {
			break
		}
		api.dispatch(ctx, client, runtime, envelope)
	}
	cancel()
	select {
	case <-writeDone:
	case <-time.After(time.Second):
	}
	api.hub.Unregister(client)
}

func (api *WebSocketAPI) canEnterRoom(r *http.Request, roomID string, session account.Session) bool {
	var visibility string
	var passwordVersion int
	err := api.store.Reader().QueryRowContext(r.Context(), "select visibility,password_version from room where id=? and deleted_at is null", roomID).Scan(&visibility, &passwordVersion)
	if err != nil {
		return false
	}
	if visibility != "PRIVATE" || session.Admin() {
		return true
	}
	proof, _ := r.Cookie(RoomAccessCookieName)
	return validRoomAccessToken(roomAccessSecret(api.cfg.Auth.RoomAccessTokenSecret), cookieValue(proof), roomID, session.PublicID, passwordVersion, time.Now())
}
func (api *WebSocketAPI) sendInitial(ctx context.Context, client *wsruntime.Client, runtime *realtime.RoomRuntime) {
	snapshot, err := runtime.Snapshot(ctx)
	if err == nil {
		_ = client.Send("player.state", "", snapshot)
	}
}
func (api *WebSocketAPI) currentUser(session account.Session) map[string]any {
	result := map[string]any{"publicId": session.PublicID, "name": session.DisplayName, "isGuest": session.Guest, "role": session.Role, "isAdmin": session.Admin()}
	return result
}

func canonical(kind string) string {
	kind = strings.TrimSpace(kind)
	if strings.HasPrefix(kind, "/") {
		kind = strings.TrimPrefix(kind, "/")
		kind = strings.ReplaceAll(kind, "/", ".")
	}
	return kind
}
func decodePayload[T any](raw json.RawMessage) (T, error) {
	var value T
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	err := json.Unmarshal(raw, &value)
	return value, err
}

func (api *WebSocketAPI) dispatch(ctx context.Context, client *wsruntime.Client, runtime *realtime.RoomRuntime, envelope inboundEnvelope) {
	kind := canonical(envelope.Type)
	session := client.SessionSnapshot()
	if (kind == "rooms.create" || kind == "rooms.delete") && !session.Admin() {
		api.event(client, "CONTROL_DENIED", "administrator required")
		return
	}
	switch kind {
	case "player.resync":
		api.scheduleResync(ctx, client, runtime, session)
	case "sync.ping":
		var request map[string]any
		_ = json.Unmarshal(envelope.Payload, &request)
		request["serverReceiveTime"] = time.Now().UnixMilli()
		request["serverSendTime"] = time.Now().UnixMilli()
		_ = client.Send("sync.pong", "", request)
	case "user.me":
		_ = client.Send("user.me", "", api.currentUser(session))
	case "users.online":
		_ = client.Send("users.online", client.RoomID, api.hub.Presence(client.RoomID))
	case "enqueue":
		api.enqueue(ctx, client, runtime, envelope)
	case "enqueue.playlist", "enqueue.album":
		api.enqueueRemoteCollection(ctx, client, runtime, envelope, kind == "enqueue.album")
	case "enqueue.room-playlist":
		api.enqueueRoomPlaylist(ctx, client, runtime, envelope)
	case "control.next":
		api.control(client, envelope.RequestID, runtime.Next(ctx))
	case "control.toggle-pause":
		api.control(client, envelope.RequestID, runtime.TogglePause(ctx))
	case "control.toggle-shuffle":
		api.control(client, envelope.RequestID, runtime.ToggleShuffle(ctx))
	case "control.seek":
		request, err := decodePayload[struct {
			PositionMS int64 `json:"positionMs"`
		}](envelope.Payload)
		if err != nil {
			api.nack(client, "queue.mutation.nack", "", err)
			return
		}
		api.control(client, envelope.RequestID, runtime.Seek(ctx, request.PositionMS, session.PublicID, session.Admin()))
	case "control.like":
		api.control(client, envelope.RequestID, runtime.Like(ctx, session.PublicID))
	case "queue.remove", "queue.top":
		request, err := decodePayload[struct{ QueueID, MutationID string }](envelope.Payload)
		if err != nil {
			api.nack(client, "queue.mutation.nack", "", err)
			return
		}
		if kind == "queue.remove" {
			err = runtime.Remove(ctx, []string{request.QueueID}, request.MutationID)
		} else {
			err = runtime.Top(ctx, []string{request.QueueID}, request.MutationID)
		}
		api.mutationResult(client, request.MutationID, err, false)
	case "queue.batch-remove", "queue.batch-top":
		request, err := decodePayload[struct {
			QueueIDs   []string `json:"queueIds"`
			MutationID string   `json:"mutationId"`
		}](envelope.Payload)
		if err == nil {
			if kind == "queue.batch-remove" {
				err = runtime.Remove(ctx, request.QueueIDs, request.MutationID)
			} else {
				err = runtime.Top(ctx, request.QueueIDs, request.MutationID)
			}
		}
		api.mutationResult(client, request.MutationID, err, false)
	case "queue.reorder":
		request, err := decodePayload[struct {
			OldIndex, NewIndex int
			MutationID         string `json:"mutationId"`
			QueueID            string `json:"queueId"`
			TargetQueueID      string `json:"targetQueueId"`
			Position           string `json:"position"`
		}](envelope.Payload)
		if err == nil {
			if request.QueueID != "" || request.TargetQueueID != "" {
				err = runtime.ReorderByID(ctx, request.QueueID, request.TargetQueueID, request.Position, request.MutationID)
			} else {
				err = runtime.Reorder(ctx, request.OldIndex, request.NewIndex, request.MutationID)
			}
		}
		api.mutationResult(client, request.MutationID, err, true)
	case "chat.message", "public-chat.message":
		request, err := decodePayload[struct{ Content, Message string }](envelope.Payload)
		if err == nil {
			content := request.Content
			if content == "" {
				content = request.Message
			}
			_, err = runtime.AppendChat(ctx, session, content, kind == "public-chat.message")
		}
		if err != nil {
			api.event(client, "CONTROL_DENIED", err.Error())
		}
	case "chat.history.fetch", "public-chat.history.fetch":
		request, _ := decodePayload[struct{ Offset, Limit int }](envelope.Payload)
		values, err := runtime.ChatHistory(ctx, request.Offset, request.Limit, kind == "public-chat.history.fetch")
		if err == nil {
			responseType := "chat.history"
			if kind == "public-chat.history.fetch" {
				responseType = "public-chat.history"
			}
			_ = client.Send(responseType, "", values)
		}
	case "user.rename":
		request, err := decodePayload[struct {
			NewName string `json:"newName"`
		}](envelope.Payload)
		if err == nil {
			updated, updateErr := api.accounts.UpdateProfile(ctx, session.SessionToken, request.NewName)
			err = updateErr
			if err == nil {
				api.hub.UpdateUserSession(updated)
			}
		}
		if err != nil {
			api.event(client, "RENAME_FAILED", err.Error())
		}
	case "user.bind":
		request, err := decodePayload[struct {
			Platform  string `json:"platform"`
			AccountID string `json:"accountId"`
		}](envelope.Payload)
		request.Platform = strings.TrimSpace(request.Platform)
		request.AccountID = strings.TrimSpace(request.AccountID)
		if err == nil && request.Platform != "" && request.AccountID != "" {
			repository := storesqlite.NewUserProfileRepository(api.store, time.Now)
			bindings, findErr := repository.FindBindingsByPublicID(ctx, session.PublicID)
			if findErr == nil {
				bindings[request.Platform] = request.AccountID
				err = repository.ReplaceBindings(ctx, session.PublicID, bindings)
			} else {
				err = findErr
			}
		} else if err == nil {
			err = errors.New("platform and accountId are required")
		}
		if err != nil {
			api.event(client, "BIND_FAILED", err.Error())
		} else {
			_ = client.Send("user.me", "", api.currentUser(session))
		}
	case "rooms.create":
		api.createRoom(ctx, client, envelope)
	case "rooms.delete":
		request, err := decodePayload[struct {
			RoomID string `json:"roomId"`
		}](envelope.Payload)
		if err == nil {
			_, err = api.rooms.Delete(ctx, request.RoomID, session.SessionToken)
		}
		if err != nil {
			api.event(client, "CONTROL_DENIED", err.Error())
		}
	}
}

func (api *WebSocketAPI) scheduleResync(ctx context.Context, client *wsruntime.Client, runtime *realtime.RoomRuntime, session account.Session) {
	go func() {
		timer := time.NewTimer(750 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		snapshot, err := runtime.Snapshot(ctx)
		if err != nil {
			return
		}
		_ = client.Send("player.state", client.RoomID, snapshot)
		if rooms, listErr := api.rooms.List(ctx, session.SessionToken); listErr == nil {
			for index := range rooms {
				rooms[index].OnlineCount = len(api.hub.Online(rooms[index].RoomID))
			}
			_ = client.Send("rooms.list", "", rooms)
		}
		_ = client.Send("player.state", client.RoomID, snapshot)
		_ = client.Send("users.online", client.RoomID, api.hub.Presence(client.RoomID))
	}()
}

func (api *WebSocketAPI) enqueueRemoteCollection(ctx context.Context, client *wsruntime.Client, runtime *realtime.RoomRuntime, envelope inboundEnvelope, album bool) {
	session := client.SessionSnapshot()
	request, err := decodePayload[struct {
		Platform   string `json:"platform"`
		PlaylistID string `json:"playlistId"`
		AlbumID    string `json:"albumId"`
		MutationID string `json:"mutationId"`
	}](envelope.Payload)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	service, ok := api.platforms.Service(request.Platform)
	if !ok {
		api.nack(client, "enqueue.nack", request.MutationID, errors.New("platform unavailable"))
		return
	}
	if check := api.platforms.access[request.Platform]; check != nil && !check(ctx, session.SessionToken) {
		api.nack(client, "enqueue.nack", request.MutationID, errors.New("forbidden"))
		return
	}
	var values []platform.Music
	if album {
		values, err = service.AlbumSongs(ctx, request.AlbumID)
	} else {
		values, err = service.PlaylistSongs(ctx, request.PlaylistID, 0, api.cfg.Player.MaxPlaylistImportSize)
	}
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	musics := make([]storesqlite.Music, 0, len(values))
	for _, value := range values {
		musics = append(musics, storesqlite.Music{ID: value.ID, Name: value.Name, Artists: value.Artists, Duration: value.Duration, Platform: value.Platform, CoverURL: value.CoverURL})
	}
	items, err := runtime.EnqueueMany(ctx, session, musics, request.MutationID)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	_ = client.Send("enqueue.ack", client.RoomID, map[string]any{"accepted": true, "count": len(items), "mutationId": request.MutationID})
}

func (api *WebSocketAPI) enqueueRoomPlaylist(ctx context.Context, client *wsruntime.Client, runtime *realtime.RoomRuntime, envelope inboundEnvelope) {
	session := client.SessionSnapshot()
	request, err := decodePayload[struct {
		PlaylistID string `json:"playlistId"`
		MutationID string `json:"mutationId"`
	}](envelope.Payload)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	tracks, err := storesqlite.NewRoomPlaylistRepository(api.store, time.Now, nil).ListTracks(ctx, client.RoomID, request.PlaylistID, 0, api.cfg.Player.MaxPlaylistImportSize)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	musics := make([]storesqlite.Music, len(tracks))
	for i := range tracks {
		musics[i] = tracks[i].Music
	}
	items, err := runtime.EnqueueMany(ctx, session, musics, request.MutationID)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	_ = client.Send("enqueue.ack", client.RoomID, map[string]any{"accepted": true, "count": len(items), "mutationId": request.MutationID})
}

func (api *WebSocketAPI) enqueue(ctx context.Context, client *wsruntime.Client, runtime *realtime.RoomRuntime, envelope inboundEnvelope) {
	session := client.SessionSnapshot()
	request, err := decodePayload[struct {
		Platform, MusicID, MutationID string
	}](envelope.Payload)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	playable, err := api.platforms.ResolvePlayable(ctx, request.Platform, request.MusicID, session.SessionToken)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	music := storesqlite.Music{ID: playable.ID, Name: playable.Name, Artists: playable.Artists, Duration: playable.Duration, Platform: playable.Platform, CoverURL: playable.CoverURL}
	item, err := runtime.Enqueue(ctx, session, music, request.MutationID)
	if err != nil {
		api.nack(client, "enqueue.nack", request.MutationID, err)
		return
	}
	_ = client.Send("enqueue.ack", client.RoomID, map[string]any{"accepted": true, "queueId": item.QueueID, "mutationId": request.MutationID})
}
func (api *WebSocketAPI) mutationResult(client *wsruntime.Client, mutationID string, err error, reorder bool) {
	kind := "queue.mutation.ack"
	if reorder {
		kind = "queue.reorder.ack"
	}
	if err != nil {
		kind = strings.Replace(kind, ".ack", ".nack", 1)
		api.nack(client, kind, mutationID, err)
		return
	}
	_ = client.Send(kind, client.RoomID, map[string]any{"mutationId": mutationID})
}
func (api *WebSocketAPI) control(client *wsruntime.Client, requestID string, err error) {
	if err != nil {
		api.event(client, "CONTROL_DENIED", err.Error())
		return
	}
	_ = client.Send("queue.mutation.ack", "", map[string]any{})
}
func (api *WebSocketAPI) nack(client *wsruntime.Client, kind, mutationID string, err error) {
	reason := "PERSISTENCE_FAILED"
	if errors.Is(err, realtime.ErrStaleQueue) {
		reason = "STALE_QUEUE"
	} else if errors.Is(err, realtime.ErrDuplicate) {
		reason = "DUPLICATE"
	} else if errors.Is(err, realtime.ErrCommandQueueFull) {
		reason = "COMMAND_REJECTED"
	} else if err != nil && err.Error() == "guest" {
		reason = "GUEST"
	}
	payload := map[string]any{"mutationId": mutationID, "reason": reason}
	if kind == "queue.reorder.nack" {
		payload["queueVersion"] = nil
	}
	_ = client.Send(kind, "", payload)
}
func (api *WebSocketAPI) event(client *wsruntime.Client, action, message string) {
	api.eventWithMetadata(client, action, message, nil)
}
func (api *WebSocketAPI) eventWithMetadata(client *wsruntime.Client, code, message string, metadata map[string]any) {
	payload := map[string]any{"code": code, "severity": "error", "message": message}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}
	_ = client.Send("player.events", client.RoomID, payload)
}
func (api *WebSocketAPI) createRoom(ctx context.Context, client *wsruntime.Client, envelope inboundEnvelope) {
	session := client.SessionSnapshot()
	request, err := decodePayload[struct {
		Name, Password string
		IsPrivate      bool `json:"isPrivate"`
	}](envelope.Payload)
	if err != nil {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || (request.IsPrivate && strings.TrimSpace(request.Password) == "") {
		api.event(client, "ROOM_CREATE_FAILED", "Room name and private room password are required")
		return
	}
	var duplicate int
	if err := api.store.Reader().QueryRowContext(ctx, "select count(1) from room where lower(name)=lower(?) and deleted_at is null", request.Name).Scan(&duplicate); err != nil || duplicate > 0 {
		api.event(client, "ROOM_CREATE_FAILED", "Room name already exists")
		return
	}
	roomID := "room-" + uuid.NewString()[:8]
	visibility := "PUBLIC"
	var passwordHash any
	passwordVersion := 0
	if request.IsPrivate {
		visibility = "PRIVATE"
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			api.event(client, "ROOM_CREATE_FAILED", hashErr.Error())
			return
		}
		passwordHash = string(hash)
		passwordVersion = 1
	}
	now := time.Now().UnixMilli()
	err = api.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "insert into room(id,name,owner_public_id,visibility,password_hash,password_version,system,created_at,last_active_at) values(?,?,?,?,?,?,0,?,?)", roomID, request.Name, session.PublicID, visibility, passwordHash, passwordVersion, now, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "insert into room_membership(room_id,public_id,role,created_at,updated_at) values(?,?,'OWNER',?,?)", roomID, session.PublicID, now, now)
		return err
	})
	if err != nil {
		api.event(client, "ROOM_CREATE_FAILED", err.Error())
		return
	}
	room := roomdomain.Info{RoomID: roomID, Name: request.Name, CreatorPublicID: session.PublicID, CreatedAt: now, PrivateRoom: request.IsPrivate, AccessGranted: true}
	_ = client.Send("rooms.created", "", room)
	if rooms, err := api.rooms.List(ctx, session.SessionToken); err == nil {
		api.hub.BroadcastAll("rooms.list", rooms)
	}
}
