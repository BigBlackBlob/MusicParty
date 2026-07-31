package sqlite

import (
	"context"
	"database/sql"
	"time"
)

// RealtimeRepository atomically persists the queue, playback state, and an
// optional history entry owned by a room runtime.
type RealtimeRepository struct {
	store *Store
	now   func() time.Time
}

func NewRealtimeRepository(store *Store, now func() time.Time) *RealtimeRepository {
	if now == nil {
		now = time.Now
	}
	return &RealtimeRepository{store: store, now: now}
}

func (repository *RealtimeRepository) CommitRoomState(ctx context.Context, roomID string, queue []QueueItem, state PlaybackState, history *HistoryEntry) error {
	encodedQueue, err := encodeQueueItems(queue)
	if err != nil {
		return err
	}
	encodedState, err := encodePlaybackState(state)
	if err != nil {
		return err
	}
	var historyJSON string
	if history != nil {
		historyJSON, err = marshalJavaJSON(history.Music)
		if err != nil {
			return err
		}
	}
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if err := synchronizeQueueTx(ctx, tx, repository.now, roomID, queue, encodedQueue); err != nil {
			return err
		}
		if history != nil {
			if err := appendHistoryTx(ctx, tx, *history, historyJSON); err != nil {
				return err
			}
		}
		return upsertPlaybackStateTx(ctx, tx, state, encodedState)
	})
}
