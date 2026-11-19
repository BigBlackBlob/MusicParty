package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	LikedSongsSystemKey = "liked-songs"
	LikedSongsName      = "喜欢的歌曲"
)

// CommitRoomLike commits the shared playback markers and the caller's personal
// liked songs together. A failure leaves both unchanged.
func (repository *RealtimeRepository) CommitRoomLike(ctx context.Context, state PlaybackState, publicID string, music Music, liked bool) error {
	encodedState, err := encodePlaybackState(state)
	if err != nil {
		return err
	}
	musicJSON, err := marshalJavaJSON(music)
	if err != nil {
		return err
	}
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var guest bool
		err := tx.QueryRowContext(ctx, `select is_guest from user_profile where public_id = ?`, publicID).Scan(&guest)
		if errors.Is(err, sql.ErrNoRows) || guest {
			return upsertPlaybackStateTx(ctx, tx, state, encodedState)
		}
		if err != nil {
			return err
		}
		now := repository.now().UnixMilli()
		if liked {
			if _, err := tx.ExecContext(ctx, `insert into user_playlist(id,owner_public_id,name,system_key,created_at,updated_at)
				values(?,?,?,?,?,?) on conflict(owner_public_id,system_key) do nothing`, uuid.NewString(), publicID, LikedSongsName, LikedSongsSystemKey, now, now); err != nil {
				return err
			}
		}
		var playlistID string
		err = tx.QueryRowContext(ctx, `select id from user_playlist where owner_public_id=? and system_key=?`, publicID, LikedSongsSystemKey).Scan(&playlistID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			musicKey := music.Platform + ":" + music.ID
			var result sql.Result
			if liked {
				result, err = tx.ExecContext(ctx, `insert into user_playlist_track(id,playlist_id,music_json,music_key,sort_order,created_at)
					select ?,?,?,?,coalesce(max(sort_order),-1)+1,? from user_playlist_track where playlist_id=?
					having not exists(select 1 from user_playlist_track where playlist_id=? and music_key=?)`, uuid.NewString(), playlistID, musicJSON, musicKey, now, playlistID, playlistID, musicKey)
			} else {
				result, err = tx.ExecContext(ctx, `delete from user_playlist_track where playlist_id=? and music_key=?`, playlistID, musicKey)
			}
			if err != nil {
				return err
			}
			changed, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if changed > 0 {
				if liked {
					if err := pruneLikedSongs(ctx, tx, playlistID); err != nil {
						return err
					}
				} else {
					if err := rewriteUserPlaylistOrder(ctx, tx, publicID, playlistID); err != nil {
						return err
					}
				}
				if _, err := tx.ExecContext(ctx, `update user_playlist set updated_at=? where id=?`, now, playlistID); err != nil {
					return err
				}
			}
		}
		return upsertPlaybackStateTx(ctx, tx, state, encodedState)
	})
}

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

// pruneLikedSongs keeps the 500 most recently added tracks inside the write transaction.
func pruneLikedSongs(ctx context.Context, tx *sql.Tx, playlistID string) error {
	_, err := tx.ExecContext(ctx, `delete from user_playlist_track where playlist_id = ? and id in (
		select id from user_playlist_track where playlist_id = ? order by sort_order desc, created_at desc, id desc limit -1 offset 500
	)`, playlistID, playlistID)
	return err
}
