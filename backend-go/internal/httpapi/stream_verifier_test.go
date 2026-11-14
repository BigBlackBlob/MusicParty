package httpapi

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

// Like NetEase, metadata resolves successfully while stream resolution rejects previews.
type previewStreamSource struct{ fixturePlatform }

func (previewStreamSource) Name() string { return "netease" }
func (previewStreamSource) StreamURL(context.Context, string) (string, error) {
	return "", &platform.StreamResolutionError{Kind: platform.StreamTrialOnly}
}
func (previewStreamSource) CoverURL(context.Context, string) (string, error) { return "", nil }

type streamFailureRecorder struct {
	mu       sync.Mutex
	failures []map[string]string
}

func (*streamFailureRecorder) BroadcastAll(string, any) {}
func (b *streamFailureRecorder) BroadcastRoom(_ string, kind string, payload any) {
	if kind == "player.failed" {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.failures = append(b.failures, payload.(map[string]string))
	}
}

func TestVerifyStreamConfirmsPreviewAndAdvancesRoomOnce(t *testing.T) {
	ctx := context.Background()
	api := NewPlatformAPI(testConfig(t), previewStreamSource{})
	_, err := api.ResolvePlayable(ctx, "netease", "first", "")
	require.NoError(t, err, "metadata alone cannot detect a preview")
	broadcaster := &streamFailureRecorder{}
	store := httpStore(t)
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	manager := realtime.NewManager(store, 32, time.Second, time.Hour, broadcaster)
	manager.SetStreamVerifier(func(ctx context.Context, music storesqlite.Music) error {
		return api.VerifyStream(ctx, music.Platform, music.ID)
	})
	t.Cleanup(manager.Close)
	room, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	for _, id := range []string{"first", "second", "third"} {
		_, err = room.Enqueue(ctx, account.Session{PublicID: "user"}, storesqlite.Music{ID: id, Platform: "netease", Duration: 60000}, id)
		require.NoError(t, err)
	}
	state, err := room.Snapshot(ctx)
	require.NoError(t, err)
	now := state["nowPlaying"].(map[string]any)
	report := realtime.PlaybackReport{PlayEpoch: now["playEpoch"].(int64), QueueID: now["queueId"].(string), Code: "stream-trial-only"}
	require.NoError(t, room.ReportPlayback(ctx, "playback.error", report, nil, "client"))
	require.Eventually(t, func() bool {
		broadcaster.mu.Lock()
		defer broadcaster.mu.Unlock()
		return len(broadcaster.failures) == 1
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, room.ReportPlayback(ctx, "playback.error", report, nil, "client"))
	report.Code = "retries-exhausted"
	require.NoError(t, room.ReportPlayback(ctx, "playback.error", report, nil, "client"))
	state, err = room.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, "second", state["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic).ID)
	broadcaster.mu.Lock()
	defer broadcaster.mu.Unlock()
	require.Len(t, broadcaster.failures, 1)
	require.Equal(t, "preview_only", broadcaster.failures[0]["reason"])
}

func TestVerifyStreamRespectsMediaSourceAuthorization(t *testing.T) {
	api := NewPlatformAPI(testConfig(t), previewStreamSource{})
	api.SetAccess("netease", func(_ context.Context, token string) bool { return token != "" })
	err := api.VerifyStream(context.Background(), "netease", "first")
	var denied *APIError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, 403, denied.Status)
	source, err := api.MediaSourceFor(context.Background(), "netease", "authorized-session")
	require.NoError(t, err)
	_, err = source.StreamURL(context.Background(), "first")
	var streamErr *platform.StreamResolutionError
	require.ErrorAs(t, err, &streamErr)
	require.Equal(t, platform.StreamTrialOnly, streamErr.Kind)
}
