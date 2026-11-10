package media

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrResolverBusy = errors.New("media resolver capacity reached")
var youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
var defaultYTDLP = newYTDLPResolver(2)

type ytdlpResult struct {
	done    chan struct{}
	url     string
	err     error
	expires time.Time
}
type ytdlpResolver struct {
	mu       sync.Mutex
	capacity int
	active   int
	entries  map[string]*ytdlpResult
	run      func(context.Context, string, string) (string, error)
}

func newYTDLPResolver(capacity int) *ytdlpResolver {
	return &ytdlpResolver{capacity: capacity, entries: map[string]*ytdlpResult{}, run: resolveYTDLP}
}

// Requests for one video share a bounded resolution independent of client disconnects.
func (r *ytdlpResolver) resolve(ctx context.Context, executable, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !youtubeID.MatchString(id) {
		return "", errors.New("invalid YouTube video ID")
	}
	key := executable + "\x00" + id
	r.mu.Lock()
	for k, entry := range r.entries {
		if !entry.expires.IsZero() && time.Now().After(entry.expires) {
			delete(r.entries, k)
		}
	}
	entry := r.entries[key]
	if entry == nil {
		if r.active >= r.capacity {
			r.mu.Unlock()
			return "", ErrResolverBusy
		}
		// Successful URLs are cached briefly; keep the map bounded as well.
		if len(r.entries) >= 256 {
			for k, cached := range r.entries {
				if !cached.expires.IsZero() {
					delete(r.entries, k)
				}
			}
		}
		entry = &ytdlpResult{done: make(chan struct{})}
		r.entries[key] = entry
		r.active++
		go func() {
			opCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			value, err := r.run(opCtx, executable, id)
			r.mu.Lock()
			defer r.mu.Unlock()
			entry.url, entry.err = value, err
			entry.expires = time.Now().Add(time.Minute)
			if err != nil {
				delete(r.entries, key)
			}
			r.active--
			close(entry.done)
		}()
	}
	r.mu.Unlock()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-entry.done:
		return entry.url, entry.err
	}
}

func ResolveYTDLP(ctx context.Context, executable, videoID string) (string, error) {
	return defaultYTDLP.resolve(ctx, executable, videoID)
}

func resolveYTDLP(ctx context.Context, executable, videoID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := runProcessTreeOutput(ctx, executable, "--no-playlist", "--no-warnings", "-f", "bestaudio", "-g", "https://www.youtube.com/watch?v="+videoID)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if value := strings.TrimSpace(line); strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
			return value, nil
		}
	}
	return "", errors.New("yt-dlp did not return a media URL")
}
