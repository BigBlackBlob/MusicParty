package realtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestBatch5ChatRateLimitPreventsPersistenceAndBroadcast(t *testing.T) {
	store := realtimeStore(t)
	broadcaster := &recordingBroadcaster{}
	manager := NewManager(store, 10, time.Second, time.Hour, broadcaster)
	defer manager.Close()
	room, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	user := account.Session{PublicID: "user"}
	results := make(chan error, 20)
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() { _, err := room.AppendChat(context.Background(), user, "message", false); results <- err })
	}
	workers.Wait()
	close(results)
	admitted := 0
	for err := range results {
		if err == nil {
			admitted++
		} else {
			require.ErrorIs(t, err, ErrChatRateLimited)
		}
	}
	require.Equal(t, 1, admitted)
	messages, err := room.ChatHistory(context.Background(), 0, 100, false)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, 1, broadcaster.count("chat.message"))
}

func TestBatch5ChatStreamsSharePublicLimitAndRetainSeparately(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	require.NoError(t, storesqlite.NewRoomRepository(store).Upsert(ctx, storesqlite.Room{ID: "other", Name: "Other", Visibility: "PUBLIC"}))
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	defer manager.Close()
	manager.SetChatLimits(ChatLimits{MinInterval: time.Second, MaxHistorySize: 2, MaxMessageLength: 5})
	now := time.Now()
	manager.chat.now = func() time.Time { return now }
	first, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	second, err := manager.Room(ctx, "other")
	require.NoError(t, err)
	user := account.Session{PublicID: "user"}
	_, err = first.AppendChat(ctx, user, "public", true)
	require.Error(t, err, "configured length must be enforced")
	_, err = first.AppendChat(ctx, user, "pub1", true)
	require.NoError(t, err)
	_, err = second.AppendChat(ctx, user, "pub2", true)
	require.ErrorIs(t, err, ErrChatRateLimited)
	_, err = first.AppendChat(ctx, user, "room1", false)
	require.NoError(t, err)
	_, err = second.AppendChat(ctx, user, "other", false)
	require.NoError(t, err)
	now = now.Add(time.Second)
	_, err = second.AppendChat(ctx, user, "pub2", true)
	require.NoError(t, err)
	now = now.Add(time.Second)
	_, err = first.AppendChat(ctx, user, "pub3", true)
	require.NoError(t, err)
	public, err := second.ChatHistory(ctx, 0, 100, true)
	require.NoError(t, err)
	require.Len(t, public, 2)
	require.Equal(t, "pub3", public[0].Content)
	require.Equal(t, "pub2", public[1].Content)
	private, err := first.ChatHistory(ctx, 0, 100, false)
	require.NoError(t, err)
	require.Len(t, private, 1)
	require.Equal(t, "room1", private[0].Content)
	first.Close()
	rebuilt, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	_, err = rebuilt.AppendChat(ctx, user, "retry", true)
	require.ErrorIs(t, err, ErrChatRateLimited)
}
