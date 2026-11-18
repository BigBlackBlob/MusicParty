package soundcloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

func TestAvailableRequiresClientID(t *testing.T) {
	service := New(platform.NewClient(time.Second), true, "", "")
	if service.Available() {
		t.Fatal("Available() = true with an empty client ID")
	}
}

func TestResolvePlayableUsesSoundCloudWidget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tracks/123" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("client_id"); got != "client-id" {
			t.Errorf("client_id = %q", got)
		}
		if got := r.URL.Query().Get("oauth_token"); got != "test-token" {
			t.Errorf("oauth_token = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "OAuth test-token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"title":"Track","duration":180000,"artwork_url":"https://img.example/cover-large.jpg","streamable":true,"user":{"username":"Artist"}}`))
	}))
	defer server.Close()

	client := platform.NewClient(time.Second)
	client.HTTP = &http.Client{Transport: rewriteTransport{target: server.URL, base: server.Client().Transport}}
	service := New(client, true, "client-id", "")
	service.UpdateCredential("test-token")
	playable, err := service.ResolvePlayable(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if playable.PlayerType != "soundcloud-widget" || playable.EmbedID != "123" || playable.URL != "" {
		t.Fatalf("unexpected playable target: %+v", playable)
	}
	if playable.ID != "123" || playable.Platform != "soundcloud" || playable.Duration != 180000 {
		t.Fatalf("unexpected track metadata: %+v", playable.Music)
	}
}

type rewriteTransport struct {
	target string
	base   http.RoundTripper
}

func (r rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	target, err := url.Parse(r.target)
	if err != nil {
		return nil, err
	}
	copy := request.Clone(request.Context())
	copy.URL.Scheme = target.Scheme
	copy.URL.Host = target.Host
	copy.Host = target.Host
	return r.base.RoundTrip(copy)
}
