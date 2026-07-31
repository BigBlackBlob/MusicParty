package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginAllowedRequiresOriginAndMatchesSameOrigin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://music.example/ws", nil)
	if OriginAllowed(request, nil) {
		t.Fatal("missing Origin was accepted")
	}
	request.Header.Set("Origin", "http://music.example")
	if !OriginAllowed(request, nil) {
		t.Fatal("same origin was rejected")
	}
}

func TestOriginAllowedMatchesConfiguredOriginExactly(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://music.internal/ws", nil)
	request.Header.Set("Origin", "https://music.example")
	if !OriginAllowed(request, []string{"https://music.example"}) {
		t.Fatal("configured origin was rejected")
	}
	request.Header.Set("Origin", "https://evil.example")
	if OriginAllowed(request, []string{"https://music.example"}) {
		t.Fatal("unconfigured origin was accepted")
	}
}
