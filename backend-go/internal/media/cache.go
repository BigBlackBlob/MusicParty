package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrDownloadQueueFull = errors.New("download queue is full")

type cacheEntry struct {
	path       string
	size       int64
	lastAccess time.Time
}

func ParseByteSize(value string) (int64, error) {
	raw := strings.ToUpper(strings.TrimSpace(value))
	multiplier := int64(1)
	for suffix, factor := range map[string]int64{"KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40} {
		if strings.HasSuffix(raw, suffix) {
			multiplier = factor
			raw = strings.TrimSpace(strings.TrimSuffix(raw, suffix))
			break
		}
	}
	number, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || number <= 0 {
		return 0, errors.New("invalid cache size")
	}
	return number * multiplier, nil
}

type downloadJob struct{ key, url string }

// Cache owns a bounded download queue, atomic .part files, and LRU space reclamation.
type Cache struct {
	ctx       context.Context
	cancel    context.CancelFunc
	root      string
	maxBytes  int64
	client    *http.Client
	retries   int
	retryBase time.Duration
	jobs      chan downloadJob
	mu        sync.Mutex
	entries   map[string]cacheEntry
	inflight  map[string]struct{}
	limits    map[string]chan struct{}
	wait      sync.WaitGroup
	closeOne  sync.Once
	closed    bool
}

func NewCache(root string, maxBytes int64, client *http.Client, workers, capacity, retries int, retryBase time.Duration, platformLimits ...map[string]int) (*Cache, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	if partials, globErr := filepath.Glob(filepath.Join(root, "*.part")); globErr == nil {
		for _, partial := range partials {
			_ = os.Remove(partial)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cache{ctx: ctx, cancel: cancel, root: root, maxBytes: maxBytes, client: client, retries: retries, retryBase: retryBase, jobs: make(chan downloadJob, max(1, capacity)), entries: map[string]cacheEntry{}, inflight: map[string]struct{}{}, limits: map[string]chan struct{}{}}
	if len(platformLimits) > 0 {
		for platform, limit := range platformLimits[0] {
			c.limits[platform] = make(chan struct{}, max(1, limit))
		}
	}
	for range max(1, workers) {
		c.wait.Add(1)
		go c.worker()
	}
	return c, nil
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		path := c.pathForKey(key)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", false
		}
		entry = cacheEntry{path: path, size: info.Size(), lastAccess: time.Now()}
		c.entries[key] = entry
	}
	if _, err := os.Stat(entry.path); err != nil {
		delete(c.entries, key)
		return "", false
	}
	entry.lastAccess = time.Now()
	c.entries[key] = entry
	return entry.path, true
}

func (c *Cache) Submit(key, url string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return context.Canceled
	}
	if _, ok := c.entries[key]; ok {
		c.mu.Unlock()
		return nil
	}
	if _, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		return nil
	}
	c.inflight[key] = struct{}{}
	select {
	case <-c.ctx.Done():
		delete(c.inflight, key)
		c.mu.Unlock()
		return context.Canceled
	case c.jobs <- downloadJob{key: key, url: url}:
		c.mu.Unlock()
		return nil
	default:
		delete(c.inflight, key)
		c.mu.Unlock()
		return ErrDownloadQueueFull
	}
}

func (c *Cache) Close() {
	c.closeOne.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		c.cancel()
		c.wait.Wait()
	})
}

func (c *Cache) worker() {
	defer c.wait.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		case job := <-c.jobs:
			c.download(job)
			c.finish(job.key)
		}
	}
}

func (c *Cache) download(job downloadJob) {
	platform := strings.SplitN(job.key, ":", 2)[0]
	if limit := c.limits[platform]; limit != nil {
		select {
		case limit <- struct{}{}:
			defer func() { <-limit }()
		case <-c.ctx.Done():
			return
		}
	}
	path := c.pathForKey(job.key)
	part := path + ".part"
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			delay := c.retryBase << (attempt - 1)
			select {
			case <-time.After(delay):
			case <-c.ctx.Done():
				return
			}
		}
		request, err := http.NewRequestWithContext(c.ctx, http.MethodGet, job.url, nil)
		if err != nil {
			return
		}
		response, err := c.client.Do(request)
		if err != nil {
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			if response.StatusCode == http.StatusTooManyRequests {
				if delay, ok := retryAfter(response.Header.Get("Retry-After"), time.Now()); ok {
					response.Body.Close()
					select {
					case <-time.After(delay):
					case <-c.ctx.Done():
						return
					}
					continue
				}
			}
			response.Body.Close()
			if response.StatusCode < 500 && response.StatusCode != http.StatusTooManyRequests {
				return
			}
			continue
		}
		file, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
		if err != nil {
			response.Body.Close()
			return
		}
		size, copyErr := io.Copy(file, response.Body)
		closeErr := errors.Join(file.Close(), response.Body.Close())
		if copyErr != nil || closeErr != nil || c.ctx.Err() != nil {
			_ = os.Remove(part)
			continue
		}
		if err := os.Rename(part, path); err != nil {
			_ = os.Remove(part)
			return
		}
		c.mu.Lock()
		c.entries[job.key] = cacheEntry{path: path, size: size, lastAccess: time.Now()}
		c.evictLocked()
		c.mu.Unlock()
		return
	}
	_ = os.Remove(part)
}

func retryAfter(value string, now time.Time) (time.Duration, bool) {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now), true
	}
	return 0, false
}

func (c *Cache) pathForKey(key string) string {
	nameHash := sha256.Sum256([]byte(key))
	return filepath.Join(c.root, hex.EncodeToString(nameHash[:])+".media")
}

func (c *Cache) evictLocked() {
	var total int64
	entries := make([]struct {
		key string
		cacheEntry
	}, 0, len(c.entries))
	for key, entry := range c.entries {
		total += entry.size
		entries = append(entries, struct {
			key string
			cacheEntry
		}{key, entry})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].lastAccess.Before(entries[j].lastAccess) })
	for _, entry := range entries {
		if total <= c.maxBytes {
			break
		}
		if os.Remove(entry.path) == nil {
			total -= entry.size
			delete(c.entries, entry.key)
		}
	}
}

func (c *Cache) finish(key string) {
	c.mu.Lock()
	delete(c.inflight, key)
	c.mu.Unlock()
}
