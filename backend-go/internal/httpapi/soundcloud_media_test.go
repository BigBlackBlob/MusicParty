package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

type soundCloudDownloadFixture struct {
	resourceFixture
	download func(context.Context, string, int64) error
}

func (f *soundCloudDownloadFixture) Download(ctx context.Context, _ string, path string, maxBytes int64) error {
	return f.download(ctx, path, maxBytes)
}

func TestSoundCloudStreamServesOnlySessionCache(t *testing.T) {
	const id = "54202938"
	for _, fail := range []bool{false, true} {
		name := "completed download supports Range"
		if fail {
			name = "failed download remains unavailable"
		}
		t.Run(name, func(t *testing.T) {
			cache, err := media.NewCache(t.TempDir(), 4096, nil, 1, 1, 0, 0)
			require.NoError(t, err)
			t.Cleanup(cache.Close)
			started, finish := make(chan struct{}), make(chan struct{})
			source := &soundCloudDownloadFixture{resourceFixture: resourceFixture{name: "soundcloud"}, download: func(ctx context.Context, path string, _ int64) error {
				if err := os.WriteFile(path+".part", []byte("0123456789"), 0o600); err != nil {
					return err
				}
				close(started)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-finish:
				}
				if fail {
					return errors.New("download rejected")
				}
				return os.Rename(path+".part", path)
			}}
			cfg := testConfig(t)
			platforms := NewPlatformAPI(cfg, source)
			api := NewMediaAPI(cfg, nil, nil, nil, platforms, http.DefaultClient, cache)
			api.SetSoundCloudCache(cache)
			platforms.SetSoundCloudCache(cache)
			request := func(bytes string) *httptest.ResponseRecorder {
				t.Helper()
				r := httptest.NewRequest(http.MethodGet, "/api/soundcloud/stream/"+id, nil)
				if bytes != "" {
					r.Header.Set("Range", bytes)
				}
				w := httptest.NewRecorder()
				err := api.soundCloudStreamFor(w, r, id)
				if err != nil {
					var apiError *APIError
					require.ErrorAs(t, err, &apiError)
					w.WriteHeader(apiError.Status)
				}
				return w
			}
			platforms.PreResolveMedia(context.Background(), platform.Music{ID: id, Platform: "soundcloud"}, cache)
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("download job did not start")
			}
			require.False(t, platforms.SoundCloudCached(id, cache))
			require.Equal(t, http.StatusNotFound, request("bytes=0-3").Code)
			close(finish)
			require.Eventually(t, func() bool { return !cache.IsInflight("soundcloud:" + id) }, time.Second, time.Millisecond)
			require.Zero(t, source.calls, "stream must never resolve an upstream URL")
			if fail {
				require.False(t, platforms.SoundCloudCached(id, cache))
				require.Equal(t, http.StatusNotFound, request("").Code)
				return
			}
			require.True(t, platforms.SoundCloudCached(id, cache))
			full := request("")
			require.Equal(t, http.StatusOK, full.Code)
			require.Equal(t, "audio/mp4", full.Header().Get("Content-Type"))
			require.Equal(t, "0123456789", full.Body.String())
			part := request("bytes=3-5")
			require.Equal(t, http.StatusPartialContent, part.Code)
			require.Equal(t, "bytes 3-5/10", part.Header().Get("Content-Range"))
			require.Equal(t, "345", part.Body.String())
		})
	}
}

// TestSoundCloudAudioNeverReachesTheGeneralCache guards the provider terms.
//
// SoundCloud only permits storing audio for the session that requested it, so
// a SoundCloud track must land in the session-scoped cache even when the caller
// hands over the general-purpose cache.
func TestSoundCloudAudioNeverReachesTheGeneralCache(t *testing.T) {
	const id = "54202938"
	general, err := media.NewCache(t.TempDir(), 4096, nil, 1, 1, 0, 0)
	require.NoError(t, err)
	t.Cleanup(general.Close)
	session, err := media.NewCache(t.TempDir(), 4096, nil, 1, 1, 0, 0)
	require.NoError(t, err)
	t.Cleanup(session.Close)
	started := make(chan struct{})
	source := &soundCloudDownloadFixture{resourceFixture: resourceFixture{name: "soundcloud"}, download: func(ctx context.Context, path string, _ int64) error {
		close(started)
		return os.WriteFile(path, []byte("0123456789"), 0o600)
	}}
	cfg := testConfig(t)
	platforms := NewPlatformAPI(cfg, source)
	platforms.SetSoundCloudCache(session)
	platforms.PreResolveMedia(context.Background(), platform.Music{ID: id, Platform: "soundcloud"}, general)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("download job did not start")
	}
	require.Eventually(t, func() bool { return platforms.SoundCloudCached(id, session) }, time.Second, time.Millisecond)
	require.False(t, platforms.SoundCloudCached(id, general), "SoundCloud audio must not be stored in the general-purpose cache")
}
