package netease

import (
	"context"
	"net/http"
	"net/http/httptest"
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
func TestLyricDetailMapsAllVariants(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"lrc":{"lyric":"a"},"tlyric":{"lyric":"b"},"romalrc":{"lyric":"c"}}`))
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 1024}, server.URL, "secret")
	value, err := service.Lyric(context.Background(), "1")
	require.NoError(t, err)
	require.Equal(t, platform.Lyric{Lyric: "a", TranslatedLyric: "b", RomanizedLyric: "c"}, value)
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
