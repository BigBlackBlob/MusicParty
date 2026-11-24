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

type youtubeDownloadFixture struct {
	resourceFixture
	download func(context.Context, string, int64) error
}

func (f *youtubeDownloadFixture) Download(ctx context.Context, _ string, path string, maxBytes int64) error {
	return f.download(ctx, path, maxBytes)
}

func TestYouTubeStreamServesOnlyCommittedDownload(t *testing.T) {
	const id = "0Ojarj6nTM4"
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
			source := &youtubeDownloadFixture{resourceFixture: resourceFixture{name: "youtube"}, download: func(ctx context.Context, path string, _ int64) error {
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
			request := func(bytes string) *httptest.ResponseRecorder {
				t.Helper()
				r := httptest.NewRequest(http.MethodGet, "/api/youtube/stream/"+id, nil)
				if bytes != "" {
					r.Header.Set("Range", bytes)
				}
				w := httptest.NewRecorder()
				require.NoError(t, api.proxyPlatform(w, r, "youtube", id, false))
				return w
			}
			platforms.PreResolveMedia(context.Background(), platform.Music{ID: id, Platform: "youtube"}, cache)
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("download job did not start")
			}
			require.False(t, platforms.YouTubeCached(id, cache))
			require.Equal(t, http.StatusNotFound, request("bytes=0-3").Code)
			close(finish)
			require.Eventually(t, func() bool { return !cache.IsInflight("youtube:" + id) }, time.Second, time.Millisecond)
			require.Zero(t, source.calls, "stream must never resolve an upstream URL")
			if fail {
				require.False(t, platforms.YouTubeCached(id, cache))
				require.Equal(t, http.StatusNotFound, request("").Code)
				return
			}
			require.True(t, platforms.YouTubeCached(id, cache))
			full := request("")
			require.Equal(t, http.StatusOK, full.Code)
			require.Equal(t, "audio/mp4", full.Header().Get("Content-Type"))
			require.Equal(t, "0123456789", full.Body.String())
			part := request("bytes=3-5")
			require.Equal(t, http.StatusPartialContent, part.Code)
			require.Equal(t, "bytes 3-5/10", part.Header().Get("Content-Range"))
			require.Equal(t, "345", part.Body.String())
			invalid := request("bytes=10-")
			require.Equal(t, http.StatusRequestedRangeNotSatisfiable, invalid.Code)
			require.Equal(t, "bytes */10", invalid.Header().Get("Content-Range"))
		})
	}
}
