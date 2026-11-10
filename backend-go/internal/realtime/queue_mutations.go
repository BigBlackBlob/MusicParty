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
	expires     time.Time
	actor       string
}

// Queue retries replay their original result in this runtime for 60 seconds.
// A full cache rejects new IDs instead of evicting executable reservations.
// Wire IDs are namespaced by the authenticated actor at the HTTP/WS boundary.
func (r *RoomRuntime) executeQueue(ctx context.Context, id string, fingerprint any, apply func(*RoomRuntime, context.Context) (any, error)) (any, error) {
	encoded, err := json.Marshal(fingerprint)
	if err != nil {
		return nil, err
	}
	return r.Execute(ctx, func(runtime *RoomRuntime, ctx context.Context) (any, error) {
		// Empty IDs are reserved for trusted internal callers; wire handlers reject them.
		if id == "" {
			return apply(runtime, ctx)
		}
		if strings.TrimSpace(id) == "" || len(id) > 512 {
			return nil, ErrMutationInvalid
		}
		var identity [2]string
		_ = json.Unmarshal([]byte(id), &identity)
		actor := identity[0]
		// Entries have a fixed TTL, so insertion order is expiration order.
		runtime.purgeQueueMutations(time.Now())
		if record, ok := runtime.mutations[id]; ok {
			if record.fingerprint != sha256.Sum256(encoded) {
				return nil, ErrMutationConflict
			}
			return record.value, nil
		}
		if len(runtime.mutations) >= 2048 || actor != "" && runtime.mutationCounts[actor] >= mutationActorCapacity {
			return nil, ErrMutationCapacity
		}
		value, err := apply(runtime, ctx)
		if err == nil {
			runtime.mutations[id] = queueMutationRecord{fingerprint: sha256.Sum256(encoded), value: value, actor: actor, expires: time.Now().Add(ControlMutationTTL)}
			runtime.mutationOrder = append(runtime.mutationOrder, id)
			if actor != "" {
				runtime.mutationCounts[actor]++
			}
		}
		return value, err
	})
}

func (r *RoomRuntime) purgeQueueMutations(now time.Time) {
	n := 0
	for ; n < len(r.mutationOrder); n++ {
		key := r.mutationOrder[n]
		record := r.mutations[key]
		if now.Before(record.expires) {
			break
		}
		delete(r.mutations, key)
		if record.actor != "" {
			r.mutationCounts[record.actor]--
			if r.mutationCounts[record.actor] == 0 {
				delete(r.mutationCounts, record.actor)
			}
		}
		r.mutationOrder[n] = ""
	}
	r.mutationOrder = r.mutationOrder[n:]
}
