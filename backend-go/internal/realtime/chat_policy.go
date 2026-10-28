package realtime

import (
	"errors"
	"sync"
	"time"
)

var ErrChatRateLimited = errors.New("chat rate limit exceeded")

type ChatLimits struct {
	MinInterval                      time.Duration
	MaxHistorySize, MaxMessageLength int
}
type chatKey struct{ stream, user string }
type chatPolicy struct {
	limits   ChatLimits
	mu       sync.Mutex
	lastSent map[chatKey]time.Time
	now      func() time.Time
}

func newChatPolicy(limits ChatLimits) *chatPolicy {
	return &chatPolicy{limits: limits, lastSent: map[chatKey]time.Time{}, now: time.Now}
}

// SetChatLimits configures all streams, including public chat, before serving rooms.
func (m *Manager) SetChatLimits(limits ChatLimits) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chat = newChatPolicy(limits)
}

func (p *chatPolicy) reserve(key chatKey) (time.Time, error) {
	now := p.now()
	if p.limits.MinInterval == 0 {
		return now, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if last, ok := p.lastSent[key]; ok && now.Sub(last) < p.limits.MinInterval {
		return now, ErrChatRateLimited
	}
	if len(p.lastSent) >= 10000 {
		for key, last := range p.lastSent {
			if now.Sub(last) >= p.limits.MinInterval {
				delete(p.lastSent, key)
			}
		}
		if len(p.lastSent) >= 10000 {
			return now, ErrChatRateLimited
		}
	}
	p.lastSent[key] = now
	return now, nil
}
func (p *chatPolicy) release(key chatKey, reservation time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastSent[key] == reservation {
		delete(p.lastSent, key)
	}
}
