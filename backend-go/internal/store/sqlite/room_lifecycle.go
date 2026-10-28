package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

var ErrRoomGone = errors.New("room is deleted or missing")

// RequireActiveRoom orders a room write against soft deletion in the same transaction.
// A foreign key alone cannot protect room data because the soft-deleted room row remains.
func RequireActiveRoom(ctx context.Context, tx *sql.Tx, roomID string) error {
	var id string
	err := tx.QueryRowContext(ctx, "select id from room where id=? and deleted_at is null", roomID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRoomGone
	}
	return err
}
