package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
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
}

func NewAdminAPI(store *storesqlite.Store, accounts *account.Service, client *platform.Client, platforms *PlatformAPI) *AdminAPI {
	return &AdminAPI{store: store, accounts: accounts, client: client, platforms: platforms}
}
func (api *AdminAPI) Routes(r chi.Router) {
	r.Get("/api/admin/subsonic-sources", Adapt(api.listSources))
	r.Post("/api/admin/subsonic-source", Adapt(api.saveSource))
	r.Post("/api/admin/subsonic-source/order", Adapt(api.orderSource))
	r.Post("/api/admin/subsonic-source/remove", Adapt(api.removeSource))
	r.Post("/api/admin/subsonic-source/test", Adapt(api.testSource))
	r.Post("/api/admin/navidrome-access/grant", Adapt(api.grantNavidrome))
	r.Post("/api/admin/navidrome-access/revoke", Adapt(api.revokeNavidrome))
}

type sourceRequest struct {
	SessionToken string `json:"sessionToken"`
	RoomID       string `json:"roomId"`
	ID           string `json:"id"`
	Label        string `json:"label"`
	BaseURL      string `json:"baseUrl"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	AllowedUsers string `json:"allowedUsers"`
	Enabled      *bool  `json:"enabled"`
	SortOrder    *int   `json:"sortOrder"`
}

func (api *AdminAPI) admin(r *http.Request, requestToken string) bool {
	token := requestToken
	if token == "" {
		token = sessionToken(r)
	}
	session, err := api.accounts.Resolve(r.Context(), token)
	return err == nil && session.Admin()
}
func (api *AdminAPI) listSources(w http.ResponseWriter, r *http.Request) error {
	if !api.admin(r, r.URL.Query().Get("sessionToken")) {
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
	if !api.admin(r, request.SessionToken) {
		return accessDenied(w)
	}
	request.RoomID = defaultValue(request.RoomID, "lounge")
	request.ID = strings.ToLower(strings.TrimSpace(request.ID))
	if request.ID == "" || request.BaseURL == "" || request.Username == "" {
		return badRequest("id, baseUrl and username are required")
	}
	repository := storesqlite.NewSubsonicSourceRepository(api.store)
	existing, _ := repository.FindByID(r.Context(), request.ID)
	password := request.Password
	if password == "" && existing != nil {
		password = existing.Password
	}
	secret, err := api.siteSecret(r.Context())
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
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	source := storesqlite.SubsonicSource{ID: request.ID, Label: defaultValue(request.Label, request.ID), BaseURL: strings.TrimRight(request.BaseURL, "/"), Username: request.Username, Password: password, Client: "musicparty", APIVersion: "1.16.1", AllowedUsers: stringPointer(defaultValue(request.AllowedUsers, "*")), Enabled: enabled, System: existing != nil && existing.System, CreatedAt: now, UpdatedAt: now}
	if existing != nil {
		source.CreatedAt = existing.CreatedAt
		source.OwnerRoomID = existing.OwnerRoomID
	}
	if err := repository.Upsert(r.Context(), source); err != nil {
		return err
	}
	sortOrder := 0
	if request.SortOrder != nil {
		sortOrder = *request.SortOrder
	}
	binding := storesqlite.RoomSubsonicSource{RoomID: request.RoomID, SourceID: source.ID, Enabled: enabled, DisplayLabel: stringPointer(request.Label), AllowedUsers: stringPointer(request.AllowedUsers), SortOrder: sortOrder, CreatedAt: now, UpdatedAt: now}
	if err := repository.UpsertRoomBinding(r.Context(), binding); err != nil {
		return err
	}
	platformID := "subsonic-" + source.ID + "@" + request.RoomID
	api.platforms.Register(subsonic.New(api.client, subsonic.Config{ID: platformID, Label: source.Label, RoomID: request.RoomID, BaseURL: source.BaseURL, Username: source.Username, Password: runtimePassword, Client: source.Client, Version: source.APIVersion, ProxyPrefix: "/api/subsonic/" + request.RoomID + "/" + source.ID + "/cover", Enabled: enabled}))
	api.platforms.SetAccess(platformID, api.allowedAccess(defaultValue(request.AllowedUsers, "*")))
	writeJSON(w, map[string]string{"message": "SOURCE SAVED", "platformId": "subsonic-" + source.ID})
	return nil
}
func (api *AdminAPI) orderSource(w http.ResponseWriter, r *http.Request) error {
	var request sourceRequest
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r, request.SessionToken) {
		return accessDenied(w)
	}
	if request.SortOrder == nil {
		return badRequest("sortOrder is required")
	}
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
	if !api.admin(r, request.SessionToken) {
		return accessDenied(w)
	}
	repository := storesqlite.NewSubsonicSourceRepository(api.store)
	if _, err := repository.FindRoomBinding(r.Context(), defaultValue(request.RoomID, "lounge"), request.ID); err != nil {
		return badRequest("Source not found in this Lounge")
	}
	if err := repository.DeleteRoomBinding(r.Context(), defaultValue(request.RoomID, "lounge"), request.ID); err != nil {
		return err
	}
	api.platforms.Unregister("subsonic-" + request.ID + "@" + defaultValue(request.RoomID, "lounge"))
	source, err := repository.FindByID(r.Context(), request.ID)
	if err == nil && !source.System {
		_ = repository.Delete(r.Context(), request.ID)
	}
	writeJSON(w, map[string]string{"message": "SOURCE REMOVED"})
	return nil
}
func (api *AdminAPI) testSource(w http.ResponseWriter, r *http.Request) error {
	var request sourceRequest
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r, request.SessionToken) {
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
		SessionToken string `json:"sessionToken"`
		UserName     string `json:"userName"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if !api.admin(r, request.SessionToken) {
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
	api.platforms.SetAccess("navidrome", api.allowedAccess(joined))
	writeJSON(w, map[string]string{"message": "NAVIDROME ACCESS UPDATED"})
	return nil
}
func (api *AdminAPI) siteSecret(ctx context.Context) (string, error) {
	repository := storesqlite.NewSiteSettingRepository(api.store)
	value, err := repository.FindValue(ctx, "security.site-secret.v1")
	if err == nil && value != nil && *value != "" {
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
	if err := repository.Upsert(ctx, storesqlite.SiteSetting{Key: "security.site-secret.v1", Value: &secret, Secret: true, UpdatedAt: time.Now().UnixMilli()}); err != nil {
		return "", err
	}
	return secret, nil
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
func (api *AdminAPI) allowedAccess(raw string) func(context.Context, string) bool {
	allowed := map[string]struct{}{}
	for _, value := range strings.Split(raw, ",") {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	return func(ctx context.Context, token string) bool {
		session, err := api.accounts.Resolve(ctx, token)
		if err != nil {
			return false
		}
		if _, ok := allowed["*"]; ok {
			return true
		}
		_, byID := allowed[strings.ToLower(session.PublicID)]
		_, byName := allowed[strings.ToLower(session.Username)]
		return byID || byName
	}
}
