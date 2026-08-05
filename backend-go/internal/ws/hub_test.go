package ws

import (
	"encoding/json"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

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
