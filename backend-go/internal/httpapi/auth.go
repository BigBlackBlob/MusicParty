package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/go-chi/chi/v5"
)

type AuthAPI struct {
	service       *account.Service
	cookies       CookieFactory
	limiter       *loginLimiter
	sessionCloser sessionCloser
	presence      sessionPresenceUpdater
}

type sessionCloser interface {
	CloseSession(string)
}

type sessionPresenceUpdater interface {
	UpdateUserSession(account.Session)
}

func NewAuthAPI(cfg config.Config, service *account.Service) *AuthAPI {
	return &AuthAPI{service: service, cookies: CookieFactory{SecureCookies: cfg.Auth.SecureCookies}, limiter: newLoginLimiter(cfg)}
}

// SetSessionCloser lets the realtime hub revoke existing WebSocket clients
// when an HTTP logout or password reset invalidates their cookie session.
func (api *AuthAPI) SetSessionCloser(closer sessionCloser) { api.sessionCloser = closer }

// SetSessionPresenceUpdater propagates profile changes to existing room
// connections without making the frontend infer identity from stale data.
func (api *AuthAPI) SetSessionPresenceUpdater(updater sessionPresenceUpdater) { api.presence = updater }
func (api *AuthAPI) Routes(r chi.Router) {
	r.Get("/api/account/status", Adapt(api.status))
	r.Post("/api/account/register", Adapt(api.register))
	r.Post("/api/account/login", Adapt(api.login))
	r.Post("/api/account/guest", Adapt(api.createGuest))
	r.Post("/api/account/upgrade", Adapt(api.upgradeGuest))
	r.Get("/api/account/me", Adapt(api.me))
	r.Post("/api/account/logout", Adapt(api.logout))
	r.Post("/api/account/change-password", Adapt(api.changePassword))
	r.Put("/api/account/profile", Adapt(api.profile))
	r.Get("/api/join/{secret}/metadata", Adapt(api.inviteMetadata))
	r.Post("/api/invites/redeem", Adapt(api.redeem))
}
func (api *AuthAPI) status(w http.ResponseWriter, r *http.Request) error {
	value, err := api.service.Status(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, value)
	return nil
}
func (*AuthAPI) register(w http.ResponseWriter, _ *http.Request) error {
	writeErrorJSON(w, http.StatusGone, map[string]string{"message": "Member passwords were replaced by invitation links"})
	return nil
}
func (api *AuthAPI) createGuest(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return &APIError{Status: http.StatusBadRequest, Message: "Invalid request body"}
	}
	session, err := api.service.CreateGuestSession(r.Context(), request.DisplayName)
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, map[string]string{"message": err.Error()})
		return nil
	}
	cookies, err := api.cookies.EstablishMember(r, session.SessionToken)
	if err != nil {
		return err
	}
	for _, cookie := range cookies {
		http.SetCookie(w, cookie)
	}
	session.SessionToken = ""
	writeJSON(w, session)
	return nil
}
func (api *AuthAPI) upgradeGuest(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		Secret string `json:"secret"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return &APIError{Status: http.StatusBadRequest, Message: "Invalid request body"}
	}
	session, err := api.service.UpgradeGuestToUser(r.Context(), sessionToken(r), request.Secret)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "Unknown session token" {
			status = http.StatusUnauthorized
		} else if err.Error() == "session is already a registered user" {
			status = http.StatusConflict
		} else if err.Error() == "Invitation is invalid or expired" {
			status = http.StatusUnauthorized
		}
		writeErrorJSON(w, status, map[string]string{"message": err.Error()})
		return nil
	}
	cookies, err := api.cookies.EstablishMember(r, session.SessionToken)
	if err != nil {
		return err
	}
	for _, cookie := range cookies {
		http.SetCookie(w, cookie)
	}
	session.SessionToken = ""
	writeJSON(w, session)
	return nil
}

func (api *AuthAPI) login(w http.ResponseWriter, r *http.Request) error {
	ip := ClientIP(r)
	if retry, blocked := api.limiter.blocked(ip); blocked {
		w.Header().Set("Retry-After", retry)
		writeErrorJSON(w, http.StatusTooManyRequests, map[string]string{"message": "Too many login attempts. Try again later."})
		return nil
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return &APIError{Status: http.StatusBadRequest, Message: "Invalid request body"}
	}
	session, err := api.service.Login(r.Context(), request.Username, request.Password)
	if err != nil {
		api.limiter.failure(ip)
		writeErrorJSON(w, http.StatusUnauthorized, map[string]string{"message": err.Error()})
		return nil
	}
	if !session.Admin() {
		_ = api.service.Logout(r.Context(), session.SessionToken)
		api.limiter.failure(ip)
		writeErrorJSON(w, http.StatusForbidden, map[string]string{"message": "Platform administrator account required"})
		return nil
	}
	api.limiter.success(ip)
	cookies, err := api.cookies.EstablishAdmin(r, session.SessionToken)
	if err != nil {
		return err
	}
	for _, cookie := range cookies {
		http.SetCookie(w, cookie)
	}
	session.SessionToken = ""
	writeJSON(w, session)
	return nil
}
func (api *AuthAPI) me(w http.ResponseWriter, r *http.Request) error {
	session, err := api.service.Resolve(r.Context(), sessionToken(r))
	if err != nil {
		writeErrorJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unknown session token"})
		return nil
	}
	session.SessionToken = ""
	writeJSON(w, session)
	return nil
}
func (api *AuthAPI) logout(w http.ResponseWriter, r *http.Request) error {
	if err := api.logoutSession(r.Context(), sessionToken(r)); err != nil {
		return err
	}
	for _, cookie := range api.cookies.Clear(r) {
		http.SetCookie(w, cookie)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
func (api *AuthAPI) changePassword(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return &APIError{Status: 400, Message: "Invalid request body"}
	}
	err := api.service.ChangePassword(r.Context(), sessionToken(r), request.CurrentPassword, request.NewPassword)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, account.ErrUnknownSession) {
			status = http.StatusUnauthorized
		} else if err.Error() == "Member passwords are not supported" {
			status = http.StatusForbidden
		}
		writeErrorJSON(w, status, map[string]string{"message": err.Error()})
		return nil
	}
	_ = api.logoutSession(r.Context(), sessionToken(r))
	for _, cookie := range api.cookies.Clear(r) {
		http.SetCookie(w, cookie)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (api *AuthAPI) logoutSession(ctx context.Context, token string) error {
	if err := api.service.Logout(ctx, token); err != nil {
		return err
	}
	if api.sessionCloser != nil {
		api.sessionCloser.CloseSession(token)
	}
	return nil
}
func (api *AuthAPI) profile(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return &APIError{Status: 400, Message: "Invalid request body"}
	}
	session, err := api.service.UpdateProfile(r.Context(), sessionToken(r), request.DisplayName)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, account.ErrUnknownSession) {
			status = http.StatusUnauthorized
		}
		writeErrorJSON(w, status, map[string]string{"message": err.Error()})
		return nil
	}
	session.SessionToken = ""
	if api.presence != nil {
		api.presence.UpdateUserSession(session)
	}
	writeJSON(w, session)
	return nil
}
func (api *AuthAPI) inviteMetadata(w http.ResponseWriter, r *http.Request) error {
	value := api.service.InviteMetadata(r.Context(), chi.URLParam(r, "secret"))
	if !value.Valid {
		writeJSONStatus(w, http.StatusNotFound, map[string]bool{"valid": false})
		return nil
	}
	writeJSON(w, value)
	return nil
}
func (api *AuthAPI) redeem(w http.ResponseWriter, r *http.Request) error {
	var request struct {
		Secret      string `json:"secret"`
		DisplayName string `json:"displayName"`
	}
	if err := decodeJSONBody(r, &request); err != nil {
		return &APIError{Status: 400, Message: "Invalid request body"}
	}
	session, err := api.service.RedeemInvite(r.Context(), request.Secret, request.DisplayName)
	if err != nil {
		writeErrorJSON(w, http.StatusUnauthorized, map[string]string{"message": "Invitation is invalid or expired"})
		return nil
	}
	cookies, err := api.cookies.EstablishMember(r, session.SessionToken)
	if err != nil {
		return err
	}
	for _, cookie := range cookies {
		http.SetCookie(w, cookie)
	}
	writeJSON(w, map[string]any{"publicId": session.PublicID, "displayName": session.DisplayName, "role": session.Role, "guest": session.Guest, "enabled": session.Enabled, "lastLoginAt": session.LastLoginAt})
	return nil
}
func decodeJSONBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type loginAttempt struct {
	failures            int
	first, blockedUntil time.Time
}
type loginLimiter struct {
	mu              sync.Mutex
	enabled         bool
	max, maxTracked int
	window, block   time.Duration
	items           map[string]loginAttempt
}

func newLoginLimiter(cfg config.Config) *loginLimiter {
	return &loginLimiter{enabled: cfg.Auth.RateLimitEnabled, max: cfg.Auth.MaxAttempts, maxTracked: cfg.Auth.MaxTrackedIPs, window: cfg.Auth.Window, block: cfg.Auth.BlockDuration, items: map[string]loginAttempt{}}
}
func (l *loginLimiter) blocked(ip string) (string, bool) {
	if !l.enabled {
		return "", false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	item := l.items[ip]
	remaining := time.Until(item.blockedUntil)
	if remaining <= 0 {
		return "", false
	}
	return fmt.Sprintf("%d", max(1, int(remaining.Seconds()+0.999))), true
}
func (l *loginLimiter) failure(ip string) {
	if !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	item := l.items[ip]
	if item.first.IsZero() || now.Sub(item.first) > l.window {
		item = loginAttempt{first: now}
	}
	item.failures++
	if item.failures >= l.max {
		item.blockedUntil = now.Add(l.block)
	}
	if len(l.items) < l.maxTracked || l.items[ip].failures > 0 {
		l.items[ip] = item
	}
}
func (l *loginLimiter) success(ip string) { l.mu.Lock(); delete(l.items, ip); l.mu.Unlock() }
