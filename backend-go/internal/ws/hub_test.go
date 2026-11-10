package ws

import (
	"encoding/json"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

type countedPayload struct{ calls *int }

func (p countedPayload) MarshalJSON() ([]byte, error) {
	*p.calls++
	return []byte(`{"state":1}`), nil
}

func TestBroadcastEncodesOnceAndRevocationPreservesOtherClients(t *testing.T) {
	hub := NewHub(16)
	first := hub.Register(nil, "room", account.Session{PublicID: "first"})
	second := hub.Register(nil, "room", account.Session{PublicID: "second"})
	other := hub.Register(nil, "other", account.Session{PublicID: "first"})
	calls := 0
	hub.BroadcastRoom("room", "player.state", countedPayload{&calls})
	require.Equal(t, 1, calls)
	require.Len(t, first.queue, 1)
	require.Equal(t, first.queue[0].Data, second.queue[0].Data)
	hub.CloseRoomWhere("room", "access revoked", func(s account.Session) bool { return s.PublicID == "first" })
	require.Equal(t, 2, hub.Count())
	select {
	case <-first.closed:
	default:
		t.Fatal("revoked client remains open")
	}
	require.Empty(t, other.queue)
	require.Len(t, hub.Presence("room").Users, 1)
	hub.BroadcastAll("rooms.list", countedPayload{&calls})
	require.Equal(t, 2, calls)
}

func TestClientQueuePreservesReliableFIFOAndCoalescesLatestState(t *testing.T) {
	client := newClient(nil, "lounge", account.Session{PublicID: "user"}, 4, "")
	require.NoError(t, client.Send("chat.message", "lounge", map[string]any{"value": 1}))
	require.NoError(t, client.Send("player.state", "lounge", map[string]any{"version": 1}))
	require.NoError(t, client.Send("player.state", "lounge", map[string]any{"version": 2}))
	require.NoError(t, client.Send("queue.mutation.ack", "lounge", map[string]any{"value": 3}))
	require.Len(t, client.queue, 3)

	first, ok := client.pop()
	require.True(t, ok)
	require.Equal(t, "chat.message", first.Type)
	second, ok := client.pop()
	require.True(t, ok)
	require.Equal(t, "player.state", second.Type)
	var envelope Envelope
	require.NoError(t, json.Unmarshal(second.Data, &envelope))
	payload := envelope.Payload.(map[string]any)
	require.Equal(t, float64(2), payload["version"])
	third, ok := client.pop()
	require.True(t, ok)
	require.Equal(t, "queue.mutation.ack", third.Type)
}

func TestFullPlayerBaselineSurvivesCompactBroadcastCoalescing(t *testing.T) {
	hub := NewHub(8)
	client := hub.Register(nil, "room", account.Session{})
	require.NoError(t, client.Send("player.state", "room", map[string]any{"queue": []any{}, "stateVersion": 1}))
	hub.BroadcastRoom("room", "queue.patch", map[string]any{"operation": "clear", "queueVersion": 2})
	hub.BroadcastRoom("room", "player.state", map[string]any{"stateVersion": 2})
	hub.BroadcastRoom("room", "player.state", map[string]any{"stateVersion": 3})
	require.Len(t, client.queue, 3)
	var baseline, compact Envelope
	require.NoError(t, json.Unmarshal(client.queue[0].Data, &baseline))
	require.Contains(t, baseline.Payload, "queue")
	require.Equal(t, "queue.patch", client.queue[1].Type)
	require.NoError(t, json.Unmarshal(client.queue[2].Data, &compact))
	require.NotContains(t, compact.Payload, "queue")
	require.Equal(t, float64(3), compact.Payload.(map[string]any)["stateVersion"])
}

func TestReliableMessagesAreNotEvictedByStateAndFullQueueClosesClient(t *testing.T) {
	client := newClient(nil, "lounge", account.Session{}, 2, "")
	require.NoError(t, client.Send("chat.message", "lounge", 1))
	require.NoError(t, client.Send("queue.mutation.ack", "lounge", 2))
	require.Error(t, client.Send("player.state", "lounge", 3))
	select {
	case <-client.closed:
	default:
		t.Fatal("full queue did not close client")
	}
	require.Equal(t, "chat.message", client.queue[0].Type)
	require.Equal(t, "queue.mutation.ack", client.queue[1].Type)
}

func TestHubOnlineDeduplicatesAndSessionUpdateIsVisible(t *testing.T) {
	hub := NewHub(4)
	first := hub.Register(nil, "lounge", account.Session{PublicID: "user", DisplayName: "Old"})
	second := hub.Register(nil, "lounge", account.Session{PublicID: "user", DisplayName: "Old"})
	require.Len(t, hub.Online("lounge"), 1)
	hub.UpdateUserSession(account.Session{PublicID: "user", DisplayName: "New"})
	online := hub.Online("lounge")
	require.Len(t, online, 1)
	require.Contains(t, []string{"New", "Old"}, online[0].Name)
	hub.Unregister(first)
	hub.Unregister(second)
	require.Zero(t, hub.Count())
}

func TestHubPresenceIsVersionedAndLogoutRemovesOnlyMatchingSession(t *testing.T) {
	hub := NewHub(4)
	first := hub.RegisterSession(nil, "lounge", account.Session{PublicID: "first", DisplayName: "First"}, "session-one")
	firstPresence := hub.Presence("lounge")
	require.Equal(t, uint64(1), firstPresence.Revision)
	require.Equal(t, []storesqlite.UserSummary{{PublicID: "first", Name: "First"}}, firstPresence.Users)

	hub.RegisterSession(nil, "lounge", account.Session{PublicID: "second", DisplayName: "Second"}, "session-two")
	secondPresence := hub.Presence("lounge")
	require.Equal(t, uint64(2), secondPresence.Revision)
	require.Len(t, secondPresence.Users, 2)

	hub.CloseSession("session-one")
	afterLogout := hub.Presence("lounge")
	require.Equal(t, uint64(3), afterLogout.Revision)
	require.Equal(t, []storesqlite.UserSummary{{PublicID: "second", Name: "Second"}}, afterLogout.Users)
	require.Equal(t, 1, hub.Count())

	hub.Unregister(first)
	require.Equal(t, uint64(3), hub.Presence("lounge").Revision, "already detached clients must not advance presence")
}
