package media

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
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
	entry := c.inflight[key]
	if entry == nil || entry.total < 0 {
		c.mu.Unlock()
		return false, nil
	}
	total, written, generation := entry.total, entry.written, entry.attempt
	if entry.failed || entry.done {
		c.mu.Unlock()
		return false, nil
	}
	c.mu.Unlock()
	start, _, _, rangeErr := parseRange(r.Header.Get("Range"), total)
	if rangeErr == nil && start >= written {
		return false, nil
	}
	if (rangeErr == nil && start == 0 || r.Header.Get("Range") == "") && written < minPartialBytes {
		return false, nil
	}
	part := c.pathForKey(key) + ".part"
	file, err := os.Open(part)
	if err != nil {
		return false, err
	}
	defer file.Close()
	return servePartialFile(c, entry, generation, w, r, file, total, contentType)
}

func servePartialFile(c *Cache, entry *inflightEntry, generation uint64, w http.ResponseWriter, r *http.Request, file *os.File, size int64, contentType string) (bool, error) {
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, os.ErrNotExist
	}
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
	} else {
		w.WriteHeader(http.StatusOK)
	}
	if r.Method == http.MethodHead {
		return true, nil
	}
	position := start
	buffer := make([]byte, 32*1024)
	for position <= end {
		if err := r.Context().Err(); err != nil {
			return true, nil
		}
		c.mu.Lock()
		currentAttempt, available, done, failed, changed := entry.attempt, entry.written, entry.done, entry.failed, entry.changed
		c.mu.Unlock()
		if currentAttempt != generation || failed {
			return true, errors.New("in-flight media download failed before the requested range was complete")
		}
		if available > position {
			want := min(int64(len(buffer)), min(available-position, end-position+1))
			n, readErr := file.ReadAt(buffer[:want], position)
			if n > 0 {
				written, writeErr := w.Write(buffer[:n])
				position += int64(written)
				if writeErr != nil || written != n {
					if writeErr != nil {
						return true, writeErr
					}
					return true, io.ErrShortWrite
				}
			}
			if readErr != nil && readErr != io.EOF {
				return true, readErr
			}
			continue
		}
		if done || available >= size {
			return true, io.ErrUnexpectedEOF
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-r.Context().Done():
			timer.Stop()
			return true, nil
		case <-changed:
			timer.Stop()
		case <-timer.C:
		}
	}
	return true, nil
}
