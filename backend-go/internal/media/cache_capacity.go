package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	errCacheCapacity    = errors.New("cache capacity is occupied by active downloads, readers or files that cannot be removed")
	errDownloadTooLarge = errors.New("cache download exceeds the single-file limit")
)

// scan runs before workers start. Modification time is the startup LRU baseline;
// subsequent Get calls maintain recency in memory.
func (c *Cache) scan() error {
	files, err := os.ReadDir(c.root)
	if err != nil {
		return err
	}
	type diskEntry struct {
		id string
		cacheEntry
		modified time.Time
	}
	var entries []diskEntry
	for _, file := range files {
		partial, complete := strings.HasSuffix(file.Name(), ".part"), strings.HasSuffix(file.Name(), ".media")
		if !partial && !complete {
			continue
		}
		info, err := file.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cache file %q is not a regular file", file.Name())
		}
		path := filepath.Join(c.root, file.Name())
		if partial {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove abandoned cache partial: %w", err)
			}
			continue
		}
		if info.Size() > (1<<63-1)-c.usedBytes {
			return errors.New("cache disk usage overflow")
		}
		c.usedBytes += info.Size()
		entries = append(entries, diskEntry{file.Name(), cacheEntry{path: path, size: info.Size()}, info.ModTime()})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].modified.Equal(entries[j].modified) {
			return entries[i].id < entries[j].id
		}
		return entries[i].modified.Before(entries[j].modified)
	})
	for _, entry := range entries {
		entry.accessOrder = c.nextAccessOrderLocked()
		c.entries[entry.id] = entry.cacheEntry
	}
	return c.makeRoomLocked(0)
}

// makeRoomLocked reclaims only committed files and abandoned partials. Active
// partials are protected by reservations and cannot be evicted underneath writers.
func (c *Cache) makeRoomLocked(needed int64) error {
	if needed <= c.maxBytes-c.usedBytes-c.reservedBytes {
		return nil
	}
	type victim struct {
		id string
		cacheEntry
	}
	entries := make([]victim, 0, len(c.entries))
	for id, entry := range c.entries {
		entries = append(entries, victim{id, entry})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].accessOrder < entries[j].accessOrder })
	var failures error
	for _, entry := range entries {
		if needed <= c.maxBytes-c.usedBytes-c.reservedBytes {
			return nil
		}
		if c.readers[entry.id] > 0 {
			continue
		}
		if err := os.Remove(entry.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			failures = errors.Join(failures, err)
			continue
		}
		c.usedBytes -= entry.size
		delete(c.entries, entry.id)
	}
	if needed <= c.maxBytes-c.usedBytes-c.reservedBytes {
		return nil
	}
	return errors.Join(errCacheCapacity, failures)
}

type cacheReservation struct{ bytes int64 }

func (c *Cache) reserveSpace(bytes int64) (*cacheReservation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.makeRoomLocked(bytes); err != nil {
		return nil, err
	}
	c.reservedBytes += bytes
	return &cacheReservation{bytes}, nil
}

func (c *Cache) growReservation(reservation *cacheReservation, required int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if required <= reservation.bytes {
		return nil
	}
	extra := required - reservation.bytes
	if err := c.makeRoomLocked(extra); err != nil {
		return err
	}
	c.reservedBytes += extra
	reservation.bytes = required
	return nil
}

func (c *Cache) releaseReservation(reservation *cacheReservation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reservedBytes -= reservation.bytes
	reservation.bytes = 0
}

func (c *Cache) discardPartial(part string, reservation *cacheReservation) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// If deletion fails, keep the physical bytes indexed for later eviction. Never
	// release quota for debris that is still on disk.
	if err := os.Remove(part); err != nil && !errors.Is(err, os.ErrNotExist) {
		size := reservation.bytes
		if info, err := os.Lstat(part); err == nil {
			size = info.Size()
		}
		c.entries[filepath.Base(part)] = cacheEntry{path: part, size: size, accessOrder: c.nextAccessOrderLocked()}
		c.usedBytes += size
	}
	c.reservedBytes -= reservation.bytes
	reservation.bytes = 0
}

func (c *Cache) commitPartial(ctx context.Context, part, path string, size int64, reservation *cacheReservation) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(part, path); err != nil {
		return err
	}
	id := filepath.Base(path)
	if old, exists := c.entries[id]; exists {
		c.usedBytes -= old.size
	}
	c.reservedBytes -= reservation.bytes
	reservation.bytes = 0
	c.usedBytes += size
	c.entries[id] = cacheEntry{path: path, size: size, accessOrder: c.nextAccessOrderLocked()}
	return nil
}

type cacheWriter struct {
	ctx         context.Context
	cache       *Cache
	file        io.Writer
	reservation *cacheReservation
	written     int64
}

func (w *cacheWriter) Write(bytes []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(bytes)) > w.cache.options.MaxFileBytes-w.written {
		return 0, errDownloadTooLarge
	}
	if err := w.cache.growReservation(w.reservation, w.written+int64(len(bytes))); err != nil {
		return 0, err
	}
	n, err := w.file.Write(bytes)
	w.written += int64(n)
	return n, err
}
