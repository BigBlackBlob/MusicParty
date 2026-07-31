package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RoomPlaylistRepository struct {
	store *Store
	now   func() time.Time
	newID func() string
}

func NewRoomPlaylistRepository(store *Store, now func() time.Time, newID func() string) *RoomPlaylistRepository {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &RoomPlaylistRepository{store: store, now: now, newID: newID}
}

func (repository *RoomPlaylistRepository) ListPlaylists(ctx context.Context, roomID string) ([]Playlist, error) {
	rows, err := repository.store.reader.QueryContext(ctx, `
		select p.id, p.room_id, p.name, p.created_at, p.updated_at, count(t.id) as track_count
		from room_playlist p left join room_playlist_track t on t.playlist_id = p.id
		where p.room_id = ?
		group by p.id, p.room_id, p.name, p.created_at, p.updated_at
		order by p.created_at
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list room playlists: %w", err)
	}
	defer rows.Close()
	playlists := []Playlist{}
	for rows.Next() {
		playlist, err := scanRoomPlaylist(rows)
		if err != nil {
			return nil, fmt.Errorf("scan room playlist: %w", err)
		}
		playlists = append(playlists, playlist)
	}
	return playlists, rows.Err()
}

func (repository *RoomPlaylistRepository) FindPlaylist(ctx context.Context, roomID, playlistID string) (*Playlist, error) {
	playlist, err := scanRoomPlaylist(repository.store.reader.QueryRowContext(ctx, `
		select p.id, p.room_id, p.name, p.created_at, p.updated_at, count(t.id) as track_count
		from room_playlist p left join room_playlist_track t on t.playlist_id = p.id
		where p.room_id = ? and p.id = ?
		group by p.id, p.room_id, p.name, p.created_at, p.updated_at
	`, roomID, playlistID))
	if err != nil {
		return nil, fmt.Errorf("find room playlist: %w", err)
	}
	return &playlist, nil
}

func (repository *RoomPlaylistRepository) CreatePlaylist(ctx context.Context, roomID, name string) (Playlist, error) {
	now, id := repository.now().UnixMilli(), repository.newID()
	playlist := Playlist{ID: id, OwnerID: roomID, Name: name, CreatedAt: now, UpdatedAt: now}
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `insert into room_playlist(id, room_id, name, created_at, updated_at) values (?, ?, ?, ?, ?)`, id, roomID, name, now, now)
		return err
	})
	return playlist, err
}

func (repository *RoomPlaylistRepository) RenamePlaylist(ctx context.Context, roomID, playlistID, name string) (*Playlist, error) {
	now := repository.now().UnixMilli()
	var updated bool
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `update room_playlist set name = ?, updated_at = ? where room_id = ? and id = ?`, name, now, roomID, playlistID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		updated = count > 0
		return err
	})
	if err != nil || !updated {
		return nil, err
	}
	return repository.FindPlaylist(ctx, roomID, playlistID)
}

func (repository *RoomPlaylistRepository) DeletePlaylist(ctx context.Context, roomID, playlistID string) (bool, error) {
	var deleted bool
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			delete from room_playlist_track where playlist_id = ?
			and exists (select 1 from room_playlist where id = ? and room_id = ?)
		`, playlistID, playlistID, roomID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `delete from room_playlist where room_id = ? and id = ?`, roomID, playlistID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		deleted = count > 0
		return err
	})
	return deleted, err
}

func (repository *RoomPlaylistRepository) ListTracks(ctx context.Context, roomID, playlistID string, offset, limit int) ([]PlaylistTrack, error) {
	return listPlaylistTracks(ctx, repository.store.reader, `
		select t.id, t.playlist_id, t.music_json, t.sort_order, t.created_at
		from room_playlist_track t join room_playlist p on p.id = t.playlist_id
		where p.room_id = ? and t.playlist_id = ? order by t.sort_order limit ? offset ?
	`, roomID, playlistID, min(500, max(1, limit)), max(0, offset))
}

func (repository *RoomPlaylistRepository) AddTrack(ctx context.Context, roomID, playlistID string, music *Music) (*PlaylistTrack, error) {
	if music == nil {
		return nil, nil
	}
	musicJSON, err := marshalJavaJSON(music)
	if err != nil {
		return nil, fmt.Errorf("encode room playlist track: %w", err)
	}
	now, id := repository.now().UnixMilli(), repository.newID()
	var track *PlaylistTrack
	err = repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if exists, err := roomPlaylistExists(ctx, tx, roomID, playlistID); err != nil || !exists {
			return err
		}
		var sortOrder int
		if err := tx.QueryRowContext(ctx, `select coalesce(max(sort_order), -1) + 1 from room_playlist_track where playlist_id = ?`, playlistID).Scan(&sortOrder); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `insert into room_playlist_track(id, playlist_id, music_json, sort_order, created_at) values (?, ?, ?, ?, ?)`, id, playlistID, musicJSON, sortOrder, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `update room_playlist set updated_at = ? where id = ?`, now, playlistID); err != nil {
			return err
		}
		value := PlaylistTrack{ID: id, PlaylistID: playlistID, Music: *music, SortOrder: sortOrder, CreatedAt: now}
		track = &value
		return nil
	})
	return track, err
}

// AddTracks adds the complete batch in one writer transaction.
func (repository *RoomPlaylistRepository) AddTracks(ctx context.Context, roomID, playlistID string, musics []Music) ([]PlaylistTrack, error) {
	type preparedTrack struct {
		id, musicJSON string
		music         Music
	}
	prepared := make([]preparedTrack, 0, len(musics))
	for _, music := range musics {
		encoded, err := marshalJavaJSON(&music)
		if err != nil {
			return nil, fmt.Errorf("encode room playlist track: %w", err)
		}
		prepared = append(prepared, preparedTrack{id: repository.newID(), musicJSON: encoded, music: music})
	}
	now := repository.now().UnixMilli()
	added := make([]PlaylistTrack, 0, len(prepared))
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		exists, err := roomPlaylistExists(ctx, tx, roomID, playlistID)
		if err != nil || !exists {
			return err
		}
		var sortOrder int
		if err := tx.QueryRowContext(ctx, `select coalesce(max(sort_order), -1) + 1 from room_playlist_track where playlist_id = ?`, playlistID).Scan(&sortOrder); err != nil {
			return err
		}
		for _, item := range prepared {
			if _, err := tx.ExecContext(ctx, `insert into room_playlist_track(id, playlist_id, music_json, sort_order, created_at) values (?, ?, ?, ?, ?)`, item.id, playlistID, item.musicJSON, sortOrder, now); err != nil {
				return err
			}
			added = append(added, PlaylistTrack{ID: item.id, PlaylistID: playlistID, Music: item.music, SortOrder: sortOrder, CreatedAt: now})
			sortOrder++
		}
		if len(added) > 0 {
			_, err = tx.ExecContext(ctx, `update room_playlist set updated_at = ? where id = ?`, now, playlistID)
		}
		return err
	})
	return added, err
}

func (repository *RoomPlaylistRepository) DeleteTrack(ctx context.Context, roomID, playlistID, trackID string) (bool, error) {
	var deleted bool
	now := repository.now().UnixMilli()
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if exists, err := roomPlaylistExists(ctx, tx, roomID, playlistID); err != nil || !exists {
			return err
		}
		result, err := tx.ExecContext(ctx, `delete from room_playlist_track where playlist_id = ? and id = ?`, playlistID, trackID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count == 0 {
			return err
		}
		deleted = true
		if err := rewriteRoomPlaylistOrder(ctx, tx, roomID, playlistID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `update room_playlist set updated_at = ? where id = ?`, now, playlistID)
		return err
	})
	return deleted, err
}

func (repository *RoomPlaylistRepository) ReorderTracks(ctx context.Context, roomID, playlistID string, orderedTrackIDs []string) error {
	if orderedTrackIDs == nil {
		return nil
	}
	now := repository.now().UnixMilli()
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if exists, err := roomPlaylistExists(ctx, tx, roomID, playlistID); err != nil || !exists {
			return err
		}
		for index, trackID := range orderedTrackIDs {
			if _, err := tx.ExecContext(ctx, `update room_playlist_track set sort_order = ? where playlist_id = ? and id = ?`, index, playlistID, trackID); err != nil {
				return err
			}
		}
		if err := rewriteRoomPlaylistOrder(ctx, tx, roomID, playlistID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `update room_playlist set updated_at = ? where id = ?`, now, playlistID)
		return err
	})
}

func (repository *RoomPlaylistRepository) DeleteRoomData(ctx context.Context, roomID string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `delete from room_playlist_track where playlist_id in (select id from room_playlist where room_id = ?)`, roomID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `delete from room_playlist where room_id = ?`, roomID)
		return err
	})
}

func roomPlaylistExists(ctx context.Context, tx *sql.Tx, roomID, playlistID string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `select count(1) from room_playlist where room_id = ? and id = ?`, roomID, playlistID).Scan(&count)
	return count > 0, err
}

func rewriteRoomPlaylistOrder(ctx context.Context, tx *sql.Tx, roomID, playlistID string) error {
	rows, err := tx.QueryContext(ctx, `
		select t.id from room_playlist_track t join room_playlist p on p.id = t.playlist_id
		where p.room_id = ? and t.playlist_id = ? order by t.sort_order, t.created_at
	`, roomID, playlistID)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for index, id := range ids {
		if _, err := tx.ExecContext(ctx, `update room_playlist_track set sort_order = ? where id = ?`, index, id); err != nil {
			return err
		}
	}
	return nil
}

func scanRoomPlaylist(row rowScanner) (Playlist, error) {
	var playlist Playlist
	if err := row.Scan(&playlist.ID, &playlist.OwnerID, &playlist.Name, &playlist.CreatedAt, &playlist.UpdatedAt, &playlist.TrackCount); err != nil {
		return Playlist{}, err
	}
	return playlist, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func listPlaylistTracks(ctx context.Context, database queryer, query string, arguments ...any) ([]PlaylistTrack, error) {
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list playlist tracks: %w", err)
	}
	defer rows.Close()
	tracks := []PlaylistTrack{}
	for rows.Next() {
		var track PlaylistTrack
		var musicJSON string
		if err := rows.Scan(&track.ID, &track.PlaylistID, &musicJSON, &track.SortOrder, &track.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan playlist track: %w", err)
		}
		if err := unmarshalJSON(musicJSON, &track.Music); err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}
