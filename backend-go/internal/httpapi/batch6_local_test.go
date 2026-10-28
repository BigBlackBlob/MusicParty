package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestBatch6DisabledLocalRoutes(t *testing.T) {
	cfg := testConfig(t)
	cfg.LocalLibrary.Enabled = false
	store := httpStore(t)
	api := NewMediaAPI(cfg, account.New(store), store, nil, NewPlatformAPI(cfg), http.DefaultClient, nil)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/local/tracks"}, {"POST", "/api/local/tracks/upload"},
		{"PATCH", "/api/local/tracks/track"}, {"DELETE", "/api/local/tracks/track"},
		{"GET", "/api/local/media/track"}, {"HEAD", "/api/local/media/track"},
		{"GET", "/api/local/cover/track"}, {"GET", "/api/local/upload-access"},
		{"POST", "/api/local/upload-access/grant"}, {"POST", "/api/local/upload-access/revoke"},
	} {
		t.Run(route.method+route.path, func(t *testing.T) {
			w := batch2Request(handler, route.method, route.path, "", "", "")
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}

func TestBatch6MetadataCannotResurrectDeletedTrack(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "admin-token", "admin", "ADMIN")
	cfg := testConfig(t)
	cfg.LocalLibrary.Enabled = true
	repository := storesqlite.NewLocalTrackRepository(store)
	transcoder := media.NewTranscoder("unused", 1, 1, time.Second)
	defer transcoder.Close()
	library, err := media.NewLocalLibrary(t.TempDir(), 1024, repository, transcoder)
	require.NoError(t, err)
	require.NoError(t, repository.Upsert(context.Background(), storesqlite.LocalTrack{ID: "deleted", Title: "old", Artists: []string{"a"}, Status: "DELETED", CreatedAt: 1, UpdatedAt: 1}))
	api := NewMediaAPI(cfg, accounts, store, library, NewPlatformAPI(cfg), http.DefaultClient, nil)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	w := batch2Request(handler, "PATCH", "/api/local/tracks/deleted", `{"title":"edited"}`, "admin-token", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	track, err := repository.FindByID(context.Background(), "deleted")
	require.NoError(t, err)
	require.Equal(t, "DELETED", track.Status)
	require.Equal(t, "old", track.Title)
}
