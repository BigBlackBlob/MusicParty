package httpapi

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMemberCookiesMatchJavaAttributes(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://music.example/api/account/login", nil)
	cookies, err := (CookieFactory{SecureCookies: true}).EstablishMember(request, "session")
	if err != nil {
		t.Fatalf("EstablishMember() error = %v", err)
	}
	if len(cookies) != 2 || cookies[0].Name != SessionCookieName || !cookies[0].HttpOnly || cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].Path != "/" || cookies[0].MaxAge != memberSessionSeconds {
		t.Fatalf("unexpected session cookie: %+v", cookies[0])
	}
	if cookies[1].Name != CSRFCookieName || cookies[1].HttpOnly || cookies[1].Value == "" || cookies[1].MaxAge != memberSessionSeconds {
		t.Fatalf("unexpected CSRF cookie: %+v", cookies[1])
	}
}

func TestSecureCookieResolutionMatchesJava(t *testing.T) {
	factory := CookieFactory{SecureCookies: true}
	request := httptest.NewRequest(http.MethodPost, "http://music.example/api/account/login", nil)
	request.Header.Set("X-Forwarded-Proto", "https, http")
	cookies, err := factory.EstablishAdmin(request, "session")
	if err != nil {
		t.Fatalf("EstablishAdmin() error = %v", err)
	}
	if !cookies[0].Secure || cookies[0].MaxAge != adminSessionSeconds {
		t.Fatalf("unexpected admin cookie: %+v", cookies[0])
	}

	tlsRequest := httptest.NewRequest(http.MethodPost, "https://music.example/api/account/login", nil)
	tlsRequest.TLS = &tls.ConnectionState{}
	disabled, err := (CookieFactory{SecureCookies: false}).EstablishMember(tlsRequest, "session")
	if err != nil {
		t.Fatalf("EstablishMember() error = %v", err)
	}
	if disabled[0].Secure {
		t.Fatal("secure cookie was not explicitly disabled")
	}
}

func TestElevationAndClearCookiePaths(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://music.example/api/admin/auth/elevate", nil)
	factory := CookieFactory{}
	elevation := factory.EstablishElevation(request, "elevated")
	if elevation.Path != "/api/admin" || elevation.MaxAge != adminElevationSeconds || elevation.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected elevation cookie: %+v", elevation)
	}
	cleared := factory.Clear(request)
	if len(cleared) != 4 || cleared[0].MaxAge >= 0 || cleared[1].MaxAge >= 0 || cleared[2].Path != "/api/admin" || cleared[3].Name != RoomAccessCookieName || cleared[3].MaxAge >= 0 {
		t.Fatalf("unexpected clear cookies: %+v", cleared)
	}
}
