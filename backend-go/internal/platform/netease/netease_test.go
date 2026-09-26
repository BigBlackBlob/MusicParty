package netease

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
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
	_, err := service.Search(context.Background(), "contract", 0, 20)

	var credentialErr *platform.CredentialNotConfiguredError
	require.ErrorAs(t, err, &credentialErr)
	require.Equal(t, "netease", credentialErr.Platform)
	require.Zero(t, calls)
}
