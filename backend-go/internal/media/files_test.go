package media

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServeFileRanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audio.bin")
	require.NoError(t, os.WriteFile(path, []byte("0123456789"), 0o600))
	tests := []struct {
		rangeHeader, body, contentRange string
		status                          int
	}{
		{"", "0123456789", "", 200},
		{"bytes=2-5", "2345", "bytes 2-5/10", 206},
		{"bytes=-3", "789", "bytes 7-9/10", 206},
		{"bytes=20-", "", "bytes */10", 416},
	}
	for _, test := range tests {
		r := httptest.NewRequest(http.MethodGet, "/media", nil)
		r.Header.Set("Range", test.rangeHeader)
		w := httptest.NewRecorder()
		require.NoError(t, ServeFile(w, r, path, "audio/mpeg", "no-store"))
		require.Equal(t, test.status, w.Code)
		require.Equal(t, test.body, w.Body.String())
		require.Equal(t, test.contentRange, w.Header().Get("Content-Range"))
	}
}

func TestResolveWithinRejectsTraversal(t *testing.T) {
	_, err := ResolveWithin(t.TempDir(), "../secret")
	require.ErrorIs(t, err, ErrPathOutsideRoot)
}
