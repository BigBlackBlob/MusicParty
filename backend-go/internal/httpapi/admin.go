package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/subsonic"
	securitycompat "github.com/BigBlackBlob/MusicParty/backend-go/internal/security"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/go-chi/chi/v5"
)

type AdminAPI struct {
	store     *storesqlite.Store
	accounts  *account.Service
	client    *platform.Client
	platforms *PlatformAPI
	sourcesMu sync.Mutex // Serializes source persistence and its post-commit registry publication.
}

func NewAdminAPI(store *storesqlite.Store, accounts *account.Service, client *platform.Client, platforms *PlatformAPI) *AdminAPI {
	return &AdminAPI{store: store, accounts: accounts, client: client, platforms: platforms}
}
func (api *AdminAPI) Routes(r chi.Router) {
	r.With(RequireAdmin(api.accounts)).Get("/api/admin/subsonic-sources", Adapt(api.listSources))
	r.With(RequireAdmin(api.accounts)).Post("/api/admin/subsonic-source", Adapt(api.saveSource))
	r.With(RequireAdmin(api.accounts)).Post("/api/admin/subsonic-source/order", Adapt(api.orderSource))
	r.With(RequireAdmin(api.accounts)).Post("/api/admin/subsonic-source/remove", Adapt(api.removeSource))
	r.With(RequireAdmin(api.accounts)).Post("/api/admin/subsonic-source/test", Adapt(api.testSource))
	r.With(RequireAdmin(api.accounts)).Post("/api/admin/navidrome-access/grant", Adapt(api.grantNavidrome))
	r.With(RequireAdmin(api.accounts)).Post("/api/admin/navidrome-access/revoke", Adapt(api.revokeNavidrome))
	r.With(RequireAdmin(api.accounts)).Post("/api/admin/platform-credentials", Adapt(api.updatePlatformCredential))
}

type sourceRequest struct {
	RoomID       string  `json:"roomId"`
	ID           string  `json:"id"`
	Label        *string `json:"label"`
	BaseURL      string  `json:"baseUrl"`
	Username     string  `json:"username"`
	Password     string  `json:"password"`
	AllowedUsers *string `json:"allowedUsers"`
	Enabled      *bool   `json:"enabled"`
	SortOrder    *int    `json:"sortOrder"`
}
type platformCredentialRequest struct {
	Platform   string `json:"platform"`
	Credential string `json:"credential"`
}

func (api *AdminAPI) admin(r *http.Request) bool {
	session, err := api.accounts.Resolve(r.Context(), sessionToken(r))
	return err == nil && session.Admin()
}
func (api *AdminAPI) updatePlatformCredential(w http.ResponseWriter, r *http.Request) error {
	var request platformCredentialRequest
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r) {
		return accessDenied(w)
	}
	platformName := strings.ToLower(strings.TrimSpace(request.Platform))
	if platformName != "netease" && platformName != "bilibili" {
		return badRequest("Unsupported platform")
	}
	credential := strings.TrimSpace(request.Credential)
	if credential == "" || len(credential) > 16*1024 {
		return badRequest("Credential must be between 1 and 16384 bytes")
	}
	if !api.platforms.SupportsCredentialUpdate(platformName) {
		return badRequest("Platform credential updates are not supported")
	}
	key := "netease.cookie"
	if platformName == "bilibili" {
		key = "bilibili.sessdata"
	}
	if err := storesqlite.NewSiteSettingRepository(api.store).Upsert(r.Context(), storesqlite.SiteSetting{Key: key, Value: &credential, Secret: true, UpdatedAt: time.Now().UnixMilli()}); err != nil {
		return err
	}
	if err := api.platforms.UpdateCredential(platformName, credential); err != nil {
		return err
	}
	writeJSON(w, map[string]string{"message": "PLATFORM CREDENTIAL UPDATED", "platform": platformName})
	return nil
}
func (api *AdminAPI) listSources(w http.ResponseWriter, r *http.Request) error {
	if !api.admin(r) {
		return accessDenied(w)
	}
	roomID := defaultValue(r.URL.Query().Get("roomId"), "lounge")
	repository := storesqlite.NewSubsonicSourceRepository(api.store)
	sources, err := repository.FindAll(r.Context())
	if err != nil {
		return err
	}
	bindings, err := repository.FindRoomBindings(r.Context(), roomID)
	if err != nil {
		return err
	}
	byID := map[string]storesqlite.SubsonicSource{}
	for _, source := range sources {
		byID[source.ID] = source
	}
	out := []map[string]any{}
	for _, binding := range bindings {
		source, ok := byID[binding.SourceID]
		if !ok {
			continue
		}
		label := source.Label
		if binding.DisplayLabel != nil {
			label = *binding.DisplayLabel
		}
		allowed := ""
		if source.AllowedUsers != nil {
			allowed = *source.AllowedUsers
		}
		if binding.AllowedUsers != nil {
			allowed = *binding.AllowedUsers
		}
		out = append(out, map[string]any{"id": source.ID, "platformId": "subsonic-" + source.ID, "label": label, "baseUrl": source.BaseURL, "username": source.Username, "allowedUsers": allowed, "enabled": binding.Enabled, "active": binding.Enabled && source.Enabled && source.BaseURL != "" && source.Username != "" && source.Password != "", "system": source.System, "sortOrder": binding.SortOrder, "updatedAt": binding.UpdatedAt})
	}
	writeJSON(w, out)
	return nil
}
func (api *AdminAPI) saveSource(w http.ResponseWriter, r *http.Request) error {
	var request sourceRequest
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r) {
		return accessDenied(w)
	}
	request.RoomID = defaultValue(request.RoomID, "lounge")
	request.ID = strings.ToLower(strings.TrimSpace(request.ID))
	if request.ID == "" || request.BaseURL == "" || request.Username == "" {
		return badRequest("id, baseUrl and username are required")
	}
	api.sourcesMu.Lock()
	defer api.sourcesMu.Unlock()
	if err := r.Context().Err(); err != nil {
		return err
	}
	ctx := context.WithoutCancel(r.Context())
	repository := storesqlite.NewSubsonicSourceRepository(api.store)
	existing, err := repository.FindByID(ctx, request.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	password := request.Password
	if password == "" && existing != nil {
		password = existing.Password
	}
	secret, err := api.siteSecret(ctx)
	if err != nil {
		return err
	}
	password, err = securitycompat.EncryptSubsonicCredential(password, secret)
	if err != nil {
		return err
	}
	runtimePassword, err := securitycompat.DecryptSubsonicCredential(password, secret)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	source := storesqlite.SubsonicSource{ID: request.ID, Label: defaultValue(dereference(request.Label), request.ID), Client: "musicparty", APIVersion: "1.16.1", AllowedUsers: stringPointer("*"), Enabled: true, CreatedAt: now}
	if existing != nil {
		source = *existing
	}
	source.BaseURL, source.Username, source.Password, source.UpdatedAt = strings.TrimRight(request.BaseURL, "/"), request.Username, password, now
	binding := storesqlite.RoomSubsonicSource{RoomID: request.RoomID, SourceID: source.ID, Enabled: true, CreatedAt: now}
	previous, err := repository.FindRoomBinding(ctx, request.RoomID, request.ID)
	if err == nil {
		binding = *previous
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if request.Enabled != nil {
		binding.Enabled = *request.Enabled
	}
	if request.SortOrder != nil {
		binding.SortOrder = *request.SortOrder
	}
	if request.Label != nil {
		binding.DisplayLabel = stringPointer(*request.Label)
	}
	if request.AllowedUsers != nil {
		binding.AllowedUsers = request.AllowedUsers
	}
	binding.UpdatedAt = now
	bindings, err := repository.SaveWithBinding(ctx, source, binding)
	if err != nil {
		return err
	}
	api.platforms.replaceSubsonicSource(api.client, api.accounts, source, bindings, runtimePassword)
	writeJSON(w, map[string]string{"message": "SOURCE SAVED", "platformId": "subsonic-" + source.ID})
	return nil
}
func (api *AdminAPI) orderSource(w http.ResponseWriter, r *http.Request) error {
	var request sourceRequest
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r) {
		return accessDenied(w)
	}
	if request.SortOrder == nil {
		return badRequest("sortOrder is required")
	}
	api.sourcesMu.Lock()
	defer api.sourcesMu.Unlock()
	repository := storesqlite.NewSubsonicSourceRepository(api.store)
	binding, err := repository.FindRoomBinding(r.Context(), defaultValue(request.RoomID, "lounge"), request.ID)
	if err != nil {
		return badRequest("Source not found")
	}
	binding.SortOrder = *request.SortOrder
	binding.UpdatedAt = time.Now().UnixMilli()
	if err := repository.UpsertRoomBinding(r.Context(), *binding); err != nil {
		return err
	}
	writeJSON(w, map[string]any{"message": "SOURCE ORDER UPDATED"})
	return nil
}
func (api *AdminAPI) removeSource(w http.ResponseWriter, r *http.Request) error {
	var request sourceRequest
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r) {
		return accessDenied(w)
	}
	api.sourcesMu.Lock()
	defer api.sourcesMu.Unlock()
	if err := r.Context().Err(); err != nil {
		return err
	}
	ctx := context.WithoutCancel(r.Context())
	repository := storesqlite.NewSubsonicSourceRepository(api.store)
	if err := repository.RemoveBindingAndUnusedSource(ctx, defaultValue(request.RoomID, "lounge"), request.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return badRequest("Source not found in this Lounge")
		}
		return err
	}
	api.platforms.Unregister("subsonic-" + request.ID + "@" + defaultValue(request.RoomID, "lounge"))
	writeJSON(w, map[string]string{"message": "SOURCE REMOVED"})
	return nil
}
func (api *AdminAPI) testSource(w http.ResponseWriter, r *http.Request) error {
	var request sourceRequest
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r) {
		return accessDenied(w)
	}
	repository := storesqlite.NewSubsonicSourceRepository(api.store)
	source, err := repository.FindByID(r.Context(), request.ID)
	if err != nil {
		return badRequest("Source not found")
	}
	secret, err := api.siteSecret(r.Context())
	if err != nil {
		return err
	}
	password, err := securitycompat.DecryptSubsonicCredential(source.Password, secret)
	if err != nil {
		return err
	}
	service := subsonic.New(api.client, subsonic.Config{ID: source.ID, BaseURL: source.BaseURL, Username: source.Username, Password: password, Client: source.Client, Version: source.APIVersion, Enabled: true})
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	if _, err := service.Search(ctx, "musicparty-connectivity-test", 0, 1); err != nil {
		writeErrorJSON(w, http.StatusBadGateway, map[string]string{"message": "Subsonic test failed: " + err.Error()})
		return nil
	}
	writeJSON(w, map[string]string{"message": "SUBSONIC SOURCE OK: " + source.Label})
	return nil
}
func (api *AdminAPI) grantNavidrome(w http.ResponseWriter, r *http.Request) error {
	return api.updateNavidrome(w, r, true)
}
func (api *AdminAPI) revokeNavidrome(w http.ResponseWriter, r *http.Request) error {
	return api.updateNavidrome(w, r, false)
}
func (api *AdminAPI) updateNavidrome(w http.ResponseWriter, r *http.Request, grant bool) error {
	var request struct {
		UserName string `json:"userName"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r) {
		return accessDenied(w)
	}
	repository := storesqlite.NewSiteSettingRepository(api.store)
	setting, err := repository.FindValue(r.Context(), "navidrome.allowed-users")
	current := ""
	if err == nil && setting != nil {
		current = *setting
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	names := map[string]struct{}{}
	for _, name := range strings.Split(current, ",") {
		if value := strings.ToLower(strings.TrimSpace(name)); value != "" {
			names[value] = struct{}{}
		}
	}
	name := strings.ToLower(strings.TrimSpace(request.UserName))
	if grant {
		names[name] = struct{}{}
	} else {
		delete(names, name)
	}
	values := make([]string, 0, len(names))
	for value := range names {
		values = append(values, value)
	}
	joined := strings.Join(values, ",")
	if err := repository.Upsert(r.Context(), storesqlite.SiteSetting{Key: "navidrome.allowed-users", Value: &joined, UpdatedAt: time.Now().UnixMilli()}); err != nil {
		return err
	}
	api.platforms.SetAccess("navidrome", PlatformAccess(api.accounts, joined))
	writeJSON(w, map[string]string{"message": "NAVIDROME ACCESS UPDATED"})
	return nil
}
func (api *AdminAPI) siteSecret(ctx context.Context) (string, error) {
	repository := storesqlite.NewSiteSettingRepository(api.store)
	value, err := repository.FindValue(ctx, "security.site-secret.v1")
	if err == nil {
		if value == nil || *value == "" {
			return "", errors.New("existing site secret is empty; repair the setting before saving credentials")
		}
		return *value, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	secret := base64.StdEncoding.EncodeToString(bytes)
	return repository.InitializeSecret(ctx, "security.site-secret.v1", secret, time.Now().UnixMilli())
}
func accessDenied(w http.ResponseWriter) error {
	writeErrorJSON(w, 403, map[string]string{"message": "ACCESS DENIED"})
	return nil
}
func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
