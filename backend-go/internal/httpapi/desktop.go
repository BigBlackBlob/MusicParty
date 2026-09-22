package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
)

const desktopAPIVersion = "2026-01"
const desktopServerVersion = "1.2.0"
const desktopMinimumClientVersion = "0.2.0"

// Desktop clients use stable numeric releases; prereleases are not negotiated.
func desktopCompatible(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	var numbers [3]int
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return false
		}
		numbers[i] = n
	}
	return numbers[0] > 0 || numbers[1] >= 2
}

func desktopVersion(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if r.Header.Get("X-Desktop-API-Version") != desktopAPIVersion || !desktopCompatible(r.Header.Get("X-Desktop-Client-Version")) {
		writeErrorJSON(w, 426, map[string]any{"code": "version-incompatible", "error": "version-incompatible", "message": "Desktop client version is incompatible", "status": 426, "apiVersion": desktopAPIVersion, "minimumClientVersion": desktopMinimumClientVersion})
		return false
	}
	return true
}

func desktopGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if desktopVersion(w, r) {
			next.ServeHTTP(w, r)
		}
	})
}

type DesktopAPI struct {
	cfg       config.Config
	accounts  *account.Service
	rooms     *roomdomain.Service
	platforms *PlatformAPI
	cookies   CookieFactory
}

func NewDesktopAPI(cfg config.Config, accounts *account.Service, rooms *roomdomain.Service, platforms *PlatformAPI) *DesktopAPI {
	return &DesktopAPI{cfg: cfg, accounts: accounts, rooms: rooms, platforms: platforms, cookies: CookieFactory{SecureCookies: cfg.Auth.SecureCookies}}
}

func (api *DesktopAPI) Routes(r chi.Router) {
	r.Get("/api/desktop/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "apiVersion": desktopAPIVersion})
	})
	r.Get("/api/desktop/v1/capabilities", api.capabilities)
	r.With(desktopGuard).Post("/api/desktop/v1/invites/redeem", Adapt(api.redeemInvite))
	r.With(desktopGuard).Get("/api/desktop/v1/media/{platform}/{songId}/resolve", Adapt(api.resolveMedia))
	r.With(desktopGuard).Get("/api/desktop/v1/media/{platform}/{songId}/cover", Adapt(api.cover))
	r.With(desktopGuard).Get("/api/desktop/v1/media/{platform}/{songId}/lyrics", Adapt(api.lyrics))
	r.With(desktopGuard).Get("/api/desktop/v1/music/{platform}/{songId}/lyrics", Adapt(api.lyrics))
	r.With(desktopGuard).Get("/api/desktop/v1/search/{platform}", Adapt(api.searchDesktop))
}

// Negotiate before registering presence or exposing room state. Each reconnect
// repeats this handshake; session credentials belong in Cookie, never the URL.
func desktopHandshake(parent context.Context, connection *websocket.Conn, roomID string) bool {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	var envelope inboundEnvelope
	if err := wsruntime.ReadJSON(ctx, connection, &envelope); err != nil {
		_ = connection.CloseNow()
		return false
	}
	var hello struct {
		APIVersion    string `json:"apiVersion"`
		ClientVersion string `json:"clientVersion"`
	}
	if envelope.Type != "client.hello" || json.Unmarshal(envelope.Payload, &hello) != nil || hello.APIVersion != desktopAPIVersion || !desktopCompatible(hello.ClientVersion) {
		_ = connection.Close(websocket.StatusPolicyViolation, "version-incompatible")
		return false
	}
	data, err := json.Marshal(wsruntime.Envelope{Type: "server.hello", RoomID: roomID, Payload: map[string]string{
		"apiVersion": desktopAPIVersion, "minimumClientVersion": desktopMinimumClientVersion,
	}})
	if err != nil || connection.Write(ctx, websocket.MessageText, data) != nil {
		_ = connection.CloseNow()
		return false
	}
	return true
}

func (api *DesktopAPI) capabilities(w http.ResponseWriter, _ *http.Request) {
	providers := map[string]bool{}
	for _, name := range []string{"netease", "youtube", "bilibili"} {
		_, providers[name] = api.platforms.Service(name)
	}
	writeJSON(w, map[string]any{"apiVersion": desktopAPIVersion, "serverVersion": desktopServerVersion, "minimumClientVersion": desktopMinimumClientVersion,
		"authentication": "cookie", "reconnect": "snapshot", "eventReplay": false, "providers": providers,
		// Proven live 2026-09-22: the deployed netease-api (moefurina/ncm-api) serves
		// /lyric/new with yrc; clients may rely on the wordLyric fields when present.
		"features": map[string]bool{"inviteRedeem": true, "mediaResolve": true, "queue": true, "chat": true, "lyrics": true, "controlAck": true, "controlPreconditions": true, "controlIdempotency": true, "lyricsWordLevel": true}})
}

func (api *DesktopAPI) redeemInvite(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		Code        string `json:"code"`
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSONBody(r, &request); err != nil || strings.TrimSpace(request.Code) == "" {
		return &APIError{Status: http.StatusBadRequest, Name: "invite-invalid", Message: "邀请码无效"}
	}
	metadata := api.accounts.InviteMetadata(r.Context(), request.Code)
	if metadata.RoomID == "" {
		return &APIError{Status: http.StatusBadRequest, Name: "invite-invalid", Message: "邀请码无效"}
	}
	session, err := api.accounts.RedeemInvite(r.Context(), request.Code, request.DisplayName)
	if err != nil {
		return &APIError{Status: http.StatusBadRequest, Name: "invite-invalid", Message: "邀请码无效"}
	}
	cookies, cookieErr := api.cookies.EstablishMember(r, session.SessionToken)
	if cookieErr != nil {
		return cookieErr
	}
	for _, cookie := range cookies {
		http.SetCookie(w, cookie)
	}
	session.SessionToken = ""
	writeJSON(w, map[string]any{"roomId": metadata.RoomID, "roomName": metadata.RoomName, "membership": map[string]string{"userId": session.PublicID, "displayName": session.DisplayName, "role": "member"}, "apiVersion": desktopAPIVersion})
	return nil
}

func (api *DesktopAPI) resolveMedia(w http.ResponseWriter, r *http.Request) error {
	session, err := api.accounts.Resolve(r.Context(), sessionToken(r))
	if err != nil {
		return &APIError{Status: http.StatusUnauthorized, Name: "unauthorized", Message: "Authentication required"}
	}
	value, err := api.platforms.ResolvePlayable(r.Context(), chi.URLParam(r, "platform"), chi.URLParam(r, "songId"), session.SessionToken)
	if err != nil {
		return desktopMediaError(err)
	}
	writeJSON(w, map[string]any{"url": value.URL, "expiresAt": nil, "contentType": nil, "resolvedAt": time.Now().UnixMilli(), "music": value.Music})
	return nil
}

func desktopMediaError(err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == 403 {
		return &APIError{Status: 403, Name: "forbidden", Message: "Provider access denied"}
	}
	return &APIError{Status: 502, Name: "media-failed", Message: "Provider unavailable or media could not be resolved"}
}

func (api *DesktopAPI) mediaSession(r *http.Request) error {
	if _, err := api.accounts.Resolve(r.Context(), sessionToken(r)); err != nil {
		return &APIError{Status: 401, Name: "unauthorized", Message: "Authentication required"}
	}
	name := chi.URLParam(r, "platform")
	api.platforms.mu.RLock()
	check := api.platforms.access[name]
	api.platforms.mu.RUnlock()
	if check != nil && !check(r.Context(), sessionToken(r)) {
		return &APIError{Status: 403, Name: "forbidden", Message: "Provider access denied"}
	}
	return nil
}

func (api *DesktopAPI) searchDesktop(w http.ResponseWriter, r *http.Request) error {
	if err := api.mediaSession(r); err != nil {
		return err
	}
	offset, limit, err := queryPage(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if err != nil || q == "" || len(q) > 128 || offset < 0 || offset > 10000 || limit < 1 || limit > 100 {
		return &APIError{Status: 400, Name: "invalid-request", Message: "Invalid search parameters"}
	}
	service, ok := api.platforms.Service(chi.URLParam(r, "platform"))
	if !ok {
		return desktopMediaError(errors.New("unavailable"))
	}
	items, err := service.Search(r.Context(), q, offset, limit)
	if err != nil {
		return desktopMediaError(err)
	}
	writeJSON(w, map[string]any{"items": items, "offset": offset, "limit": limit})
	return nil
}

func (api *DesktopAPI) lyrics(w http.ResponseWriter, r *http.Request) error {
	if err := api.mediaSession(r); err != nil {
		return err
	}
	service, ok := api.platforms.Service(chi.URLParam(r, "platform"))
	if !ok {
		return desktopMediaError(errors.New("unavailable"))
	}
	value, err := service.Lyric(r.Context(), chi.URLParam(r, "songId"))
	if err != nil {
		return desktopMediaError(err)
	}
	// The desktop adapter consumes this compatibility route as plain LRC text.
	if strings.HasPrefix(r.URL.Path, "/api/desktop/v1/music/") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, err = w.Write([]byte(value.Lyric))
		return err
	}
	writeJSON(w, value)
	return nil
}

func (api *DesktopAPI) cover(w http.ResponseWriter, r *http.Request) error {
	if err := api.mediaSession(r); err != nil {
		return err
	}
	value, err := api.platforms.ResolvePlayable(r.Context(), chi.URLParam(r, "platform"), chi.URLParam(r, "songId"), sessionToken(r))
	if err != nil {
		return desktopMediaError(err)
	}
	writeJSON(w, map[string]any{"url": value.CoverURL})
	return nil
}
