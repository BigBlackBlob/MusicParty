package realtime

import (
	"context"
	"errors"
	"time"
)

const ControlMutationTTL = 60 * time.Second
const controlMutationCapacity = 256

var (
	ErrMutationConflict = errors.New("control mutation fingerprint conflicts")
	ErrMutationScope    = errors.New("control idempotency scope is stale")
	ErrMutationCapacity = errors.New("control idempotency capacity reached")
	ErrMutationInvalid  = errors.New("control mutation requires scope, actor and mutation IDs")
	ErrMutationUnknown  = errors.New("control mutation outcome is unknown")
)

// ActorID must come from the authenticated session, never from the wire payload.
type ControlMutation struct{ ScopeID, ActorID, MutationID string }
type controlKey struct{ actor, mutation string }
type controlFingerprint struct {
	kind            string
	position, epoch int64
	hasEpoch        bool
}
type controlRecord struct {
	fingerprint controlFingerprint
	result      ControlResult
	err         error
	expires     time.Time
	unknown     bool
}

func controlResultValue(value any) ControlResult {
	result, _ := value.(ControlResult)
	return result
}

// Scope and room are implicit in this runtime-owned map. Unknown outcomes stay
// reserved until runtime destruction; they must never become executable again.
func (r *RoomRuntime) executeControl(ctx context.Context, kind string, positionMS int64, epoch *int64, mutations []ControlMutation, apply func(*RoomRuntime) (any, error)) (any, error) {
	return r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		if len(mutations) == 0 {
			return apply(runtime)
		}
		m := mutations[0]
		result := ControlResult{MutationID: m.MutationID}
		if m.MutationID == "" || m.ScopeID == "" || m.ActorID == "" {
			return result, ErrMutationInvalid
		}
		if m.ScopeID != runtime.controlScope {
			return result, ErrMutationScope
		}
		fingerprint := controlFingerprint{kind: kind, position: positionMS}
		if epoch != nil {
			fingerprint.hasEpoch = true
			fingerprint.epoch = *epoch
		}
		now := time.Now()
		for key, record := range runtime.controlMutations {
			if !record.unknown && !now.Before(record.expires) {
				delete(runtime.controlMutations, key)
			}
		}
		key := controlKey{actor: m.ActorID, mutation: m.MutationID}
		if record, exists := runtime.controlMutations[key]; exists {
			if record.fingerprint != fingerprint {
				return result, ErrMutationConflict
			}
			result = record.result
			result.Replayed = true
			if result.Committed != nil {
				watermark := *result.Committed
				result.Committed = &watermark
			}
			return result, record.err
		}
		if len(runtime.controlMutations) >= controlMutationCapacity {
			return result, ErrMutationCapacity
		}
		value, err := apply(runtime)
		result = controlResultValue(value)
		result.MutationID = m.MutationID
		record := controlRecord{fingerprint: fingerprint, result: result, err: err, expires: time.Now().Add(ControlMutationTTL)}
		if err != nil && !errors.Is(err, ErrControlLocked) && !errors.Is(err, ErrControlDenied) && !errors.Is(err, ErrSeekForbidden) && !errors.Is(err, ErrPreconditionFailed) && !errors.Is(err, ErrNoHistory) {
			record.unknown = true
			record.err = ErrMutationUnknown
		}
		if result.Committed != nil {
			watermark := *result.Committed
			record.result.Committed = &watermark
		}
		runtime.controlMutations[key] = record
		return result, err
	})
}
