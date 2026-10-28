package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestLocalMediaRequiresSessionAndSupportsRange(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "media-token", "listener", "MEMBER")
	cfg := testConfig(t)
	root := t.TempDir()
	transcoder := media.NewTranscoder("unused", 1, 1, time.Second)
	defer transcoder.Close()
	library, err := media.NewLocalLibrary(root, 1024, storesqlite.NewLocalTrackRepository(store), transcoder)
	require.NoError(t, err)
	path := filepath.Join(root, "media", "track.ogg")
	require.NoError(t, os.WriteFile(path, []byte("0123456789"), 0o600))
	relative := "media/track.ogg"
	now := time.Now().UnixMilli()
	require.NoError(t, storesqlite.NewLocalTrackRepository(store).Upsert(context.Background(), storesqlite.LocalTrack{ID: "track", Title: "Track", Artists: []string{"Artist"}, OGGPath: &relative, Status: "COMPLETED", CreatedAt: now, UpdatedAt: now}))
	api := NewMediaAPI(cfg, accounts, store, library, NewPlatformAPI(cfg), http.DefaultClient, nil)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/local/media/track", nil))
	require.Equal(t, http.StatusForbidden, unauthorized.Code)

	legacy := httptest.NewRecorder()
	handler.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/local/media/track?token=media-token", nil))
	require.Equal(t, http.StatusForbidden, legacy.Code)

	r := httptest.NewRequest(http.MethodGet, "/api/local/media/track", nil)
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "media-token"})
	r.Header.Set("Range", "bytes=3-6")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, "3456", w.Body.String())
	require.Equal(t, "bytes 3-6/10", w.Header().Get("Content-Range"))
}

func TestPlatformMediaCacheHitStillRequiresCurrentAccess(t *testing.T) {
	root := t.TempDir()
	cache, err := media.NewCache(root, 1024, http.DefaultClient, 1, 1, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	key := "navidrome:track"
	digest := sha256.Sum256([]byte(key))
	contents := []byte("private cached media")
	require.NoError(t, os.WriteFile(filepath.Join(root, hex.EncodeToString(digest[:])+".media"), contents, 0o600))

	cfg := testConfig(t)
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "media-token", "listener", "MEMBER")
	platforms := NewPlatformAPI(cfg, &restrictedFixture{})
	platforms.SetAccess("navidrome", PlatformAccess(accounts, "listener"))
	api := NewMediaAPI(cfg, accounts, store, nil, platforms, http.DefaultClient, cache)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	allowed := batch2Request(handler, http.MethodGet, "/api/navidrome/stream/track", "", "media-token", "")
	require.Equal(t, http.StatusOK, allowed.Code)
	require.Equal(t, string(contents), allowed.Body.String())
	platforms.SetAccess("navidrome", PlatformAccess(accounts, "nobody"))
	for _, path := range []string{"/api/navidrome/stream/track", "/api/navidrome/cover/track"} {
		denied := batch2Request(handler, http.MethodGet, path, "", "media-token", "")
		require.Equal(t, http.StatusForbidden, denied.Code)
		require.NotContains(t, denied.Body.String(), string(contents))
	}
	_, exists := cache.Get(key)
	require.True(t, exists, "revocation must be enforced while the cache still exists")
	platforms.Unregister("navidrome")
	removed := batch2Request(handler, http.MethodGet, "/api/navidrome/stream/track", "", "media-token", "")
	require.Equal(t, http.StatusForbidden, removed.Code)
}
