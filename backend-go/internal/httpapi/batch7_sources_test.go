package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/subsonic"
	securitycompat "github.com/BigBlackBlob/MusicParty/backend-go/internal/security"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func batch7Admin(t *testing.T) (*AdminAPI, *storesqlite.Store, http.Handler) {
	t.Helper()
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "admin-token", "admin", "ADMIN")
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	cfg := testConfig(t)
	api := NewAdminAPI(store, accounts, platform.NewClient(time.Second), NewPlatformAPI(cfg))
	return api, store, NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
}

func TestBatch7FirstSecretInitializationUsesOneKey(t *testing.T) {
	api, store, _ := batch7Admin(t)
	start := make(chan struct{})
	results := make(chan string, 32)
	errors := make(chan error, 32)
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := api.siteSecret(context.Background())
			results <- value
			errors <- err
		}()
	}
	close(start)
	wg.Wait()
	stored, err := storesqlite.NewSiteSettingRepository(store).FindValue(context.Background(), "security.site-secret.v1")
	require.NoError(t, err)
	for range 32 {
		require.NoError(t, <-errors)
		require.Equal(t, *stored, <-results)
	}
}

func TestBatch7FailedBindingSaveRollsBackSource(t *testing.T) {
	_, store, handler := batch7Admin(t)
	w := batch2Request(handler, "POST", "/api/admin/subsonic-source", `{"roomId":"missing-room","id":"orphan","baseUrl":"https://example.invalid","username":"u","password":"p"}`, "admin-token", "")
	require.NotEqual(t, http.StatusOK, w.Code)
	_, err := storesqlite.NewSubsonicSourceRepository(store).FindByID(context.Background(), "orphan")
	require.ErrorIs(t, err, sql.ErrNoRows, "failed binding must not leave a source")
}

func TestBatch7RemovingOneSharedBindingKeepsOtherRooms(t *testing.T) {
	_, store, handler := batch7Admin(t)
	insertStage5Room(t, store, "other", "PUBLIC", 0)
	repository := storesqlite.NewSubsonicSourceRepository(store)
	require.NoError(t, repository.Upsert(context.Background(), storesqlite.SubsonicSource{ID: "shared", Label: "Shared", BaseURL: "https://example.invalid", Username: "u", Password: "p", Client: "musicparty", APIVersion: "1.16.1", Enabled: true, CreatedAt: 1, UpdatedAt: 1}))
	for _, room := range []string{"lounge", "other"} {
		require.NoError(t, repository.UpsertRoomBinding(context.Background(), storesqlite.RoomSubsonicSource{RoomID: room, SourceID: "shared", Enabled: true, CreatedAt: 1, UpdatedAt: 1}))
	}
	w := batch2Request(handler, "POST", "/api/admin/subsonic-source/remove", `{"roomId":"lounge","id":"shared"}`, "admin-token", "")
	require.Equal(t, http.StatusOK, w.Code)
	_, err := repository.FindByID(context.Background(), "shared")
	require.NoError(t, err, "another room still owns a binding")
	_, err = repository.FindRoomBinding(context.Background(), "other", "shared")
	require.NoError(t, err)
}

func TestBatch7SharedUpdatesMatchStartupAndKeepRoomOverrides(t *testing.T) {
	api, store, handler := batch7Admin(t)
	ctx := context.Background()
	insertStage5Room(t, store, "other", "PUBLIC", 0)
	insertStage5User(t, store, "other-token", "other-user", "MEMBER")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"subsonic-response":{"status":"failed","error":{"code":40,"message":"bad credentials"}}}`)
	}))
	defer upstream.Close()
	save := func(room, extra string) {
		t.Helper()
		body := fmt.Sprintf(`{"roomId":%q,"id":"shared","baseUrl":%q,"username":"u"%s}`, room, upstream.URL, extra)
		w := batch2Request(handler, "POST", "/api/admin/subsonic-source", body, "admin-token", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	save("lounge", `,"password":"first","label":"Lounge label","allowedUsers":"admin","sortOrder":7`)
	save("other", `,"label":"Other label","allowedUsers":"other-user","sortOrder":12`)
	// Changing a room toggle must not disable the global source or other rooms.
	save("lounge", `,"password":"second","enabled":false`)
	_, available := api.platforms.Service("subsonic-shared@lounge")
	require.False(t, available)
	other, available := api.platforms.Service("subsonic-shared@other")
	require.True(t, available)
	mediaSource := other.(platform.MediaSource)
	streamURL, err := mediaSource.StreamURL(ctx, "song")
	require.NoError(t, err)
	parsed, err := url.Parse(streamURL)
	require.NoError(t, err)
	require.Equal(t, upstream.Listener.Addr().String(), parsed.Host)
	require.True(t, api.platforms.PlatformAllowed(ctx, other.Name(), "other-token"))
	require.False(t, api.platforms.PlatformAllowed(ctx, other.Name(), "admin-token"))
	repository := storesqlite.NewSubsonicSourceRepository(store)
	sources, err := repository.FindAll(ctx)
	require.NoError(t, err)
	bindings, err := repository.FindAllRoomBindings(ctx)
	require.NoError(t, err)
	secret, err := api.siteSecret(ctx)
	require.NoError(t, err)
	password, err := securitycompat.DecryptSubsonicCredential(sources[0].Password, secret)
	require.NoError(t, err)
	require.Equal(t, "second", password)
	startup, access, err := SubsonicServices(api.client, sources, bindings, secret)
	require.NoError(t, err)
	require.Len(t, startup, 2)
	var startupOther platform.Service
	for _, service := range startup {
		if service.Name() == other.Name() {
			startupOther = service
		} else {
			require.False(t, service.Available(), "disabled binding must shadow an environment fallback")
		}
	}
	require.NotNil(t, startupOther)
	fallback := subsonic.New(api.client, subsonic.Config{ID: "subsonic-shared@lounge", BaseURL: upstream.URL, Username: "u", Password: "environment", Enabled: true})
	restarted := NewPlatformAPI(testConfig(t), fallback)
	for _, service := range startup {
		restarted.Register(service)
	}
	_, available = restarted.Service(fallback.Name())
	require.False(t, available, "a disabled persisted binding must not revive the environment fallback")
	startupURL, err := startupOther.(platform.MediaSource).StreamURL(ctx, "song")
	require.NoError(t, err)
	require.Equal(t, streamURL, startupURL, "all bound rooms must receive updated credentials before restart")
	require.Equal(t, "other-user", access[other.Name()])
	for _, binding := range bindings {
		if binding.RoomID == "lounge" {
			require.Equal(t, 7, binding.SortOrder)
			require.Equal(t, "Lounge label", *binding.DisplayLabel)
		} else {
			require.Equal(t, 12, binding.SortOrder)
			require.Equal(t, "Other label", *binding.DisplayLabel)
		}
	}
	w := batch2Request(handler, "POST", "/api/admin/subsonic-source/test", `{"id":"shared"}`, "admin-token", "")
	require.Equal(t, http.StatusBadGateway, w.Code, "HTTP 200 protocol failures must fail the admin test")
	require.Contains(t, w.Body.String(), "40")
}

func TestBatch7SourceUpdateFailureKeepsDatabaseAndRegistry(t *testing.T) {
	api, store, handler := batch7Admin(t)
	body := `{"id":"stable","baseUrl":"https://original.invalid","username":"u","password":"old"}`
	w := batch2Request(handler, "POST", "/api/admin/subsonic-source", body, "admin-token", "")
	require.Equal(t, http.StatusOK, w.Code)
	before, ok := api.platforms.Service("subsonic-stable@lounge")
	require.True(t, ok)
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create trigger reject_binding_update before update on room_subsonic_source begin select raise(abort,'forced binding failure'); end`)
		return err
	}))
	body = `{"id":"stable","baseUrl":"https://changed.invalid","username":"u","password":"new"}`
	w = batch2Request(handler, "POST", "/api/admin/subsonic-source", body, "admin-token", "")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	current, ok := api.platforms.Service("subsonic-stable@lounge")
	require.True(t, ok)
	require.Same(t, before, current)
	source, err := storesqlite.NewSubsonicSourceRepository(store).FindByID(context.Background(), "stable")
	require.NoError(t, err)
	require.Equal(t, "https://original.invalid", source.BaseURL)
}

func TestBatch7ConcurrentSavesPublishCommittedCredentials(t *testing.T) {
	api, store, handler := batch7Admin(t)
	var wg sync.WaitGroup
	errors := make(chan int, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(`{"id":"shared","baseUrl":"https://example.invalid","username":"u","password":"password-%d"}`, i)
			w := batch2Request(handler, "POST", "/api/admin/subsonic-source", body, "admin-token", "")
			errors <- w.Code
		}()
	}
	wg.Wait()
	for range 20 {
		require.Equal(t, http.StatusOK, <-errors)
	}
	repository := storesqlite.NewSubsonicSourceRepository(store)
	sources, err := repository.FindAll(context.Background())
	require.NoError(t, err)
	bindings, err := repository.FindAllRoomBindings(context.Background())
	require.NoError(t, err)
	secret, err := api.siteSecret(context.Background())
	require.NoError(t, err)
	startup, _, err := SubsonicServices(api.client, sources, bindings, secret)
	require.NoError(t, err)
	require.Len(t, startup, 1)
	live, ok := api.platforms.Service(startup[0].Name())
	require.True(t, ok)
	left, err := startup[0].(platform.MediaSource).StreamURL(context.Background(), "song")
	require.NoError(t, err)
	right, err := live.(platform.MediaSource).StreamURL(context.Background(), "song")
	require.NoError(t, err)
	require.Equal(t, left, right)
}

func TestBatch7UnreadableCredentialsFailStartupWithoutChangingData(t *testing.T) {
	api, store, _ := batch7Admin(t)
	ctx := context.Background()
	encrypted, err := securitycompat.EncryptSubsonicCredential("password", "original-key")
	require.NoError(t, err)
	source := storesqlite.SubsonicSource{ID: "broken", Label: "Broken", Password: encrypted}
	require.NoError(t, storesqlite.NewSubsonicSourceRepository(store).Upsert(ctx, source))
	_, _, err = SubsonicServices(api.client, []storesqlite.SubsonicSource{source}, nil, "wrong-key")
	require.ErrorContains(t, err, "broken")
	_, err = api.siteSecret(ctx)
	require.Error(t, err, "missing key must not be replaced when ciphertext already exists")
	_, err = storesqlite.NewSiteSettingRepository(store).FindValue(ctx, "security.site-secret.v1")
	require.ErrorIs(t, err, sql.ErrNoRows)
	saved, err := storesqlite.NewSubsonicSourceRepository(store).FindByID(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, encrypted, saved.Password)
}
