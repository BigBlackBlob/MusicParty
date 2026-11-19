package netease

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchMapsJavaCompatibleMusic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "contract", r.URL.Query().Get("keywords"))
		require.Equal(t, "secret", r.URL.Query().Get("cookie"))
		_, _ = w.Write([]byte(`{"result":{"songs":[{"id":42,"name":"Song","dt":1234,"ar":[{"name":"Artist"}],"al":{"picUrl":"http://img.test/a.jpg"}}]}}`))
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 1024}, server.URL, "secret")
	songs, err := service.Search(context.Background(), "contract", 0, 20)
	require.NoError(t, err)
	require.Equal(t, []string{"Artist"}, songs[0].Artists)
	require.Equal(t, "42", songs[0].ID)
	require.Equal(t, "https://img.test/a.jpg?param=1000y1000", songs[0].CoverURL)
}

// The desktop album view needs a real page and the upstream's own match count. Measured against
// the deployed netease-api on 2026-09-25: offset 0 and offset 5 return different ids and both
// report albumCount 592, so `total` is a whole-session count rather than a page size.
func TestSearchTrackURLs(t *testing.T) {
	const id = "28816031"
	const link = "https://music.163.com/song?id=" + id
	const song = `{"songs":[{"id":` + id + `,"name":"Track","dt":62000,"ar":[{"name":"Artist"}],"al":{"picUrl":"http://img.test/cover.jpg"}}]}`
	want := []platform.Music{{ID: id, Name: "Track", Artists: []string{"Artist"}, Duration: 62000, Platform: "netease", CoverURL: "https://img.test/cover.jpg?param=1000y1000"}}
	for _, test := range []struct {
		name, keyword string
		status        int
		body          string
		offset        int
		fallback      bool
	}{
		{name: "hash route", keyword: "https://music.163.com/#/song?id=" + id, body: song},
		{name: "fragment query takes precedence", keyword: "https://music.163.com/?id=42#/song?foo=bar&id=" + id, body: song},
		{name: "plain query", keyword: link, body: song},
		{name: "mobile", keyword: "https://y.music.163.com/m/song?id=" + id, body: song},
		{name: "later page", keyword: link, offset: 1},
		{name: "empty songs", keyword: link, body: `{"songs":[]}`},
		{name: "not found", keyword: link, status: http.StatusNotFound},
		{name: "unauthorized", keyword: link, status: http.StatusUnauthorized},
		{name: "upstream failure", keyword: link, status: http.StatusInternalServerError},
		{name: "profile", keyword: "https://music.163.com/user/home?id=" + id, fallback: true},
		{name: "playlist", keyword: "https://music.163.com/#/playlist?id=" + id, fallback: true},
		{name: "wrong host", keyword: "https://music.163.com.other.test/song?id=" + id, fallback: true},
		{name: "http scheme", keyword: "http://music.163.com/song?id=" + id, fallback: true},
		{name: "invalid numeric ID", keyword: "https://music.163.com/song?id=123a", fallback: true},
		{name: "missing ID", keyword: "https://music.163.com/song?name=track", fallback: true},
		{name: "invalid fragment ID", keyword: "https://music.163.com/#/song?id=123%26foo=bar", fallback: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				query := r.URL.Query()
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "secret", query.Get("cookie"))
				if test.fallback {
					assert.Equal(t, "/cloudsearch", r.URL.Path)
					assert.Equal(t, test.keyword, query.Get("keywords"))
					_, _ = w.Write([]byte(`{"result":{"songs":[]}}`))
					return
				}
				assert.Equal(t, "/song/detail", r.URL.Path)
				assert.Equal(t, id, query.Get("ids"))
				if test.status != 0 {
					w.WriteHeader(test.status)
				}
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)
			client := platform.NewClient(time.Second)
			client.HTTP.Transport = rewriteToFixture{base: server.URL}
			client.Retries = 0
			service := New(client, "https://netease-api.test", "secret")
			tracks, err := service.Search(t.Context(), test.keyword, test.offset, 20)
			switch {
			case test.offset != 0:
				require.NoError(t, err)
				require.Equal(t, []platform.Music{}, tracks)
				require.Zero(t, calls.Load())
			case test.status == http.StatusUnauthorized || test.status == http.StatusInternalServerError:
				var upstream *platform.UpstreamError
				require.ErrorAs(t, err, &upstream)
				require.Equal(t, test.status, upstream.Status)
				require.Empty(t, tracks)
			case test.body == song:
				require.NoError(t, err)
				require.Equal(t, want, tracks)
			default:
				require.NoError(t, err)
				require.Equal(t, []platform.Music{}, tracks)
			}
			if test.offset == 0 {
				require.EqualValues(t, 1, calls.Load())
			}
		})
	}
}

func TestSearchAlbumsPagesAndReportsTotal(t *testing.T) {
	var askedOffset, askedLimit string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		askedOffset, askedLimit = r.URL.Query().Get("offset"), r.URL.Query().Get("limit")
		require.Equal(t, "10", r.URL.Query().Get("type"))
		_, _ = w.Write([]byte(`{"result":{"albumCount":592,"albums":[{"id":7,"name":"A","size":12,"picUrl":"http://img.test/a.jpg","artist":{"name":"Singer"}}]}}`))
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 4096}, server.URL, "secret")

	page, err := service.SearchAlbums(context.Background(), "周杰伦", 30, 10)
	require.NoError(t, err)
	require.Equal(t, "30", askedOffset)
	require.Equal(t, "10", askedLimit)
	require.Equal(t, 592, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, "7", page.Items[0].ID)
	require.Equal(t, "Singer", page.Items[0].ArtistName)
	require.Equal(t, 12, page.Items[0].TrackCount)
	require.Equal(t, "netease", page.Items[0].Platform)

	// The web route asks for no page at all and must not start sending one, so its response stays
	// whatever the upstream default is.
	_, err = service.SearchAlbums(context.Background(), "周杰伦", 0, 0)
	require.NoError(t, err)
	require.Equal(t, "", askedOffset)
	require.Equal(t, "", askedLimit)
}

// Line-level slots must always come from legacy /lyric (the live /lyric/new
// rewrites credits as JSON lines), while /lyric/new only augments word fields.
func TestLyricMapsLineLevelFromLegacyAndWordLevelFromNew(t *testing.T) {
	const line = `{"lrc":{"lyric":"a"},"tlyric":{"lyric":"b"},"romalrc":{"lyric":"c"}}`
	cases := []struct {
		name     string
		lineBody string
		wordBody string
		want     platform.Lyric
		wantKey  bool
	}{
		{name: "all-variants", lineBody: line, wordBody: `{"lrc":{"lyric":"drift"},"yrc":{"lyric":"[0,4440](0,1320,0)Lately"},"ytlrc":{"lyric":"[0,4440](0,1320,0)最近"},"yromalrc":{"lyric":"[0,4440](0,1320,0)lieshi"}}`,
			want: platform.Lyric{Lyric: "a", TranslatedLyric: "b", RomanizedLyric: "c", WordLyric: "[0,4440](0,1320,0)Lately", WordTranslated: "[0,4440](0,1320,0)最近", WordRomanized: "[0,4440](0,1320,0)lieshi"}, wantKey: true},
		{name: "line-level-only", lineBody: line, wordBody: `{"lrc":{"lyric":"drift"}}`,
			want: platform.Lyric{Lyric: "a", TranslatedLyric: "b", RomanizedLyric: "c"}},
		{name: "yrc-without-word-translation", lineBody: line, wordBody: `{"yrc":{"lyric":"w"}}`,
			want: platform.Lyric{Lyric: "a", TranslatedLyric: "b", RomanizedLyric: "c", WordLyric: "w"}, wantKey: true},
		{name: "empty-response", lineBody: `{}`, wordBody: `{}`, want: platform.Lyric{}},
		{name: "new-route-unknown-degrades-silently", lineBody: line, wordBody: `{"code":400,"message":"unknown uri"}`,
			want: platform.Lyric{Lyric: "a", TranslatedLyric: "b", RomanizedLyric: "c"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				if r.URL.Path == "/lyric" {
					_, _ = w.Write([]byte(test.lineBody))
				} else {
					_, _ = w.Write([]byte(test.wordBody))
				}
			}))
			defer server.Close()
			service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 4096}, server.URL, "secret")
			value, err := service.Lyric(context.Background(), "1")
			require.NoError(t, err)
			require.Equal(t, test.want, value)
			require.Equal(t, []string{"/lyric", "/lyric/new"}, paths)
			encoded, err := json.Marshal(value)
			require.NoError(t, err)
			require.Equal(t, test.wantKey, strings.Contains(string(encoded), `"wordLyric"`))
			require.Contains(t, string(encoded), `"lyric"`)
		})
	}
}

func TestLyricPropagatesLegacyRouteFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 4096}, server.URL, "secret")
	_, err := service.Lyric(context.Background(), "1")
	require.Error(t, err)
}

func TestResolvePlayableReturnsCanonicalMetadataAndProxyURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/song/detail", r.URL.Path)
		require.Equal(t, "28816031", r.URL.Query().Get("ids"))
		_, _ = w.Write([]byte(`{"songs":[{"id":28816031,"name":"Cling Cling","dt":253000,"ar":[{"name":"中田ヤスタカ"}],"al":{"picUrl":"http://img.test/cover.jpg"}}]}`))
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 2048}, server.URL, "secret")
	playable, err := service.ResolvePlayable(context.Background(), "28816031")
	require.NoError(t, err)
	require.Equal(t, "Cling Cling", playable.Name)
	require.Equal(t, []string{"中田ヤスタカ"}, playable.Artists)
	require.Equal(t, int64(253000), playable.Duration)
	require.Equal(t, "/api/netease/stream/28816031", playable.URL)
	require.Equal(t, "https://img.test/cover.jpg?param=1000y1000", playable.CoverURL)
}

func TestMissingCookieDoesNotCallUpstream(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer server.Close()

	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 1024}, server.URL, "")
	for _, keyword := range []string{"contract", "https://music.163.com/#/song?id=28816031"} {
		_, err := service.Search(t.Context(), keyword, 0, 20)
		var credentialErr *platform.CredentialNotConfiguredError
		require.ErrorAs(t, err, &credentialErr)
		require.Equal(t, "netease", credentialErr.Platform)
	}
	require.Zero(t, calls)
}

func TestStreamResolution(t *testing.T) {
	cases := []struct {
		name       string
		quality    string
		responses  map[string]string
		wantURL    string
		wantKind   string
		wantLevels []string
	}{
		{name: "full", quality: "exhigh", responses: map[string]string{"exhigh": `{"data":[{"id":42,"url":"https://cdn.test/full","code":200,"expi":1200,"br":320000,"type":"mp3","size":10,"freeTrialInfo":null,"freeTrialPrivilege":{"cannotListenReason":0}}]}`}, wantURL: "https://cdn.test/full", wantLevels: []string{"exhigh"}},
		{name: "trial", quality: "exhigh", responses: map[string]string{"exhigh": `{"data":[{"id":42,"url":"https://cdn.test/preview","code":200,"expi":1200,"freeTrialInfo":{"start":0,"end":30},"freeTrialPrivilege":{"cannotListenReason":1}}]}`}, wantKind: platform.StreamTrialOnly, wantLevels: []string{"exhigh"}},
		{name: "step down", quality: "exhigh", responses: map[string]string{"exhigh": `{"data":[]}`, "higher": `{"data":[{"id":42,"url":"https://cdn.test/lower","code":200,"expi":1200}]}`}, wantURL: "https://cdn.test/lower", wantLevels: []string{"exhigh", "higher"}},
		{name: "invalid quality", quality: "invalid", responses: map[string]string{"exhigh": `{"data":[{"id":42,"url":"https://cdn.test/default","code":200,"expi":1200}]}`}, wantURL: "https://cdn.test/default", wantLevels: []string{"exhigh"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var levels []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NotEmpty(t, r.URL.Query().Get("timestamp"))
				levels = append(levels, r.URL.Query().Get("level"))
				_, _ = w.Write([]byte(test.responses[r.URL.Query().Get("level")]))
			}))
			defer server.Close()
			service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 2048}, server.URL, "secret", test.quality)
			got, err := service.StreamURL(context.Background(), "42")
			require.Equal(t, test.wantURL, got)
			if test.wantKind != "" {
				var streamErr *platform.StreamResolutionError
				require.ErrorAs(t, err, &streamErr)
				require.Equal(t, test.wantKind, streamErr.Kind)
				if test.wantKind == platform.StreamTrialOnly {
					require.Equal(t, 1, streamErr.Reason)
				}
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantLevels, levels)
		})
	}
}

func TestStreamCacheExpiryInvalidationAndCredentialUpdate(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.NotEmpty(t, r.URL.Query().Get("timestamp"))
		_, _ = w.Write([]byte(`{"data":[{"id":42,"url":"https://cdn.test/stream","code":200,"expi":180}]}`))
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 2048}, server.URL, "first")
	now := time.Unix(100, 0)
	service.now = func() time.Time { return now }
	for i := 0; i < 2; i++ {
		_, err := service.StreamURL(context.Background(), "42")
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, calls.Load())
	now = now.Add(61 * time.Second) // expi minus two-minute margin is clamped to one second.
	_, err := service.StreamURL(context.Background(), "42")
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load())
	service.InvalidateStream("42")
	_, err = service.StreamURL(context.Background(), "42")
	require.NoError(t, err)
	require.EqualValues(t, 3, calls.Load())
	service.UpdateCredential("second")
	_, err = service.StreamURL(context.Background(), "42")
	require.NoError(t, err)
	require.EqualValues(t, 4, calls.Load())
}

func TestConcurrentStreamRequestsShareUpstreamCall(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	proceed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-proceed
		_, _ = w.Write([]byte(`{"data":[{"id":42,"url":"https://cdn.test/shared","code":200,"expi":1200}]}`))
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 2048}, server.URL, "cookie")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := service.StreamURL(context.Background(), "42")
			require.NoError(t, err)
			require.Equal(t, "https://cdn.test/shared", got)
		}()
	}
	<-started
	close(proceed)
	wg.Wait()
	require.EqualValues(t, 1, calls.Load())
}

type rewriteToFixture struct{ base string }

func (t rewriteToFixture) RoundTrip(request *http.Request) (*http.Response, error) {
	fixture, err := url.Parse(t.base)
	if err != nil {
		return nil, err
	}
	forked := request.Clone(request.Context())
	forked.URL.Scheme = fixture.Scheme
	forked.URL.Host = fixture.Host
	forked.Host = ""
	return http.DefaultTransport.RoundTrip(forked)
}
