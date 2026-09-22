//go:build live

package netease

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestLiveNeteaseWordLyrics(t *testing.T) {
	baseURL := os.Getenv("NETEASE_LIVE_URL")
	cookieBytes, err := os.ReadFile(os.Getenv("NETEASE_LIVE_COOKIE_FILE"))
	require.NoError(t, err)
	service := New(&platform.Client{HTTP: http.DefaultClient, Retries: 0, MaxBody: 1 << 20}, baseURL, string(cookieBytes))

	withYRC, err := service.Lyric(context.Background(), "28816031")
	require.NoError(t, err)
	require.NotEmpty(t, withYRC.Lyric)
	require.NotEmpty(t, withYRC.WordLyric)
	require.Contains(t, withYRC.WordLyric, "(3580,490,0)Cling")
	require.Contains(t, withYRC.WordLyric, `{"t":0,"c":[{"tx":"作词: "}`)
	withJSON, err := json.Marshal(withYRC)
	require.NoError(t, err)
	t.Logf("yrc bytes=%d total json=%d", len(withYRC.WordLyric), len(withJSON))

	withoutYRC, err := service.Lyric(context.Background(), "186016")
	require.NoError(t, err)
	require.NotEmpty(t, withoutYRC.Lyric)
	require.Empty(t, withoutYRC.WordLyric)
	withoutJSON, err := json.Marshal(withoutYRC)
	require.NoError(t, err)
	require.NotContains(t, string(withoutJSON), "wordLyric")
	t.Logf("legacy-shape json bytes=%d", len(withoutJSON))
}
