package httpapi

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxPlaylistCoverBytes = 2 << 20

func (api *PlaylistAPI) userUploadCover(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	data, err := readPlaylistCoverUpload(w, r)
	if err != nil {
		return err
	}
	id := chi.URLParam(r, "playlistId")
	coverURL := "/api/me/playlists/" + url.PathEscape(id) + "/cover?v=" + uuid.NewString()
	changed, err := api.users.SetCover(r.Context(), session.PublicID, id, coverURL, data)
	return noContentOrNotFound(w, changed, err)
}

func (api *PlaylistAPI) roomUploadCover(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomAccess(r); err != nil {
		return roomAccessError(w, err)
	}
	data, err := readPlaylistCoverUpload(w, r)
	if err != nil {
		return err
	}
	room, id := chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId")
	coverURL := "/api/rooms/" + url.PathEscape(room) + "/playlists/" + url.PathEscape(id) + "/cover?v=" + uuid.NewString()
	changed, err := api.roomPlaylists.SetCover(r.Context(), room, id, coverURL, data)
	return noContentOrNotFound(w, changed, err)
}

func readPlaylistCoverUpload(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPlaylistCoverBytes+64<<10)
	if err := r.ParseMultipartForm(maxPlaylistCoverBytes); err != nil {
		return nil, badRequest("Invalid cover upload")
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("file")
	if err != nil {
		return nil, badRequest("Cover file is required")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPlaylistCoverBytes+1))
	if err != nil || len(data) > maxPlaylistCoverBytes {
		return nil, badRequest("Cover must be 2 MiB or smaller")
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" {
		return nil, badRequest("Cover must be JPEG or PNG")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 {
		return nil, badRequest("Invalid cover or dimensions exceed 4096 pixels")
	}
	return data, nil
}

func (api *PlaylistAPI) userCover(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	data, err := api.users.Cover(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"))
	return writePlaylistCover(w, r, data, err)
}

func (api *PlaylistAPI) roomCover(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomAccess(r); err != nil {
		return roomAccessError(w, err)
	}
	data, err := api.roomPlaylists.Cover(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"))
	return writePlaylistCover(w, r, data, err)
}

func writePlaylistCover(w http.ResponseWriter, r *http.Request, data []byte, err error) error {
	if err != nil {
		return err
	}
	if len(data) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "cover", time.Time{}, bytes.NewReader(data))
	return nil
}

func requestPlaylistNotes(r *http.Request) (string, error) {
	var body struct {
		Notes *string `json:"notes"`
	}
	if err := decodeJSONBody(r, &body); err != nil || body.Notes == nil || utf8.RuneCountInString(*body.Notes) > 1000 {
		return "", badRequest("notes must be a string of up to 1000 characters")
	}
	return *body.Notes, nil
}

func (api *PlaylistAPI) userTrackNotes(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	notes, err := requestPlaylistNotes(r)
	if err != nil {
		return err
	}
	changed, err := api.users.SetTrackNotes(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), chi.URLParam(r, "trackId"), notes)
	return noContentOrNotFound(w, changed, err)
}

func (api *PlaylistAPI) roomTrackNotes(w http.ResponseWriter, r *http.Request) error {
	if err := api.roomAccess(r); err != nil {
		return roomAccessError(w, err)
	}
	notes, err := requestPlaylistNotes(r)
	if err != nil {
		return err
	}
	changed, err := api.roomPlaylists.SetTrackNotes(r.Context(), chi.URLParam(r, "roomId"), chi.URLParam(r, "playlistId"), chi.URLParam(r, "trackId"), notes)
	return noContentOrNotFound(w, changed, err)
}

func (api *PlaylistAPI) userPlaylistOrder(w http.ResponseWriter, r *http.Request) error {
	session, err := api.user(r)
	if err != nil {
		return unauthorized(w)
	}
	var body struct {
		Position *int `json:"position"`
	}
	if err := decodeJSONBody(r, &body); err != nil || body.Position == nil || *body.Position < 0 {
		return badRequest("position must be a non-negative integer")
	}
	changed, err := api.users.MovePlaylist(r.Context(), session.PublicID, chi.URLParam(r, "playlistId"), *body.Position)
	return noContentOrNotFound(w, changed, err)
}
