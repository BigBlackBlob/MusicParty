package httpapi

import (
	"fmt"
	"net/netip"
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
	overflow        loginAttempt
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
		for key, item := range l.items {
			until := l.expires(item)
			if !now.Before(until) {
				delete(l.items, key)
				continue
			}
		}
		// Preserve tracked source budgets and use a shared budget at capacity.
		if len(l.items) >= l.maxTracked {
			// Bound untracked sources together without denying every new source.
			if l.overflow.first.IsZero() || !now.Before(l.expires(l.overflow)) {
				l.overflow = loginAttempt{first: now}
			}
			if now.Before(l.overflow.blockedUntil) {
				return retryAfter(l.overflow.blockedUntil, now), true
			}
			l.overflow.attempts++
			if l.overflow.attempts >= l.max {
				l.overflow.blockedUntil = now.Add(l.block)
			}
			return "", false
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
	ip = limiterKey(ip)
	now := l.now()
	if retry, blocked := l.blockedLocked(ip, now); blocked {
		return retry, true
	}
	if _, exists := l.items[ip]; exists || len(l.items) < l.maxTracked {
		l.recordAttemptLocked(ip, now)
	}
	return "", false
}
func (l *loginLimiter) success(ip string) {
	l.mu.Lock()
	delete(l.items, limiterKey(ip))
	l.mu.Unlock()
}

// IPv6 privacy addresses in the same subnet share an authentication budget.
func limiterKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr, 64).Masked().String()
}
