package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/go-chi/chi/v5"
)

type PlatformAPI struct {
	cfg      config.Config
	mu       sync.RWMutex
	services map[string]platform.Service
	access   map[string]func(context.Context, string) bool
	cacheMu  sync.Mutex
	cache    map[string]cachedPlayable
}

type cachedPlayable struct {
	value     platform.PlayableMusic
	expiresAt time.Time
}

func NewPlatformAPI(cfg config.Config, services ...platform.Service) *PlatformAPI {
	m := make(map[string]platform.Service, len(services))
	for _, service := range services {
		if service != nil {
			m[service.Name()] = service
		}
	}
	return &PlatformAPI{cfg: cfg, services: m, access: map[string]func(context.Context, string) bool{}, cache: map[string]cachedPlayable{}}
}
func (api *PlatformAPI) SetAccess(platformName string, check func(context.Context, string) bool) {
	api.mu.Lock()
	defer api.mu.Unlock()
	if check != nil {
		api.access[platformName] = check
	}
}
func (api *PlatformAPI) Register(service platform.Service) {
	api.mu.Lock()
	defer api.mu.Unlock()
	if service != nil {
		api.services[service.Name()] = service
	}
}
func (api *PlatformAPI) Unregister(name string) {
	api.mu.Lock()
	delete(api.services, name)
	delete(api.access, name)
	api.mu.Unlock()
}
func (api *PlatformAPI) Service(name string) (platform.Service, bool) {
	api.mu.RLock()
	service := api.services[name]
	api.mu.RUnlock()
	return service, service != nil && service.Available()
}
func (api *PlatformAPI) SupportsCredentialUpdate(name string) bool {
	api.mu.RLock()
	service := api.services[name]
	api.mu.RUnlock()
	_, ok := service.(platform.CredentialUpdater)
	return ok
}
func (api *PlatformAPI) UpdateCredential(name, credential string) error {
	api.mu.RLock()
	service := api.services[name]
	api.mu.RUnlock()
	updater, ok := service.(platform.CredentialUpdater)
	if !ok {
		return fmt.Errorf("platform credential updates are not supported")
	}
	updater.UpdateCredential(credential)
	api.cacheMu.Lock()
	for key := range api.cache {
		if strings.HasPrefix(key, name+":") {
			delete(api.cache, key)
		}
	}
	api.cacheMu.Unlock()
	return nil
}
func (api *PlatformAPI) ResolvePlayable(ctx context.Context, platformName, musicID, sessionToken string) (platform.PlayableMusic, error) {
	service, err := api.service(platformName)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	api.mu.RLock()
	check := api.access[platformName]
	api.mu.RUnlock()
	if check != nil && !check(ctx, sessionToken) {
		return platform.PlayableMusic{}, &APIError{Status: http.StatusForbidden, Message: "Forbidden"}
	}
	key := platformName + ":" + musicID
	now := time.Now()
	api.cacheMu.Lock()
	if cached, ok := api.cache[key]; ok && now.Before(cached.expiresAt) {
		api.cacheMu.Unlock()
		return cached.value, nil
	}
	api.cacheMu.Unlock()
	resolver, ok := service.(platform.PlayableResolver)
	if !ok {
		return platform.PlayableMusic{}, fmt.Errorf("platform %s does not resolve playable tracks", platformName)
	}
	resolved, err := resolver.ResolvePlayable(ctx, musicID)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	api.cacheMu.Lock()
	if len(api.cache) >= 2048 {
		clear(api.cache)
	}
	api.cache[key] = cachedPlayable{value: resolved, expiresAt: now.Add(5 * time.Minute)}
	api.cacheMu.Unlock()
	return resolved, nil
}

func (api *PlatformAPI) Routes(r chi.Router) {
	r.Get("/api/config", api.config)
	r.Get("/api/platforms", api.platforms)
	r.Get("/api/search/{platform}/{keyword}", Adapt(api.search))
	r.Get("/api/user/playlists/{platform}/{userId}", Adapt(api.userPlaylists))
	r.Get("/api/playlist/songs/{platform}/{playlistId}", Adapt(api.playlistSongs))
	r.Get("/api/album/search/{platform}", Adapt(api.searchAlbums))
	r.Get("/api/album/songs/{platform}/{albumId}", Adapt(api.albumSongs))
	r.Get("/api/user/search/{platform}/{keyword}", Adapt(api.searchUsers))
	r.Get("/api/music/lyric/{platform}/{musicId}", Adapt(api.lyric))
	r.Get("/api/music/lyric-detail/{platform}/{musicId}", Adapt(api.lyricDetail))
	r.Get("/api/theme/extract-cover-color", Adapt(api.coverColor))
}
func (api *PlatformAPI) config(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"authorName": api.cfg.Application.AuthorName, "backWords": api.cfg.Application.BackWords})
}
func (api *PlatformAPI) platforms(w http.ResponseWriter, r *http.Request) {
	type item struct {
		ID                  string `json:"id"`
		Label               string `json:"label"`
		SupportsAlbumSearch bool   `json:"supportsAlbumSearch"`
		Subsonic            bool   `json:"subsonic"`
	}
	api.mu.RLock()
	services := make(map[string]platform.Service, len(api.services))
	for name, service := range api.services {
		services[name] = service
	}
	access := make(map[string]func(context.Context, string) bool, len(api.access))
	for name, check := range api.access {
		access[name] = check
	}
	api.mu.RUnlock()
	out := []item{{"netease", "netease", true, false}, {"bilibili", "bilibili", false, false}}
	for _, name := range []string{"youtube", "local", "navidrome", "squidify"} {
		if service := services[name]; service != nil && service.Available() {
			if check := access[name]; check != nil && !check(r.Context(), r.URL.Query().Get("token")) {
				continue
			}
			out = append(out, item{name, name, name == "navidrome" || name == "squidify", name == "navidrome" || name == "squidify"})
		}
	}
	roomID := defaultValue(r.URL.Query().Get("roomId"), "lounge")
	names := make([]string, 0, len(services))
	for name := range services {
		if strings.HasPrefix(name, "subsonic-") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		service := services[name]
		roomAware, ok := service.(interface{ RoomID() string })
		if ok && roomAware.RoomID() != roomID {
			continue
		}
		if !service.Available() {
			continue
		}
		if check := access[name]; check != nil && !check(r.Context(), r.URL.Query().Get("token")) {
			continue
		}
		label := name
		if labeled, ok := service.(interface{ Label() string }); ok {
			label = labeled.Label()
		}
		out = append(out, item{name, label, true, true})
	}
	writeJSON(w, out)
}
func (api *PlatformAPI) service(name string) (platform.Service, error) {
	api.mu.RLock()
	service := api.services[name]
	api.mu.RUnlock()
	if service == nil || !service.Available() {
		return nil, &APIError{Status: http.StatusInternalServerError, Message: "Platform not supported: " + name}
	}
	return service, nil
}
func (api *PlatformAPI) authorizedService(r *http.Request) (platform.Service, error) {
	name := chi.URLParam(r, "platform")
	service, err := api.service(name)
	if err != nil {
		return nil, err
	}
	api.mu.RLock()
	check := api.access[name]
	api.mu.RUnlock()
	if check != nil && !check(r.Context(), r.URL.Query().Get("token")) {
		return nil, &APIError{Status: http.StatusForbidden, Message: "Forbidden"}
	}
	return service, nil
}
func queryPage(r *http.Request) (int, int, error) {
	offset, err := strconv.Atoi(defaultValue(r.URL.Query().Get("offset"), "0"))
	if err != nil {
		return 0, 0, err
	}
	limit, err := strconv.Atoi(defaultValue(r.URL.Query().Get("limit"), "20"))
	return offset, limit, err
}
func (api *PlatformAPI) search(w http.ResponseWriter, r *http.Request) error {
	keyword := chi.URLParam(r, "keyword")
	offset, limit, err := queryPage(r)
	if err != nil || len(keyword) > 128 || offset < 0 || offset > 10000 || limit < 1 || limit > 100 {
		return &APIError{Status: http.StatusInternalServerError, Message: "An unexpected internal server error occurred.", Name: "ResponseStatusException"}
	}
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	result, err := service.Search(r.Context(), keyword, offset, limit)
	if err != nil && chi.URLParam(r, "platform") == "netease" {
		return BadGateway("尚未配置网易云 Cookie，请联系管理员设置")
	}
	return writePlatformResult(w, result, err)
}
func (api *PlatformAPI) userPlaylists(w http.ResponseWriter, r *http.Request) error {
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	v, err := service.UserPlaylists(r.Context(), chi.URLParam(r, "userId"))
	return writePlatformResult(w, v, err)
}
func (api *PlatformAPI) playlistSongs(w http.ResponseWriter, r *http.Request) error {
	offset, limit, err := queryPage(r)
	if err != nil {
		return err
	}
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	v, err := service.PlaylistSongs(r.Context(), chi.URLParam(r, "playlistId"), offset, limit)
	if err != nil && chi.URLParam(r, "platform") == "netease" {
		return BadGateway("尚未配置网易云 Cookie，请联系管理员设置")
	}
	return writePlatformResult(w, v, err)
}
func (api *PlatformAPI) searchAlbums(w http.ResponseWriter, r *http.Request) error {
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	v, err := service.SearchAlbums(r.Context(), r.URL.Query().Get("keyword"))
	return writePlatformResult(w, v, err)
}
func (api *PlatformAPI) albumSongs(w http.ResponseWriter, r *http.Request) error {
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	v, err := service.AlbumSongs(r.Context(), chi.URLParam(r, "albumId"))
	return writePlatformResult(w, v, err)
}
func (api *PlatformAPI) searchUsers(w http.ResponseWriter, r *http.Request) error {
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	v, err := service.SearchUsers(r.Context(), chi.URLParam(r, "keyword"))
	return writePlatformResult(w, v, err)
}
func (api *PlatformAPI) lyric(w http.ResponseWriter, r *http.Request) error {
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	v, err := service.Lyric(r.Context(), chi.URLParam(r, "musicId"))
	if err != nil {
		return mapPlatformError(err)
	}
	w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
	_, err = w.Write([]byte(v.Lyric))
	return err
}
func (api *PlatformAPI) lyricDetail(w http.ResponseWriter, r *http.Request) error {
	service, err := api.authorizedService(r)
	if err != nil {
		return err
	}
	v, err := service.Lyric(r.Context(), chi.URLParam(r, "musicId"))
	return writePlatformResult(w, v, err)
}
func (api *PlatformAPI) coverColor(w http.ResponseWriter, r *http.Request) error {
	coverPath := r.URL.Query().Get("url")
	if !(strings.HasPrefix(coverPath, "/media/") || strings.HasPrefix(coverPath, "/api/navidrome/cover/") || (strings.HasPrefix(coverPath, "/api/subsonic/") && strings.Contains(coverPath, "/cover/"))) {
		return &APIError{Status: http.StatusBadRequest, Message: "Only proxied cover URLs are supported"}
	}
	base, err := url.Parse(api.cfg.Application.BaseURL)
	if err != nil {
		return err
	}
	target, err := base.Parse(coverPath)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	coverClient := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := coverClient.Do(request)
	if err != nil {
		return mapPlatformError(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 || response.StatusCode/100 == 3 {
		return BadGateway("Cover proxy request failed")
	}
	if !strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "image/") {
		return BadGateway("Cover response is not an image")
	}
	decoded, _, err := image.Decode(io.LimitReader(response.Body, 3<<20))
	if err != nil {
		return BadGateway("Cover image could not be decoded")
	}
	bounds := decoded.Bounds()
	stepX, stepY := max(1, bounds.Dx()/64), max(1, bounds.Dy()/64)
	var red, green, blue, count uint64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += stepY {
		for x := bounds.Min.X; x < bounds.Max.X; x += stepX {
			r, g, b, a := decoded.At(x, y).RGBA()
			if a < 0x4000 {
				continue
			}
			red += uint64(r >> 8)
			green += uint64(g >> 8)
			blue += uint64(b >> 8)
			count++
		}
	}
	if count == 0 {
		return BadGateway("Cover image has no visible pixels")
	}
	r8, g8, b8 := red/count, green/count, blue/count
	accent := fmt.Sprintf("#%02x%02x%02x", r8, g8, b8)
	writeJSON(w, map[string]string{"accent": accent, "accentHover": accent, "accentMuted": fmt.Sprintf("rgba(%d, %d, %d, 0.16)", r8, g8, b8), "accentSubtle": fmt.Sprintf("rgba(%d, %d, %d, 0.08)", r8, g8, b8)})
	return nil
}
func writePlatformResult(w http.ResponseWriter, value any, err error) error {
	if err != nil {
		return mapPlatformError(err)
	}
	writeJSON(w, value)
	return nil
}
func mapPlatformError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &APIError{Status: http.StatusGatewayTimeout, Message: "Upstream request timed out"}
	}
	return BadGateway("Upstream API request failed")
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
