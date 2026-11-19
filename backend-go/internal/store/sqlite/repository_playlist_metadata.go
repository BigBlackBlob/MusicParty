package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"slices"
)

func (repository *UserPlaylistRepository) SetCover(ctx context.Context, owner, playlistID, url string, data []byte) (bool, error) {
	return updatePlaylistMetadata(ctx, repository.store, `update user_playlist set cover_url = ?, cover_data = ?, updated_at = ? where owner_public_id = ? and id = ? and system_key is null`, url, data, repository.now().UnixMilli(), owner, playlistID)
}

func (repository *RoomPlaylistRepository) SetCover(ctx context.Context, room, playlistID, url string, data []byte) (bool, error) {
	return updatePlaylistMetadata(ctx, repository.store, `update room_playlist set cover_url = ?, cover_data = ?, updated_at = ? where room_id = ? and id = ?`, url, data, repository.now().UnixMilli(), room, playlistID)
}

func (repository *UserPlaylistRepository) Cover(ctx context.Context, owner, playlistID string) ([]byte, error) {
	return readPlaylistCover(repository.store.reader.QueryRowContext(ctx, `select cover_data from user_playlist where owner_public_id = ? and id = ?`, owner, playlistID))
}

func (repository *RoomPlaylistRepository) Cover(ctx context.Context, room, playlistID string) ([]byte, error) {
	return readPlaylistCover(repository.store.reader.QueryRowContext(ctx, `select cover_data from room_playlist where room_id = ? and id = ?`, room, playlistID))
}

func readPlaylistCover(row *sql.Row) ([]byte, error) {
	var data []byte
	err := row.Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return data, err
}

func (repository *UserPlaylistRepository) SetTrackNotes(ctx context.Context, owner, playlistID, trackID, notes string) (bool, error) {
	return updatePlaylistMetadata(ctx, repository.store, `update user_playlist_track set notes = ? where playlist_id = ? and id = ?
		and exists (select 1 from user_playlist where id = ? and owner_public_id = ? and system_key is null)`, notes, playlistID, trackID, playlistID, owner)
}

func (repository *RoomPlaylistRepository) SetTrackNotes(ctx context.Context, room, playlistID, trackID, notes string) (bool, error) {
	return updatePlaylistMetadata(ctx, repository.store, `update room_playlist_track set notes = ? where playlist_id = ? and id = ?
		and exists (select 1 from room_playlist where id = ? and room_id = ?)`, notes, playlistID, trackID, playlistID, room)
}

func updatePlaylistMetadata(ctx context.Context, store *Store, statement string, args ...any) (bool, error) {
	var changed bool
	err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, statement, args...)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		changed = count > 0
		return err
	})
	return changed, err
}

// MovePlaylist places a user playlist at a zero-based position among non-system playlists.
func (repository *UserPlaylistRepository) MovePlaylist(ctx context.Context, owner, playlistID string, position int) (bool, error) {
	var found bool
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `select id from user_playlist where owner_public_id = ? and system_key is null order by sort_order, created_at, id`, owner)
		if err != nil {
			return err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		current := slices.Index(ids, playlistID)
		if current < 0 {
			return nil
		}
		found = true
		position = min(max(0, position), len(ids)-1)
		if current == position {
			return nil
		}
		ids = slices.Insert(slices.Delete(ids, current, current+1), position, playlistID)
		for i, id := range ids {
			if _, err := tx.ExecContext(ctx, `update user_playlist set sort_order = ? where id = ?`, i, id); err != nil {
				return err
			}
		}
		return nil
	})
	return found, err
}
