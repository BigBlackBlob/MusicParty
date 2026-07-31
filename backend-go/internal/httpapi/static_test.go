package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestStaticAPIServesAssetsAndSPAFallback(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("<main>MusicParty</main>"), 0o640))
	require.NoError(t, os.WriteFile(filepath.Join(root, "app.js"), []byte("console.log('ok')"), 0o640))
	router := chi.NewRouter()
	NewStaticAPI(root).Routes(router)
	for path, expected := range map[string]string{"/": "MusicParty", "/rooms/lounge": "MusicParty", "/app.js": "console.log"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, response.Code, path)
		require.Contains(t, response.Body.String(), expected, path)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}
