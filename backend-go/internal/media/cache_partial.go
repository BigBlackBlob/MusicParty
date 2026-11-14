package media

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

// minPartialBytes is the smallest prefix worth streaming.
const minPartialBytes = 256 * 1024

// IsInflight reports whether a background download is running for this key.
func (c *Cache) IsInflight(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.inflight[key]
	return ok
}

// InflightURL returns the URL and headers stored when the download was submitted.
func (c *Cache) InflightURL(key string) (string, http.Header, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.inflight[key]
	if !ok {
		return "", nil, false
	}
	return entry.url, entry.headers.Clone(), true
}

// ServePartial streams the downloaded prefix of an in-flight file. A false
// result means no response was written and the caller may use another source.
func (c *Cache) ServePartial(w http.ResponseWriter, r *http.Request, key, contentType string) (served bool, err error) {
	c.mu.Lock()
	_, inflight := c.inflight[key]
	c.mu.Unlock()
	if !inflight {
		return false, nil
	}
	part := c.pathForKey(key) + ".part"
	info, err := os.Stat(part)
	if err != nil || !info.Mode().IsRegular() || info.Size() < minPartialBytes {
		return false, nil
	}
	file, err := os.Open(part)
	if err != nil {
		return false, err
	}
	defer file.Close()
	return servePartialFile(w, r, file, info.Size(), contentType)
}

func servePartialFile(w http.ResponseWriter, r *http.Request, file *os.File, size int64, contentType string) (bool, error) {
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, os.ErrNotExist
	}
	// The initial stat bounds reads even if the downloader grows the file later.
	size = min(size, info.Size())
	if contentType == "" {
		var sniff [512]byte
		n, _ := file.ReadAt(sniff[:], 0)
		contentType = http.DetectContentType(sniff[:n])
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	start, end, partial, err := parseRange(r.Header.Get("Range"), size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return true, nil
	}
	length := end - start + 1
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	if partial {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.WriteHeader(http.StatusPartialContent)
	}
	if r.Method == http.MethodHead {
		return true, nil
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return false, err
	}
	_, err = io.CopyN(w, &contextReader{ctxDone: r.Context().Done(), reader: file}, length)
	return true, err
}
