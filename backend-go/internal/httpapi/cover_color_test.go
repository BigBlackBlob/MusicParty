package httpapi

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestUpstreamCoverURL(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"https://p1.music.126.net/cover.jpg?param=1000y1000", "https://p1.music.126.net/cover.jpg?param=128y128"},
		{"http://i1.hdslb.com/bfs/archive/a.jpg", "https://i1.hdslb.com/bfs/archive/a.jpg@128w_128h_1c.jpg"},
		{"https://i0.hdslb.com/a.jpg@1000w.webp?x=1", "https://i0.hdslb.com/a.jpg@128w_128h_1c.jpg"},
		{"https://music.126.net:443/a", "https://music.126.net/a?param=128y128"},
		{"https://HDSLB.COM/a", "https://hdslb.com/a@128w_128h_1c.jpg"},
		{"https://evilmusic.126.net.attacker.com/a", ""},
		{"https://evilmusic.126.net/a", ""},
		{"https://hdslb.com.attacker.com/a", ""},
		{"https://evilhdslb.com/a", ""},
		{"ftp://p1.music.126.net/a", ""}, {"//i1.hdslb.com/a", ""},
		{"https://user@i1.hdslb.com/a", ""}, {"https://i1.hdslb.com:8443/a", ""},
		{"http://i1.hdslb.com:80/a", ""}, {"https://127.0.0.1/a", ""},
		{"https://[::1]/a", ""}, {"https://i1.hdslb.com./a", ""},
		{"https://.hdslb.com/a", ""}, {"https://i1.hdslb.com:/a", ""},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := upstreamCoverURL(tc.input)
			if tc.want == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestPublicCoverControl(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1", "169.254.169.254", "100.64.0.1", "192.0.2.1", "198.18.0.1", "224.0.0.1", "0.0.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "fe80::1", "2001:db8::1", "2002:7f00:1::"} {
		require.Error(t, publicCoverControl("tcp", net.JoinHostPort(ip, "443"), nil), ip)
	}
	for _, ip := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		require.NoError(t, publicCoverControl("tcp", net.JoinHostPort(ip, "443"), nil))
	}
}

// A local DNS fixture returns a forbidden address for a valid CDN hostname.
// The production dialer must reject it after resolution and before connect.
func TestCoverDialRejectsPrivateDNS(t *testing.T) {
	dns, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	defer dns.Close()
	go func() {
		buffer := make([]byte, 512)
		for {
			n, peer, err := dns.ReadFrom(buffer)
			if err != nil {
				return
			}
			query := append([]byte(nil), buffer[:n]...)
			end := 12
			for end < len(query) && query[end] != 0 {
				end += int(query[end]) + 1
			}
			if end+5 > len(query) {
				continue
			}
			end += 5
			kind := binary.BigEndian.Uint16(query[end-4 : end-2])
			reply := append([]byte(nil), query[:end]...)
			binary.BigEndian.PutUint16(reply[2:4], 0x8180)
			binary.BigEndian.PutUint16(reply[6:8], 0)
			if kind == 1 {
				binary.BigEndian.PutUint16(reply[6:8], 1)
				reply = append(reply, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 0, 0, 4, 127, 0, 0, 1)
			}
			_, _ = dns.WriteTo(reply, peer)
		}
	}()
	client := newCoverClient()
	defer client.CloseIdleConnections()
	dialer := &net.Dialer{
		Control: publicCoverControl,
		Resolver: &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", dns.LocalAddr().String())
		}},
	}
	client.Transport.(*http.Transport).DialContext = dialer.DialContext
	_, err = client.Get("https://p1.music.126.net/cover.jpg")
	require.ErrorContains(t, err, "non-public cover address")
}

func TestDominantCoverColor(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{128, 128, 128, 255})
		}
	}
	palette, err := dominantCoverColor(img)
	require.NoError(t, err)
	require.Equal(t, "#808080", palette.Accent)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{224, 32, 64, 255})
		}
	}
	palette, err = dominantCoverColor(img)
	require.NoError(t, err)
	require.Equal(t, coverPalette{"#e02040", "#e02040", "rgba(224, 32, 64, 0.16)", "rgba(224, 32, 64, 0.08)"}, palette)
}

func TestUpstreamCoverCacheAndSession(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Equal(t, "/a.jpg@128w_128h_1c.jpg", r.URL.Path)
		require.NotEmpty(t, r.UserAgent())
		require.Empty(t, r.Header.Get("Cookie"))
		require.Empty(t, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Referer"))
		w.Header().Set("Content-Type", "image/png")
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{32, 64, 128, 255})
		_ = png.Encode(w, img)
	}))
	defer upstream.Close()
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "cover-token", "listener", "MEMBER")
	api := NewPlatformAPI(testConfig(t))
	api.SetAccounts(accounts)
	// Only this fixture redirects TLS connections to a local server. Production
	// uses publicCoverControl, covered by the DNS test above.
	api.coverClient = upstream.Client()
	api.coverClient.Transport.(*http.Transport).TLSClientConfig.ServerName = "example.com"
	api.coverClient.Transport.(*http.Transport).DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", upstream.Listener.Addr().String())
	}
	router := chi.NewRouter()
	api.Routes(router)
	path := "/api/theme/extract-cover-color?url=" + url.QueryEscape("http://i1.hdslb.com/a.jpg")
	w := batch2Request(router, "GET", path, "", "", "")
	require.Equal(t, http.StatusUnauthorized, w.Code)
	for range 2 {
		w = batch2Request(router, "GET", path, "", "cover-token", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.JSONEq(t, `{"accent":"#204080","accentHover":"#204080","accentMuted":"rgba(32, 64, 128, 0.16)","accentSubtle":"rgba(32, 64, 128, 0.08)"}`, w.Body.String())
	}
	require.EqualValues(t, 1, calls.Load())
	target, err := upstreamCoverURL("http://i1.hdslb.com/a.jpg")
	require.NoError(t, err)
	api.colorsMu.Lock()
	api.colors[target] = cachedCoverColor{expiresAt: time.Now().Add(-time.Second)}
	api.colorsMu.Unlock()
	w = batch2Request(router, "GET", path, "", "cover-token", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.EqualValues(t, 2, calls.Load())
}

func TestLiveCoverColors(t *testing.T) {
	if os.Getenv("MUSICPARTY_LIVE_COVERS") != "1" {
		t.Skip("explicit live verification only")
	}
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "live-cover-token", "listener", "MEMBER")
	api := NewPlatformAPI(testConfig(t))
	api.SetAccounts(accounts)
	router := chi.NewRouter()
	api.Routes(router)
	defer api.coverClient.CloseIdleConnections()
	for _, raw := range []string{
		"https://p1.music.126.net/6y-UleORITEDbvrOLV0Q8A==/5639395138885805.jpg",
		"http://i1.hdslb.com/bfs/archive/5242750857121e05146d5d5b13a47a2a6dd36e98.jpg",
	} {
		t.Run(raw, func(t *testing.T) {
			target, err := upstreamCoverURL(raw)
			require.NoError(t, err)
			w := batch2Request(router, "GET", "/api/theme/extract-cover-color?url="+url.QueryEscape(raw), "", "live-cover-token", "")
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var palette coverPalette
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &palette))
			t.Logf("GET /api/theme/extract-cover-color?url=%s -> HTTP %d %s (upstream %s)", url.QueryEscape(raw), w.Code, w.Body.String(), target)
		})
	}
}

func TestCoverFailuresAreNotCachedOrRedirected(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/redirect-target", http.StatusFound)
	}))
	defer upstream.Close()
	api := NewPlatformAPI(testConfig(t))
	api.coverClient = &http.Client{Timeout: 5 * time.Second, CheckRedirect: noCoverRedirect}
	for range 2 {
		_, err := api.upstreamColor(context.Background(), upstream.URL)
		require.Error(t, err)
	}
	require.EqualValues(t, 2, calls.Load(), "each failed request retries once without following redirects")
	require.Empty(t, api.colors)
}

func TestCoverCacheEvictsAtCapacity(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{32, 64, 128, 255})
		_ = png.Encode(w, img)
	}))
	defer upstream.Close()
	api := NewPlatformAPI(testConfig(t))
	api.coverClient = upstream.Client()
	for i := range 256 {
		api.colors[fmt.Sprint(i)] = cachedCoverColor{expiresAt: time.Now().Add(time.Minute + time.Duration(i)*time.Second)}
	}
	_, err := api.upstreamColor(context.Background(), upstream.URL)
	require.NoError(t, err)
	require.Len(t, api.colors, 256)
	require.NotContains(t, api.colors, "0")
	require.Contains(t, api.colors, upstream.URL)
}

func TestCoverAuthenticationCanBeConfiguredAfterRoutes(t *testing.T) {
	api := NewPlatformAPI(testConfig(t))
	router := chi.NewRouter()
	api.Routes(router)
	path := "/api/theme/extract-cover-color?url=https://evil.test/x"
	assertUnauthorized := func(token string) {
		t.Helper()
		w := batch2Request(router, "GET", path, "", token, "")
		require.Equal(t, http.StatusUnauthorized, w.Code)
		require.JSONEq(t, `{"message":"Authentication required"}`, w.Body.String())
	}
	assertUnauthorized("cover-token")
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "cover-token", "listener", "MEMBER")
	api.SetAccounts(accounts)
	assertUnauthorized("")
	assertUnauthorized("invalid-token")
	w := batch2Request(router, "GET", path, "", "cover-token", "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	api.SetAccounts(nil)
	assertUnauthorized("cover-token")
}
