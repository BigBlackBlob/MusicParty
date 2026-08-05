package ws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/coder/websocket"
	"github.com/google/uuid"
)

var coalescible = map[string]struct{}{"player.state": {}, "player.queue": {}, "users.online": {}, "rooms.list": {}, "player.progress": {}}

type Message struct {
	Type   string
	RoomID any
	Data   []byte
}

type Client struct {
	ID, RoomID         string
	session            account.Session
	sessionFingerprint string
	conn               *websocket.Conn
	capacity           int
	mu                 sync.Mutex
	queue              []Message
	notify             chan struct{}
	closed             chan struct{}
	closeOnce          sync.Once
}

func newClient(conn *websocket.Conn, roomID string, session account.Session, capacity int, sessionFingerprint string) *Client {
	return &Client{ID: uuid.NewString(), RoomID: roomID, session: session, sessionFingerprint: sessionFingerprint, conn: conn, capacity: max(1, capacity), notify: make(chan struct{}, 1), closed: make(chan struct{})}
}

func (c *Client) SessionSnapshot() account.Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

func (c *Client) UpdateIdentity(session account.Session) {
	c.mu.Lock()
	c.session.DisplayName = session.DisplayName
	c.session.Role = session.Role
	c.session.Guest = session.Guest
	c.session.Enabled = session.Enabled
	c.session.LastLoginAt = session.LastLoginAt
	c.mu.Unlock()
}

func (c *Client) Send(kind, roomID string, payload any) error {
	var envelopeRoom any
	if roomID != "" {
		envelopeRoom = roomID
	}
	encoded, err := json.Marshal(Envelope{Type: kind, RoomID: envelopeRoom, Payload: payload})
	if err != nil {
		return err
	}
	c.mu.Lock()
	if _, ok := coalescible[kind]; ok {
		for index := range c.queue {
			if c.queue[index].Type == kind {
				c.queue[index] = Message{kind, roomID, encoded}
				c.mu.Unlock()
				c.signal()
				return nil
			}
		}
	}
	if len(c.queue) >= c.capacity {
		c.mu.Unlock()
		c.Close(websocket.StatusPolicyViolation, "WebSocket client queue is full")
		return errors.New("websocket client queue is full")
	}
	c.queue = append(c.queue, Message{kind, roomID, encoded})
	c.mu.Unlock()
	c.signal()
	return nil
}
func (c *Client) Direct(envelope Envelope) error {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if len(c.queue) >= c.capacity {
		c.mu.Unlock()
		c.Close(websocket.StatusPolicyViolation, "WebSocket client queue is full")
		return errors.New("websocket client queue is full")
	}
	c.queue = append(c.queue, Message{envelope.Type, envelope.RoomID, encoded})
	c.mu.Unlock()
	c.signal()
	return nil
}
func (c *Client) signal() {
	select {
	case c.notify <- struct{}{}:
	default:
	}
}
func (c *Client) pop() (Message, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queue) == 0 {
		return Message{}, false
	}
	m := c.queue[0]
	copy(c.queue, c.queue[1:])
	c.queue[len(c.queue)-1] = Message{}
	c.queue = c.queue[:len(c.queue)-1]
	return m, true
}
func (c *Client) WriteLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.closed:
			return nil
		case <-c.notify:
			for {
				message, ok := c.pop()
				if !ok {
					break
				}
				writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				err := c.conn.Write(writeCtx, websocket.MessageText, message.Data)
				cancel()
				if err != nil {
					return err
				}
			}
		}
	}
}
func (c *Client) Close(status websocket.StatusCode, reason string) {
	c.closeOnce.Do(func() {
		close(c.closed)
		if c.conn != nil {
			_ = c.conn.Close(status, reason)
		}
	})
}

type Hub struct {
	mu                sync.RWMutex
	clients           map[string]*Client
	rooms             map[string]map[string]*Client
	presenceRevisions map[string]uint64
	capacity          int
}

func NewHub(capacity int) *Hub {
	return &Hub{clients: map[string]*Client{}, rooms: map[string]map[string]*Client{}, presenceRevisions: map[string]uint64{}, capacity: max(1, capacity)}
}
func (h *Hub) Register(conn *websocket.Conn, roomID string, session account.Session) *Client {
	return h.register(conn, roomID, session, "")
}

// RegisterSession records an irreversible session fingerprint so an HTTP
// logout can close all realtime connections sharing that browser session.
func (h *Hub) RegisterSession(conn *websocket.Conn, roomID string, session account.Session, sessionToken string) *Client {
	return h.register(conn, roomID, session, fingerprint(sessionToken))
}

func (h *Hub) register(conn *websocket.Conn, roomID string, session account.Session, sessionFingerprint string) *Client {
	client := newClient(conn, roomID, session, h.capacity, sessionFingerprint)
	h.mu.Lock()
	h.clients[client.ID] = client
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = map[string]*Client{}
	}
	h.rooms[roomID][client.ID] = client
	h.presenceRevisions[roomID]++
	h.mu.Unlock()
	return client
}

func (h *Hub) Unregister(client *Client) {
	if client == nil {
		return
	}
	var snapshot PresenceSnapshot
	changed := false
	h.mu.Lock()
	if _, ok := h.clients[client.ID]; ok {
		delete(h.clients, client.ID)
		room := h.rooms[client.RoomID]
		delete(room, client.ID)
		if len(room) == 0 {
			delete(h.rooms, client.RoomID)
		}
		h.presenceRevisions[client.RoomID]++
		snapshot = h.presenceLocked(client.RoomID)
		changed = true
	}
	h.mu.Unlock()
	client.Close(websocket.StatusNormalClosure, "")
	if changed {
		h.BroadcastRoom(snapshot.RoomID, "users.online", snapshot)
	}
}

// CloseSession immediately removes every realtime client associated with one
// browser session. Session matching uses an irreversible fingerprint.
func (h *Hub) CloseSession(sessionToken string) {
	fingerprint := fingerprint(sessionToken)
	if fingerprint == "" {
		return
	}
	clients, snapshots := h.detachSession(fingerprint)
	for _, client := range clients {
		client.Close(websocket.StatusPolicyViolation, "Session revoked")
	}
	for _, snapshot := range snapshots {
		h.BroadcastRoom(snapshot.RoomID, "users.online", snapshot)
	}
}

func (h *Hub) detachSession(sessionFingerprint string) ([]*Client, []PresenceSnapshot) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := make([]*Client, 0)
	changedRooms := map[string]struct{}{}
	for id, client := range h.clients {
		if client.sessionFingerprint != sessionFingerprint {
			continue
		}
		delete(h.clients, id)
		if room := h.rooms[client.RoomID]; room != nil {
			delete(room, id)
			if len(room) == 0 {
				delete(h.rooms, client.RoomID)
			}
		}
		clients = append(clients, client)
		changedRooms[client.RoomID] = struct{}{}
	}
	snapshots := make([]PresenceSnapshot, 0, len(changedRooms))
	for roomID := range changedRooms {
		h.presenceRevisions[roomID]++
		snapshots = append(snapshots, h.presenceLocked(roomID))
	}
	return clients, snapshots
}

// UpdateUserSession publishes a display identity change through the same
// versioned presence stream used for joins and leaves.
func (h *Hub) UpdateUserSession(session account.Session) {
	h.mu.Lock()
	changedRooms := map[string]struct{}{}
	for _, client := range h.clients {
		if client.SessionSnapshot().PublicID != session.PublicID {
			continue
		}
		client.UpdateIdentity(session)
		changedRooms[client.RoomID] = struct{}{}
	}
	snapshots := make([]PresenceSnapshot, 0, len(changedRooms))
	for roomID := range changedRooms {
		h.presenceRevisions[roomID]++
		snapshots = append(snapshots, h.presenceLocked(roomID))
	}
	h.mu.Unlock()
	for _, snapshot := range snapshots {
		h.BroadcastRoom(snapshot.RoomID, "users.online", snapshot)
	}
}
func (h *Hub) BroadcastRoom(roomID, kind string, payload any) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[roomID]))
	for _, client := range h.rooms[roomID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		_ = client.Send(kind, roomID, payload)
	}
}
func (h *Hub) BroadcastAll(kind string, payload any) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		_ = client.Send(kind, "", payload)
	}
}

type PresenceSnapshot struct {
	RoomID   string                    `json:"roomId"`
	Revision uint64                    `json:"revision"`
	Users    []storesqlite.UserSummary `json:"users"`
}

func (h *Hub) Presence(roomID string) PresenceSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.presenceLocked(roomID)
}

func (h *Hub) Online(roomID string) []storesqlite.UserSummary {
	return h.Presence(roomID).Users
}

func (h *Hub) presenceLocked(roomID string) PresenceSnapshot {
	seen := map[string]struct{}{}
	out := []storesqlite.UserSummary{}
	for _, client := range h.rooms[roomID] {
		session := client.SessionSnapshot()
		if _, ok := seen[session.PublicID]; ok {
			continue
		}
		seen[session.PublicID] = struct{}{}
		out = append(out, storesqlite.UserSummary{PublicID: session.PublicID, Name: session.DisplayName, Guest: session.Guest})
	}
	return PresenceSnapshot{RoomID: roomID, Revision: h.presenceRevisions[roomID], Users: out}
}
func (h *Hub) Count() int { h.mu.RLock(); defer h.mu.RUnlock(); return len(h.clients) }
func (h *Hub) Close() {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		client.Close(websocket.StatusGoingAway, "server shutdown")
	}
}

func fingerprint(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
