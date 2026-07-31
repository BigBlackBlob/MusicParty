package media

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrPathOutsideRoot = errors.New("media path is outside configured root")

// ResolveWithin confines a persisted relative path to root.
func ResolveWithin(root, relative string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathOutsideRoot
	}
	return target, nil
}

// ServeFile streams a regular file with single-range support and request cancellation.
func ServeFile(w http.ResponseWriter, r *http.Request, path, contentType, cacheControl string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return os.ErrNotExist
	}
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(path))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}
	start, end, partial, err := parseRange(r.Header.Get("Range"), info.Size())
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size()))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return nil
	}
	length := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	if partial {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size()))
		w.WriteHeader(http.StatusPartialContent)
	}
	if r.Method == http.MethodHead {
		return nil
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return err
	}
	_, err = io.CopyN(w, &contextReader{ctxDone: r.Context().Done(), reader: file}, length)
	return err
}

func parseRange(value string, size int64) (start, end int64, partial bool, err error) {
	if value == "" {
		return 0, size - 1, false, nil
	}
	if size <= 0 || !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return 0, 0, false, errors.New("invalid range")
	}
	parts := strings.SplitN(strings.TrimPrefix(value, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, errors.New("invalid range")
	}
	if parts[0] == "" {
		suffix, parseErr := strconv.ParseInt(parts[1], 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, false, errors.New("invalid range")
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true, nil
	}
	start, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, errors.New("invalid range")
	}
	end = size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, false, errors.New("invalid range")
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true, nil
}

type contextReader struct {
	ctxDone <-chan struct{}
	reader  io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctxDone:
		return 0, errors.New("request cancelled")
	default:
		return r.reader.Read(buffer)
	}
}
