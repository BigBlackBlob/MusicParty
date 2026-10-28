package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

var ErrQueueCapacity = errors.New("queue capacity exceeded")

// Limits count pending items, including priority items. HistorySize bounds only
// the in-memory previous-track window; persisted history is retained.
type QueueLimits struct{ MaxSize, MaxUserSongs, HistorySize int }

func (r *RoomRuntime) checkQueueCapacity(actor string, queue []storesqlite.QueueItem) error {
	if len(queue) > r.limits.MaxSize {
		return ErrQueueCapacity
	}
	userCount := 0
	for _, item := range queue {
		if item.EnqueuedBy.PublicID == actor {
			userCount++
		}
	}
	if userCount > r.limits.MaxUserSongs {
		return ErrQueueCapacity
	}
	return nil
}

type queueMutationRecord struct {
	fingerprint [32]byte
	value       any
	err         error
	expires     time.Time
}

// Queue retries replay their original result in this runtime for 60 seconds.
// A full cache rejects new IDs instead of evicting executable reservations.
// Wire IDs are namespaced by the authenticated actor at the HTTP/WS boundary.
func (r *RoomRuntime) executeQueue(ctx context.Context, id string, fingerprint any, apply func(*RoomRuntime) (any, error)) (any, error) {
	encoded, err := json.Marshal(fingerprint)
	if err != nil {
		return nil, err
	}
	return r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		// Empty IDs are reserved for trusted internal callers; wire handlers reject them.
		if id == "" {
			return apply(runtime)
		}
		if strings.TrimSpace(id) == "" || len(id) > 512 {
			return nil, ErrMutationInvalid
		}
		now := time.Now()
		for key, record := range runtime.mutations {
			if !record.expires.IsZero() && !now.Before(record.expires) {
				delete(runtime.mutations, key)
			}
		}
		if record, ok := runtime.mutations[id]; ok {
			if record.fingerprint != sha256.Sum256(encoded) {
				return nil, ErrMutationConflict
			}
			return record.value, record.err
		}
		if len(runtime.mutations) >= 2048 {
			return nil, ErrMutationCapacity
		}
		value, err := apply(runtime)
		if err == nil {
			runtime.mutations[id] = queueMutationRecord{fingerprint: sha256.Sum256(encoded), value: value, expires: time.Now().Add(ControlMutationTTL)}
		} else if !errors.Is(err, ErrStaleQueue) && !errors.Is(err, ErrQueueCapacity) {
			// A canceled store write can have committed. Keep the ID reserved for
			// the runtime's lifetime and require resync instead of executing again.
			runtime.mutations[id] = queueMutationRecord{fingerprint: sha256.Sum256(encoded), err: ErrMutationUnknown}
		}
		return value, err
	})
}
