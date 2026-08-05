package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/go-chi/chi/v5"
)

type MediaAPI struct {
	cfg        config.Config
	accounts   *account.Service
	repository *storesqlite.LocalTrackRepository
	library    *media.LocalLibrary
	platforms  *PlatformAPI
	proxy      media.Proxy
	cache      *media.Cache
}

func NewMediaAPI(cfg config.Config, accounts *account.Service, store *storesqlite.Store, library *media.LocalLibrary, platforms *PlatformAPI, proxyClient *http.Client, cache *media.Cache) *MediaAPI {
	return &MediaAPI{cfg: cfg, accounts: accounts, repository: storesqlite.NewLocalTrackRepository(store), library: library, platforms: platforms, proxy: media.Proxy{Client: proxyClient}, cache: cache}
}

func (api *MediaAPI) Routes(r chi.Router) {
	r.Get("/api/local/tracks", Adapt(api.listLocal))
	r.Post("/api/local/tracks/upload", Adapt(api.uploadLocal))
	r.Patch("/api/local/tracks/{id}", Adapt(api.updateLocal))
	r.Delete("/api/local/tracks/{id}", Adapt(api.deleteLocal))
	r.Get("/api/local/upload-access", Adapt(api.listUploadAccess))
	r.Post("/api/local/upload-access/grant", Adapt(api.grantUploadAccess))
	r.Post("/api/local/upload-access/revoke", Adapt(api.revokeUploadAccess))
	r.Get("/api/local/media/{id}", Adapt(api.localMedia))
	r.Head("/api/local/media/{id}", Adapt(api.localMedia))
	r.Get("/api/local/cover/{id}", Adapt(api.localCover))
	r.Get("/api/netease/stream/{songId}", Adapt(api.neteaseStream))
	r.Get("/api/bilibili/stream/{bvid}", Adapt(api.bilibiliStream))
	r.Get("/api/youtube/stream/{videoId}", Adapt(api.youtubeStream))
	r.Get("/api/navidrome/stream/{songId}", Adapt(api.navidromeStream))
	r.Get("/api/navidrome/cover/{coverId}", Adapt(api.navidromeCover))
	r.Get("/api/subsonic/{sourceId}/stream/{songId}", Adapt(api.subsonicStream))
	r.Get("/api/subsonic/{sourceId}/cover/{coverId}", Adapt(api.subsonicCover))
	r.Get("/api/subsonic/{roomId}/{sourceId}/stream/{songId}", Adapt(api.subsonicStream))
	r.Get("/api/subsonic/{roomId}/{sourceId}/cover/{coverId}", Adapt(api.subsonicCover))
}

func (api *MediaAPI) admin(r *http.Request) (account.Session, bool) {
	session, err := api.accounts.Resolve(r.Context(), sessionToken(r))
	return session, err == nil && session.Admin()
}

func (api *MediaAPI) canManage(r *http.Request) (account.Session, bool) {
	if session, ok := api.admin(r); ok {
		return session, true
	}
	session, err := api.accounts.Resolve(r.Context(), sessionToken(r))
	if err != nil {
		return account.Session{}, false
	}
	allowed, err := api.repository.FindAllowedUploadUsers(r.Context())
	if err != nil {
		return account.Session{}, false
	}
	_, byName := allowed[strings.ToLower(session.Username)]
	_, byID := allowed[strings.ToLower(session.PublicID)]
	return session, byName || byID
}

func (api *MediaAPI) listLocal(w http.ResponseWriter, r *http.Request) error {
	if _, ok := api.admin(r); !ok {
		return accessDenied(w)
	}
	tracks, err := api.repository.FindAll(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, tracks)
	return nil
}

func (api *MediaAPI) uploadLocal(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, api.cfg.LocalLibrary.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return badRequest("Upload exceeds configured limit")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	session, ok := api.canManage(r)
	if !ok {
		return accessDenied(w)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return badRequest("file is required")
	}
	defer file.Close()
	track, duplicate, err := api.library.Upload(r.Context(), media.Upload{FileName: header.Filename, ContentType: header.Header.Get("Content-Type"), Title: r.FormValue("title"), Artists: r.FormValue("artists"), Album: r.FormValue("album"), UploadedBy: defaultValue(session.DisplayName, "user"), Size: header.Size, Reader: file})
	if err != nil {
		if errors.Is(err, media.ErrTranscodeQueueFull) {
			return &APIError{Status: http.StatusServiceUnavailable, Message: err.Error()}
		}
		return badRequest(err.Error())
	}
	result := map[string]any{"track": track, "duplicate": duplicate, "duplicateOf": nil}
	if duplicate {
		result["duplicateOf"] = track.ID
	}
	writeJSON(w, result)
	return nil
}

func (api *MediaAPI) updateLocal(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		Title, Album string
		Artists      []string `json:"artists"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if _, ok := api.canManage(r); !ok {
		return accessDenied(w)
	}
	track, err := api.repository.FindByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(request.Title) != "" {
		track.Title = strings.TrimSpace(request.Title)
	}
	if len(request.Artists) > 0 {
		track.Artists = request.Artists
	}
	track.Album = stringPointer(request.Album)
	track.UpdatedAt = time.Now().UnixMilli()
	if err := api.repository.Upsert(r.Context(), *track); err != nil {
		return err
	}
	writeJSON(w, track)
	return nil
}

func (api *MediaAPI) deleteLocal(w http.ResponseWriter, r *http.Request) error {
	if _, ok := api.canManage(r); !ok {
		return accessDenied(w)
	}
	if err := api.library.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		return err
	}
	writeJSON(w, map[string]string{"message": "LOCAL TRACK DELETED"})
	return nil
}

func (api *MediaAPI) listUploadAccess(w http.ResponseWriter, r *http.Request) error {
	if _, ok := api.admin(r); !ok {
		return accessDenied(w)
	}
	users, err := api.repository.FindAllowedUploadUsers(r.Context())
	if err != nil {
		return err
	}
	values := make([]string, 0, len(users))
	for user := range users {
		values = append(values, user)
	}
	writeJSON(w, values)
	return nil
}

func (api *MediaAPI) grantUploadAccess(w http.ResponseWriter, r *http.Request) error {
	return api.changeUploadAccess(w, r, true)
}
func (api *MediaAPI) revokeUploadAccess(w http.ResponseWriter, r *http.Request) error {
	return api.changeUploadAccess(w, r, false)
}
func (api *MediaAPI) changeUploadAccess(w http.ResponseWriter, r *http.Request, grant bool) error {
	var request struct{ UserName string }
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	if _, ok := api.admin(r); !ok {
		return accessDenied(w)
	}
	name := strings.ToLower(strings.TrimSpace(request.UserName))
	if name == "" {
		return badRequest("userName is required")
	}
	var err error
	message := "LOCAL UPLOAD ACCESS REVOKED"
	if grant {
		err = api.repository.GrantUploadUser(r.Context(), name, time.Now().UnixMilli())
		message = "LOCAL UPLOAD ACCESS GRANTED"
	} else {
		err = api.repository.RevokeUploadUser(r.Context(), name)
	}
	if err != nil {
		return err
	}
	writeJSON(w, map[string]string{"message": message})
	return nil
}

func (api *MediaAPI) canRead(r *http.Request) bool {
	_, err := api.accounts.Resolve(r.Context(), sessionToken(r))
	return err == nil
}

func (api *MediaAPI) localMedia(w http.ResponseWriter, r *http.Request) error {
	return api.localFile(w, r, false)
}
func (api *MediaAPI) localCover(w http.ResponseWriter, r *http.Request) error {
	return api.localFile(w, r, true)
}
func (api *MediaAPI) localFile(w http.ResponseWriter, r *http.Request, cover bool) error {
	if !api.canRead(r) {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}
	track, err := api.repository.FindByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	if track.Status == "DELETED" {
		w.WriteHeader(http.StatusGone)
		return nil
	}
	var relative *string
	contentType := "audio/ogg"
	cache := "no-store"
	if cover {
		relative, contentType, cache = track.CoverPath, dereference(track.CoverMIMEType), "private, max-age=86400"
	} else {
		relative = track.OGGPath
	}
	if track.Status != "COMPLETED" || relative == nil {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	path, err := media.ResolveWithin(api.library.Root(), *relative)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	if err := media.ServeFile(w, r, path, contentType, cache); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		return err
	}
	return nil
}

func (api *MediaAPI) neteaseStream(w http.ResponseWriter, r *http.Request) error {
	return api.proxyPlatform(w, r, "netease", chi.URLParam(r, "songId"), false)
}
func (api *MediaAPI) bilibiliStream(w http.ResponseWriter, r *http.Request) error {
	return api.proxyPlatform(w, r, "bilibili", chi.URLParam(r, "bvid"), false)
}
func (api *MediaAPI) youtubeStream(w http.ResponseWriter, r *http.Request) error {
	return api.proxyPlatform(w, r, "youtube", chi.URLParam(r, "videoId"), false)
}
func (api *MediaAPI) navidromeStream(w http.ResponseWriter, r *http.Request) error {
	return api.proxyPlatform(w, r, "navidrome", chi.URLParam(r, "songId"), false)
}
func (api *MediaAPI) navidromeCover(w http.ResponseWriter, r *http.Request) error {
	return api.proxyPlatform(w, r, "navidrome", chi.URLParam(r, "coverId"), true)
}
func (api *MediaAPI) subsonicStream(w http.ResponseWriter, r *http.Request) error {
	roomID := defaultValue(chi.URLParam(r, "roomId"), "lounge")
	return api.proxyPlatform(w, r, "subsonic-"+chi.URLParam(r, "sourceId")+"@"+roomID, chi.URLParam(r, "songId"), false)
}
func (api *MediaAPI) subsonicCover(w http.ResponseWriter, r *http.Request) error {
	roomID := defaultValue(chi.URLParam(r, "roomId"), "lounge")
	return api.proxyPlatform(w, r, "subsonic-"+chi.URLParam(r, "sourceId")+"@"+roomID, chi.URLParam(r, "coverId"), true)
}
func (api *MediaAPI) proxyPlatform(w http.ResponseWriter, r *http.Request, name, id string, cover bool) error {
	cacheKey := name + ":" + id
	if !cover && api.cache != nil {
		if path, ok := api.cache.Get(cacheKey); ok {
			return media.ServeFile(w, r, path, "application/octet-stream", "private, max-age=86400")
		}
	}
	service, ok := api.platforms.Service(name)
	if !ok {
		return forbidden(w)
	}
	if check := api.platforms.access[name]; check != nil && !check(r.Context(), sessionToken(r)) {
		return forbidden(w)
	}
	source, ok := service.(platform.MediaSource)
	if !ok {
		return BadGateway("Platform does not expose media")
	}
	var target string
	var err error
	if cover {
		target, err = source.CoverURL(r.Context(), id)
	} else {
		target, err = source.StreamURL(r.Context(), id)
	}
	if err != nil || target == "" {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	if !cover && api.cache != nil {
		_ = api.cache.Submit(cacheKey, target)
	}
	headers := http.Header{"Accept": {"application/octet-stream"}}
	if name == "netease" {
		headers.Set("Referer", "https://music.163.com/")
		headers.Set("User-Agent", "Mozilla/5.0")
	}
	if name == "bilibili" {
		headers.Set("Referer", "https://www.bilibili.com/video/"+id)
		headers.Set("User-Agent", "Mozilla/5.0")
	}
	if cover {
		w.Header().Set("Cache-Control", "private, max-age=86400")
	}
	if err := api.proxy.Serve(w, r, target, headers); err != nil {
		return BadGateway("Media proxy failed")
	}
	return nil
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
