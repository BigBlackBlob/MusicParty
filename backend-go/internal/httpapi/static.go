package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// StaticAPI serves the built Vue application and falls back to index.html for
// client-side routes. API and operational paths remain owned by earlier routes.
type StaticAPI struct{ root string }

func NewStaticAPI(root string) *StaticAPI {
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	return &StaticAPI{root: absolute}
}

func (api *StaticAPI) Routes(r chi.Router) {
	r.Get("/", api.serve)
	r.Head("/", api.serve)
	r.Get("/*", api.serve)
	r.Head("/*", api.serve)
}

func (api *StaticAPI) serve(w http.ResponseWriter, r *http.Request) {
	if reservedStaticPath(r.URL.Path) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	relative := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(r.URL.Path)), "/")
	path := filepath.Join(api.root, filepath.FromSlash(relative))
	if relative == "" {
		path = filepath.Join(api.root, "index.html")
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		path = filepath.Join(api.root, "index.html")
	}
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	http.ServeFile(w, r, path)
}

func reservedStaticPath(path string) bool {
	for _, prefix := range []string{"/api/", "/actuator/", "/ws", "/radio/", "/media/", "/proxy/"} {
		if path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
