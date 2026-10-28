package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrDownloadQueueFull = errors.New("download queue is full")

type cacheEntry struct {
	path        string
	size        int64
	accessOrder uint64
}

// CacheOptions apply only to background downloads, never the playback proxy.
type CacheOptions struct {
	PlatformLimits                 map[string]int
	ReservationBytes, MaxFileBytes int64
	TaskTTL, RetryMaxDelay         time.Duration
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
	if err != nil || number <= 0 || number > (1<<63-1)/multiplier {
		return 0, errors.New("invalid cache size")
	}
	return number * multiplier, nil
}

type downloadJob struct {
	key, url string
	deadline time.Time
}

// Cache accounts for committed files plus reservations covering all partial writes.
// Entries use disk filenames, allowing startup eviction without reversing key hashes.
type Cache struct {
	ctx                      context.Context
	cancel                   context.CancelFunc
	root                     string
	maxBytes                 int64
	client                   *http.Client
	retries                  int
	retryBase                time.Duration
	options                  CacheOptions
	jobs                     chan downloadJob
	mu                       sync.Mutex
	entries                  map[string]cacheEntry
	usedBytes, reservedBytes int64
	inflight                 map[string]struct{}
	readers                  map[string]int
	limits                   map[string]chan struct{}
	accessSeq                uint64
	wait                     sync.WaitGroup
	closeOne                 sync.Once
	closed                   bool
}

func NewCache(root string, maxBytes int64, client *http.Client, workers, capacity, retries int, retryBase time.Duration, options ...CacheOptions) (*Cache, error) {
	var opts CacheOptions
	if len(options) > 0 {
		opts = options[0]
	}
	if opts.ReservationBytes == 0 {
		opts.ReservationBytes = 32 << 20
	}
	if opts.MaxFileBytes == 0 {
		opts.MaxFileBytes = 256 << 20
	}
	if opts.TaskTTL == 0 {
		opts.TaskTTL = 30 * time.Minute
	}
	if opts.RetryMaxDelay == 0 {
		opts.RetryMaxDelay = 30 * time.Second
	}
	if maxBytes <= 0 || retries < 0 || retryBase < 0 || opts.ReservationBytes < 0 || opts.MaxFileBytes < 0 || opts.TaskTTL < 0 || opts.RetryMaxDelay < 0 {
		return nil, errors.New("invalid cache download limits")
	}
	opts.MaxFileBytes = min(opts.MaxFileBytes, maxBytes)
	opts.ReservationBytes = min(opts.ReservationBytes, opts.MaxFileBytes)
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cache{ctx: ctx, cancel: cancel, root: root, maxBytes: maxBytes, client: client, retries: retries, retryBase: retryBase, options: opts,
		jobs: make(chan downloadJob, max(1, capacity)), entries: map[string]cacheEntry{}, inflight: map[string]struct{}{}, readers: map[string]int{}, limits: map[string]chan struct{}{}}
	if err := c.scan(); err != nil {
		cancel()
		return nil, err
	}
	for platform, limit := range opts.PlatformLimits {
		c.limits[platform] = make(chan struct{}, max(1, limit))
	}
	for range max(1, workers) {
		c.wait.Add(1)
		go c.worker()
	}
	return c, nil
}

// Get locates a cached file. Use Acquire to protect it from eviction while serving.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lookupLocked(filepath.Base(c.pathForKey(key)))
}

// Acquire pins a completed file until release. Otherwise eviction could unlink a
// playing file and claim space that its open handle still occupies on disk.
func (c *Cache) Acquire(key string) (string, func(), bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := filepath.Base(c.pathForKey(key))
	path, ok := c.lookupLocked(id)
	if !ok {
		return "", nil, false
	}
	c.readers[id]++
	release := sync.OnceFunc(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.readers[id]--
		if c.readers[id] == 0 {
			delete(c.readers, id)
		}
	})
	return path, release, true
}

func (c *Cache) lookupLocked(id string) (string, bool) {
	path := filepath.Join(c.root, id)
	info, err := os.Lstat(path)
	old, exists := c.entries[id]
	if err != nil {
		if exists && errors.Is(err, os.ErrNotExist) {
			c.usedBytes -= old.size
			delete(c.entries, id)
		}
		return "", false
	}
	if !info.Mode().IsRegular() {
		return "", false
	}
	base := c.usedBytes
	if exists {
		base -= old.size
	}
	if info.Size() > (1<<63-1)-base {
		return "", false
	}
	c.usedBytes = base + info.Size()
	c.entries[id] = cacheEntry{path: path, size: info.Size(), accessOrder: c.nextAccessOrderLocked()}
	if err := c.makeRoomLocked(0); err != nil {
		return "", false
	}
	_, exists = c.entries[id]
	return path, exists
}

func (c *Cache) Submit(key, url string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return context.Canceled
	}
	if _, ok := c.lookupLocked(filepath.Base(c.pathForKey(key))); ok {
		return nil
	}
	if _, ok := c.inflight[key]; ok {
		return nil
	}
	c.inflight[key] = struct{}{}
	select {
	case <-c.ctx.Done():
		delete(c.inflight, key)
		return context.Canceled
	case c.jobs <- downloadJob{key: key, url: url, deadline: time.Now().Add(c.options.TaskTTL)}:
		return nil
	default:
		delete(c.inflight, key)
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
		c.mu.Lock()
		defer c.mu.Unlock()
		clear(c.inflight)
		for {
			select {
			case <-c.jobs:
			default:
				return
			}
		}
	})
}

func (c *Cache) worker() {
	defer c.wait.Done()
	for {
		if c.ctx.Err() != nil {
			return
		}
		select {
		case <-c.ctx.Done():
			return
		case job := <-c.jobs:
			c.download(job)
			c.mu.Lock()
			delete(c.inflight, job.key)
			c.mu.Unlock()
		}
	}
}

func (c *Cache) pathForKey(key string) string {
	nameHash := sha256.Sum256([]byte(key))
	return filepath.Join(c.root, hex.EncodeToString(nameHash[:])+".media")
}

func (c *Cache) nextAccessOrderLocked() uint64 { c.accessSeq++; return c.accessSeq }

func retryAfter(value string, now time.Time) (time.Duration, bool) {
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds >= 0 && seconds <= (1<<63-1)/int64(time.Second) {
		return time.Duration(seconds) * time.Second, true
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now), true
	}
	return 0, false
}
