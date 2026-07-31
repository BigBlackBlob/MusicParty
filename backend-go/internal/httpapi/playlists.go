package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PlaylistAPI struct {
	store         *storesqlite.Store
	accounts      *account.Service
	rooms         *roomdomain.Service
	users         *storesqlite.UserPlaylistRepository
	roomPlaylists *storesqlite.RoomPlaylistRepository
	platforms     *PlatformAPI
	roomSecret    []byte
	runtimes      *realtime.Manager
}

func NewPlaylistAPI(cfg config.Config, store *storesqlite.Store, accounts *account.Service, rooms *roomdomain.Service, platforms *PlatformAPI) *PlaylistAPI {
	secret := []byte(cfg.Auth.RoomAccessTokenSecret)
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			panic("generate room access token secret: " + err.Error())
		}
	}
	return &PlaylistAPI{store: store, accounts: accounts, rooms: rooms, users: storesqlite.NewUserPlaylistRepository(store, time.Now, nil), roomPlaylists: storesqlite.NewRoomPlaylistRepository(store, time.Now, nil), platforms: platforms, roomSecret: secret}
}
func (api *PlaylistAPI) Routes(r chi.Router) {
	r.Get("/api/me/playlists", Adapt(api.userList))
	r.Post("/api/me/playlists", Adapt(api.userCreate))
	r.Patch("/api/me/playlists/{playlistId}", Adapt(api.userRename))
	r.Delete("/api/me/playlists/{playlistId}", Adapt(api.userDelete))
	r.Get("/api/me/playlists/{playlistId}/tracks", Adapt(api.userTracks))
	r.Post("/api/me/playlists/{playlistId}/tracks/batch", Adapt(api.userAddBatch))
	r.Delete("/api/me/playlists/{playlistId}/tracks/{trackId}", Adapt(api.userDeleteTrack))
	r.Post("/api/me/playlists/{playlistId}/tracks/reorder", Adapt(api.userReorder))
	r.Post("/api/me/playlists/{playlistId}/import", Adapt(api.userImport))
	r.Post("/api/me/playlists/{playlistId}/import/netease", Adapt(api.userImportNetease))
	r.Get("/api/me/playlists/{playlistId}/export", Adapt(api.userExport))
	r.Get("/api/me/liked-songs", Adapt(api.likedSongs))
	r.Put("/api/me/liked-songs/{platform}/{musicId}", Adapt(api.likeSong))
	r.Delete("/api/me/liked-songs/{platform}/{musicId}", Adapt(api.unlikeSong))
	r.Get("/api/rooms/{roomId}/playlists", Adapt(api.roomList))
	r.Post("/api/rooms/{roomId}/playlists", Adapt(api.roomCreate))
	r.Patch("/api/rooms/{roomId}/playlists/{playlistId}", Adapt(api.roomRename))
	r.Delete("/api/rooms/{roomId}/playlists/{playlistId}", Adapt(api.roomDelete))
	r.Get("/api/rooms/{roomId}/playlists/{playlistId}/tracks", Adapt(api.roomTracks))
	r.Post("/api/rooms/{roomId}/playlists/{playlistId}/tracks", Adapt(api.roomAddTrack))
	r.Delete("/api/rooms/{roomId}/playlists/{playlistId}/tracks/{trackId}", Adapt(api.roomDeleteTrack))
	r.Post("/api/rooms/{roomId}/playlists/{playlistId}/tracks/reorder", Adapt(api.roomReorder))
	r.Post("/api/rooms/{roomId}/playlists/{playlistId}/import", Adapt(api.roomImport))
	r.Get("/api/rooms/{roomId}/playlists/{playlistId}/export", Adapt(api.roomExport))
	r.Post("/api/me/playlists/{playlistId}/enqueue", Adapt(api.userEnqueue))
}

func (api *PlaylistAPI) SetRealtime(manager *realtime.Manager) { api.runtimes = manager }

type playlistView struct {
	ID            string  `json:"id"`
	OwnerPublicID string  `json:"ownerPublicId,omitempty"`
	RoomID        string  `json:"roomId,omitempty"`
	Name          string  `json:"name"`
	SystemKey     *string `json:"systemKey"`
	TrackCount    int     `json:"trackCount"`
	CreatedAt     int64   `json:"createdAt"`
	UpdatedAt     int64   `json:"updatedAt"`
}
type trackView struct {
	ID         string            `json:"id"`
	PlaylistID string            `json:"playlistId"`
	Music      storesqlite.Music `json:"music"`
	SortOrder  int               `json:"sortOrder"`
	CreatedAt  int64             `json:"createdAt"`
}

func userPlaylist(value storesqlite.Playlist) playlistView {
	return playlistView{ID: value.ID, OwnerPublicID: value.OwnerID, Name: value.Name, SystemKey: value.SystemKey, TrackCount: value.TrackCount, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func roomPlaylist(value storesqlite.Playlist) playlistView {
	return playlistView{ID: value.ID, RoomID: value.OwnerID, Name: value.Name, SystemKey: value.SystemKey, TrackCount: value.TrackCount, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}
func track(value storesqlite.PlaylistTrack) trackView {
	return trackView{ID: value.ID, PlaylistID: value.PlaylistID, Music: value.Music, SortOrder: value.SortOrder, CreatedAt: value.CreatedAt}
}
func (api *PlaylistAPI) user(r *http.Request) (account.Session, error) {
	return api.accounts.Resolve(r.Context(), sessionToken(r))
}
func (api *PlaylistAPI) userList(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	if _, err := api.liked(w, r, session); err != nil {
		return err
	}
	values, err := api.users.ListPlaylists(r.Context(), session.PublicID)
	if err != nil {
		return err
	}
	out := make([]playlistView, 0, len(values))
	for _, v := range values {
		out = append(out, userPlaylist(v))
	}
	writeJSON(w, out)
	return nil
}
func (api *PlaylistAPI) userCreate(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	name, err := requestName(r)
	if err != nil {
		return err
	}
	value, err := api.users.CreatePlaylist(r.Context(), session.PublicID, name)
	if err != nil {
		return err
	}
	writeJSON(w, userPlaylist(value))
	return nil
}
func (api *PlaylistAPI) userRename(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	name, err := requestName(r)
	if err != nil {
		return err
	}
	value, err := api.users.RenamePlaylist(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), name)
	if err != nil {
		return err
	}
	if value == nil {
		w.WriteHeader(404)
		return nil
	}
	writeJSON(w, userPlaylist(*value))
	return nil
}
func (api *PlaylistAPI) userDelete(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	changed, err := api.users.DeletePlaylist(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"))
	return noContentOrNotFound(w, changed, err)
}
func (api *PlaylistAPI) userTracks(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	offset, limit := pagination(r, 100)
	values, err := api.users.ListTracks(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), offset, limit)
	return writeTracks(w, values, err)
}
func (api *PlaylistAPI) userAddBatch(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	var request struct {
		Musics []storesqlite.Music `json:"musics"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	values, err := api.users.AddTracksIfAbsent(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), request.Musics)
	if err != nil {
		return err
	}
	added := make([]trackView, 0, len(values))
	for _, value := range values {
		added = append(added, track(value))
	}
	writeJSON(w, batchResult(added, len(request.Musics)))
	return nil
}
func (api *PlaylistAPI) userDeleteTrack(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	changed, err := api.users.DeleteTrack(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), chi.URLParam(r, "trackId"))
	return noContentOrNotFound(w, changed, err)
}
func (api *PlaylistAPI) userReorder(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	ids, err := requestTrackIDs(r)
	if err != nil {
		return err
	}
	if err := api.users.ReorderTracks(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), ids); err != nil {
		return err
	}
	return nil
}
func (api *PlaylistAPI) userImport(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	var request struct {
		Platform   string `json:"platform"`
		PlaylistID string `json:"playlistId"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	return api.importUser(w, r, session.PublicID, request.Platform, request.PlaylistID)
}
func (api *PlaylistAPI) userImportNetease(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	var request struct {
		PlaylistID string `json:"playlistId"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	return api.importUser(w, r, session.PublicID, "netease", request.PlaylistID)
}
func (api *PlaylistAPI) importUser(w http.ResponseWriter, r *http.Request, owner, platformName, remoteID string) error {
	songs, err := api.remoteSongs(r, platformName, remoteID)
	if err != nil {
		return err
	}
	musics := make([]storesqlite.Music, 0, len(songs))
	for i := range songs {
		musics = append(musics, toStoreMusic(songs[i]))
	}
	values, err := api.users.AddTracksIfAbsent(r.Context(), owner, chi.URLParam(r, "playlistId"), musics)
	if err != nil {
		return err
	}
	out := make([]trackView, 0, len(values))
	for _, value := range values {
		out = append(out, track(value))
	}
	writeJSON(w, batchResult(out, len(musics)))
	return nil
}
func (api *PlaylistAPI) userExport(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	values, err := api.users.ListTracks(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), 0, 500)
	return exportTracks(w, r, values, err)
}
func (api *PlaylistAPI) liked(w http.ResponseWriter, r *http.Request, session account.Session) (storesqlite.Playlist, error) {
	value, err := api.users.FindSystemPlaylist(r.Context(), session.PublicID, "liked-songs")
	if err == nil {
		return *value, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return storesqlite.Playlist{}, err
	}
	return api.users.CreateSystemPlaylist(r.Context(), session.PublicID, "喜欢的歌曲", "liked-songs")
}
func (api *PlaylistAPI) likedSongs(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	playlist, err := api.liked(w, r, session)
	if err != nil {
		return err
	}
	values, err := api.users.ListTracks(r.Context(), session.PublicID, playlist.ID, 0, 500)
	return writeTracks(w, values, err)
}
func (api *PlaylistAPI) likeSong(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	var music storesqlite.Music
	if err := decodeJSONBody(r, &music); err != nil {
		return badRequest("Invalid request body")
	}
	music.Platform = chi.URLParam(r, "platform")
	music.ID = chi.URLParam(r, "musicId")
	playlist, err := api.liked(w, r, session)
	if err != nil {
		return err
	}
	value, err := api.users.AddTrackIfAbsent(r.Context(), session.PublicID, playlist.ID, &music)
	if err != nil {
		return err
	}
	if value == nil {
		writeJSON(w, batchResult(nil, 1))
	} else {
		writeJSON(w, batchResult([]trackView{track(*value)}, 1))
	}
	return nil
}
func (api *PlaylistAPI) unlikeSong(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	playlist, err := api.liked(w, r, session)
	if err != nil {
		return err
	}
	changed, err := api.users.DeleteTrackByMusicKey(r.Context(), session.PublicID, playlist.ID, chi.URLParam(r, "platform")+":"+chi.URLParam(r, "musicId"))
	return noContentOrNotFound(w, changed, err)
}
func (api *PlaylistAPI) roomWrite(r *http.Request) error {
	session, err := api.accounts.Resolve(r.Context(), roomSessionToken(r))
	if err != nil {
		return err
	}
	roomID := chi.URLParam(r, "roomId")
	var visibility string
	var passwordVersion int
	if err := api.store.Reader().QueryRowContext(r.Context(), "select visibility,password_version from room where id=? and deleted_at is null", roomID).Scan(&visibility, &passwordVersion); err != nil {
		return errors.New("Forbidden")
	}
	if visibility != "PRIVATE" {
		return nil
	}
	if !validRoomAccessToken(api.roomSecret, r.URL.Query().Get("roomAccessToken"), roomID, session.PublicID, passwordVersion, time.Now()) {
		return errors.New("Forbidden")
	}
	return nil
}

func validRoomAccessToken(secret []byte, token, roomID, publicID string, passwordVersion int, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || publicID == "" {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(mac.Sum(nil), signature) {
		return false
	}
	fields := strings.Split(string(payload), "|")
	if len(fields) != 4 {
		return false
	}
	expiresAt, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return false
	}
	version, err := strconv.Atoi(fields[3])
	return err == nil && fields[0] == roomID && fields[1] == publicID && expiresAt >= now.UnixMilli() && version == passwordVersion
}
func (api *PlaylistAPI) roomList(w http.ResponseWriter, r *http.Request) error {
	values, err := api.roomPlaylists.ListPlaylists(r.Context(), chi.URLParam(r, "roomId"))
	if err != nil {
		return err
	}
	out := make([]playlistView, 0, len(values))
	for _, v := range values {
		out = append(out, roomPlaylist(v))
	}
	writeJSON(w, out)
	return nil
}
func (api *PlaylistAPI) roomCreate(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomWrite(r); err != nil {
		return roomAccessError(w, err)
	}
	name, err := requestName(r)
	if err != nil {
		return err
	}
	value, err := api.roomPlaylists.CreatePlaylist(r.Context(), chi.URLParam(r, "roomId"), name)
	if err != nil {
		return err
	}
	writeJSON(w, roomPlaylist(value))
	return nil
}
func (api *PlaylistAPI) roomRename(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomWrite(r); err != nil {
		return roomAccessError(w, err)
	}
	name, err := requestName(r)
	if err != nil {
		return err
	}
	value, err := api.roomPlaylists.RenamePlaylist(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), name)
	if err != nil {
		return err
	}
	if value == nil {
		w.WriteHeader(404)
	} else {
		writeJSON(w, roomPlaylist(*value))
	}
	return nil
}
func (api *PlaylistAPI) roomDelete(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomWrite(r); err != nil {
		return roomAccessError(w, err)
	}
	changed, err := api.roomPlaylists.DeletePlaylist(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"))
	return noContentOrNotFound(w, changed, err)
}
func (api *PlaylistAPI) roomTracks(w http.ResponseWriter, r *http.Request) error {
	offset, limit := pagination(r, 100)
	values, err := api.roomPlaylists.ListTracks(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), offset, limit)
	return writeTracks(w, values, err)
}
func (api *PlaylistAPI) roomAddTrack(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomWrite(r); err != nil {
		return roomAccessError(w, err)
	}
	var request struct {
		Music storesqlite.Music `json:"music"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	value, err := api.roomPlaylists.AddTrack(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), &request.Music)
	if err != nil {
		return err
	}
	if value == nil {
		w.WriteHeader(404)
	} else {
		writeJSON(w, track(*value))
	}
	return nil
}
func (api *PlaylistAPI) roomDeleteTrack(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomWrite(r); err != nil {
		return roomAccessError(w, err)
	}
	changed, err := api.roomPlaylists.DeleteTrack(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), chi.URLParam(r, "trackId"))
	return noContentOrNotFound(w, changed, err)
}
func (api *PlaylistAPI) roomReorder(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomWrite(r); err != nil {
		return roomAccessError(w, err)
	}
	ids, err := requestTrackIDs(r)
	if err != nil {
		return err
	}
	if err := api.roomPlaylists.ReorderTracks(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), ids); err != nil {
		return err
	}
	return nil
}
func (api *PlaylistAPI) roomImport(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomWrite(r); err != nil {
		return roomAccessError(w, err)
	}
	var request struct {
		Platform   string `json:"platform"`
		PlaylistID string `json:"playlistId"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return badRequest("Invalid request body")
	}
	songs, err := api.remoteSongs(r, request.Platform, request.PlaylistID)
	if err != nil {
		return err
	}
	musics := make([]storesqlite.Music, 0, len(songs))
	for i := range songs {
		musics = append(musics, toStoreMusic(songs[i]))
	}
	values, err := api.roomPlaylists.AddTracks(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), musics)
	if err != nil {
		return err
	}
	out := make([]trackView, 0, len(values))
	for _, value := range values {
		out = append(out, track(value))
	}
	writeJSON(w, out)
	return nil
}
func (api *PlaylistAPI) roomExport(w http.ResponseWriter, r *http.Request) error {
	values, err := api.roomPlaylists.ListTracks(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), 0, 500)
	return exportTracks(w, r, values, err)
}
func (api *PlaylistAPI) remoteSongs(r *http.Request, name, id string) ([]platform.Music, error) {
	service, ok := api.platforms.Service(name)
	if !ok {
		return nil, badRequest("Platform not supported: " + name)
	}
	values, err := service.PlaylistSongs(r.Context(), id, 0, 500)
	if err != nil {
		if name == "netease" {
			return nil, BadGateway("尚未配置网易云 Cookie，请联系管理员设置")
		}
		return nil, mapPlatformError(err)
	}
	return values, nil
}
func toStoreMusic(value platform.Music) storesqlite.Music {
	return storesqlite.Music{ID: value.ID, Name: value.Name, Artists: value.Artists, Duration: value.Duration, Platform: value.Platform, CoverURL: value.CoverURL}
}
func requestName(r *http.Request) (string, error) {
	var request struct {
		Name string `json:"name"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return "", badRequest("Invalid request body")
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len([]rune(request.Name)) > 64 {
		return "", badRequest("name must be 1-64 characters")
	}
	return request.Name, nil
}
func requestTrackIDs(r *http.Request) ([]string, error) {
	var request struct {
		TrackIDs []string `json:"trackIds"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return nil, badRequest("Invalid request body")
	}
	return request.TrackIDs, nil
}
func pagination(r *http.Request, defaultLimit int) (int, int) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = defaultLimit
	}
	return max(0, offset), min(500, limit)
}
func writeTracks(w http.ResponseWriter, values []storesqlite.PlaylistTrack, err error) error {
	if err != nil {
		return err
	}
	out := make([]trackView, 0, len(values))
	for _, v := range values {
		out = append(out, track(v))
	}
	writeJSON(w, out)
	return nil
}
func exportTracks(w http.ResponseWriter, r *http.Request, values []storesqlite.PlaylistTrack, err error) error {
	if err != nil {
		return err
	}
	format := r.URL.Query().Get("format")
	if format == "json" {
		w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
		out := make([]map[string]any, 0, len(values))
		for _, value := range values {
			out = append(out, map[string]any{"id": value.Music.ID, "name": value.Music.Name, "artists": value.Music.Artists, "duration": value.Music.Duration, "platform": value.Music.Platform, "coverUrl": value.Music.CoverURL, "externalUrl": ""})
		}
		return json.NewEncoder(w).Encode(out)
	}
	w.Header().Set("Content-Type", "text/plain;charset=UTF-8")
	for _, v := range values {
		_, _ = fmt.Fprintf(w, "%s - %s [%s:%s]\n", strings.Join(v.Music.Artists, ", "), v.Music.Name, v.Music.Platform, v.Music.ID)
	}
	return nil
}
func roomSessionToken(r *http.Request) string {
	if token := sessionToken(r); token != "" {
		return token
	}
	return r.URL.Query().Get("sessionToken")
}
func noContentOrNotFound(w http.ResponseWriter, changed bool, err error) error {
	if err != nil {
		return err
	}
	if changed {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(404)
	}
	return nil
}
func unauthorized(w http.ResponseWriter) error {
	writeErrorJSON(w, 401, map[string]string{"message": "Unknown session token"})
	return nil
}
func forbidden(w http.ResponseWriter) error {
	writeErrorJSON(w, 403, map[string]string{"message": "Forbidden"})
	return nil
}
func roomAccessError(w http.ResponseWriter, err error) error {
	if errors.Is(err, account.ErrUnknownSession) {
		return unauthorized(w)
	}
	return forbidden(w)
}
func badRequest(message string) error { return &APIError{Status: 400, Message: message} }

func batchResult(tracks []trackView, requested int) map[string]any {
	if tracks == nil {
		tracks = []trackView{}
	}
	return map[string]any{"addedCount": len(tracks), "skippedCount": max(0, requested-len(tracks)), "tracks": tracks}
}
func (api *PlaylistAPI) userEnqueue(w http.ResponseWriter, r *http.Request) error {
	if api.runtimes == nil {
		return &APIError{Status: http.StatusServiceUnavailable, Message: "Realtime service unavailable"}
	}
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	tracks, err := api.users.ListTracks(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), 0, 500)
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return &APIError{Status: http.StatusInternalServerError, Message: "An unexpected internal server error occurred.", Name: "ResponseStatusException"}
	}
	roomID := defaultValue(r.URL.Query().Get("roomId"), "lounge")
	runtime, err := api.runtimes.Room(r.Context(), roomID)
	if err != nil {
		return err
	}
	musics := make([]storesqlite.Music, len(tracks))
	for i := range tracks {
		musics[i] = tracks[i].Music
	}
	items, err := runtime.EnqueueMany(r.Context(), session, musics, uuid.NewString())
	if err != nil {
		return err
	}
	writeJSON(w, map[string]any{"accepted": true, "count": len(items)})
	return nil
}
