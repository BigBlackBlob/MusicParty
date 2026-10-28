package httpapi

import (
	"fmt"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
)

type loginAttempt struct {
	attempts            int
	first, blockedUntil time.Time
}
type loginLimiter struct {
	mu              sync.Mutex
	enabled         bool
	max, maxTracked int
	window, block   time.Duration
	items           map[string]loginAttempt
	now             func() time.Time
}

func newLoginLimiter(cfg config.Config) *loginLimiter {
	return &loginLimiter{enabled: cfg.Auth.RateLimitEnabled, max: cfg.Auth.MaxAttempts, maxTracked: cfg.Auth.MaxTrackedIPs, window: cfg.Auth.Window, block: cfg.Auth.BlockDuration, items: map[string]loginAttempt{}, now: time.Now}
}
func (l *loginLimiter) expires(item loginAttempt) time.Time {
	until := item.first.Add(l.window)
	if item.blockedUntil.After(until) {
		return item.blockedUntil
	}
	return until
}
func retryAfter(until, now time.Time) string {
	return fmt.Sprint(max(1, int(until.Sub(now).Seconds()+0.999)))
}

func (l *loginLimiter) blockedLocked(ip string, now time.Time) (string, bool) {
	if item, ok := l.items[ip]; ok {
		if !now.Before(l.expires(item)) {
			delete(l.items, ip)
		} else if now.Before(item.blockedUntil) {
			return retryAfter(item.blockedUntil, now), true
		} else {
			return "", false
		}
	}
	if len(l.items) >= l.maxTracked {
		var earliest time.Time
		for key, item := range l.items {
			until := l.expires(item)
			if !now.Before(until) {
				delete(l.items, key)
				continue
			}
			if earliest.IsZero() || until.Before(earliest) {
				earliest = until
			}
		}
		// Preserve active reservations. Untracked sources wait for capacity instead
		// of becoming unlimited or evicting another source's active block.
		if len(l.items) >= l.maxTracked {
			return retryAfter(earliest, now), true
		}
	}
	return "", false
}
func (l *loginLimiter) recordAttemptLocked(ip string, now time.Time) {
	item := l.items[ip]
	if item.first.IsZero() || !now.Before(l.expires(item)) {
		item = loginAttempt{first: now}
	}
	item.attempts++
	if item.attempts >= l.max {
		item.blockedUntil = now.Add(l.block)
	}
	l.items[ip] = item
}

// admit reserves the attempt before expensive work, so concurrent requests
// cannot all pass the same preflight check. Guest and login budgets are separate.
func (l *loginLimiter) admit(ip string) (string, bool) {
	if !l.enabled {
		return "", false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if retry, blocked := l.blockedLocked(ip, now); blocked {
		return retry, true
	}
	l.recordAttemptLocked(ip, now)
	return "", false
}
func (l *loginLimiter) success(ip string) { l.mu.Lock(); delete(l.items, ip); l.mu.Unlock() }
