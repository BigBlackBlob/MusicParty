package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
)

const (
	SessionCookieName        = "MP_SESSION"
	CSRFCookieName           = "MP_CSRF"
	RoomAccessCookieName     = "MP_ROOM_ACCESS"
	AdminElevationCookieName = "MP_ADMIN_ELEVATION"
	CSRFHeaderName           = "X-CSRF-Token"

	memberSessionSeconds  = 90 * 24 * 60 * 60
	adminSessionSeconds   = 12 * 60 * 60
	adminElevationSeconds = 10 * 60
	roomAccessSeconds     = 5 * 60
)

type CookieFactory struct {
	SecureCookies bool
}

func (f CookieFactory) EstablishMember(r *http.Request, sessionToken string) ([]*http.Cookie, error) {
	csrfToken, err := randomCookieToken()
	if err != nil {
		return nil, err
	}
	return f.sessionPair(r, sessionToken, csrfToken, memberSessionSeconds), nil
}

func (f CookieFactory) EstablishAdmin(r *http.Request, sessionToken string) ([]*http.Cookie, error) {
	csrfToken, err := randomCookieToken()
	if err != nil {
		return nil, err
	}
	return f.sessionPair(r, sessionToken, csrfToken, adminSessionSeconds), nil
}

func (f CookieFactory) EstablishElevation(r *http.Request, value string) *http.Cookie {
	return &http.Cookie{
		Name: AdminElevationCookieName, Value: value, Path: "/api/admin", MaxAge: adminElevationSeconds,
		HttpOnly: true, Secure: f.secureForRequest(r), SameSite: http.SameSiteStrictMode,
	}
}

func (f CookieFactory) EstablishRoomAccess(r *http.Request, value string) *http.Cookie {
	return &http.Cookie{
		Name: RoomAccessCookieName, Value: value, Path: "/", MaxAge: roomAccessSeconds,
		HttpOnly: true, Secure: f.secureForRequest(r), SameSite: http.SameSiteLaxMode,
	}
}

func (f CookieFactory) Clear(r *http.Request) []*http.Cookie {
	secure := f.secureForRequest(r)
	return []*http.Cookie{
		{Name: SessionCookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode},
		{Name: CSRFCookieName, Path: "/", MaxAge: -1, HttpOnly: false, Secure: secure, SameSite: http.SameSiteStrictMode},
		{Name: AdminElevationCookieName, Path: "/api/admin", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode},
		{Name: RoomAccessCookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode},
	}
}

func (f CookieFactory) sessionPair(r *http.Request, sessionToken, csrfToken string, maxAge int) []*http.Cookie {
	secure := f.secureForRequest(r)
	return []*http.Cookie{
		{Name: SessionCookieName, Value: sessionToken, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode},
		{Name: CSRFCookieName, Value: csrfToken, Path: "/", MaxAge: maxAge, HttpOnly: false, Secure: secure, SameSite: http.SameSiteLaxMode},
	}
}

func (f CookieFactory) secureForRequest(r *http.Request) bool {
	if !f.SecureCookies {
		return false
	}
	if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		return strings.EqualFold(strings.TrimSpace(strings.SplitN(forwardedProto, ",", 2)[0]), "https")
	}
	return r.TLS != nil || strings.EqualFold(r.URL.Scheme, "https")
}

func randomCookieToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
