package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type UserPlaylistRepository struct {
	store *Store
	now   func() time.Time
	newID func() string
}

func NewUserPlaylistRepository(store *Store, now func() time.Time, newID func() string) *UserPlaylistRepository {
	if now == nil {
		now = time.Now
	}
	if newID == nil {
		newID = uuid.NewString
	}
	return &UserPlaylistRepository{store: store, now: now, newID: newID}
}

func (repository *UserPlaylistRepository) ListPlaylists(ctx context.Context, ownerPublicID string) ([]Playlist, error) {
	rows, err := repository.store.reader.QueryContext(ctx, userPlaylistSelect+`
		where p.owner_public_id = ?
		group by p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at
		order by case when p.system_key is null then 1 else 0 end, p.created_at
	`, ownerPublicID)
	if err != nil {
		return nil, fmt.Errorf("list user playlists: %w", err)
	}
	defer rows.Close()
	playlists := []Playlist{}
	for rows.Next() {
		playlist, err := scanUserPlaylist(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user playlist: %w", err)
		}
		playlists = append(playlists, playlist)
	}
	return playlists, rows.Err()
}

func (repository *UserPlaylistRepository) FindPlaylist(ctx context.Context, ownerPublicID, playlistID string) (*Playlist, error) {
	return repository.find(ctx, `where p.owner_public_id = ? and p.id = ?`, ownerPublicID, playlistID)
}

func (repository *UserPlaylistRepository) FindSystemPlaylist(ctx context.Context, ownerPublicID, systemKey string) (*Playlist, error) {
	return repository.find(ctx, `where p.owner_public_id = ? and p.system_key = ?`, ownerPublicID, systemKey)
}

func (repository *UserPlaylistRepository) CreatePlaylist(ctx context.Context, ownerPublicID, name string) (Playlist, error) {
	return repository.create(ctx, ownerPublicID, name, nil)
}

func (repository *UserPlaylistRepository) CreateSystemPlaylist(ctx context.Context, ownerPublicID, name, systemKey string) (Playlist, error) {
	created, err := repository.create(ctx, ownerPublicID, name, &systemKey)
	if err != nil {
		return Playlist{}, err
	}
	playlist, err := repository.FindSystemPlaylist(ctx, ownerPublicID, systemKey)
	if err != nil {
		return created, err
	}
	return *playlist, nil
}

func (repository *UserPlaylistRepository) RenamePlaylist(ctx context.Context, ownerPublicID, playlistID, name string) (*Playlist, error) {
	now := repository.now().UnixMilli()
	var updated bool
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `update user_playlist set name = ?, updated_at = ? where owner_public_id = ? and id = ?`, name, now, ownerPublicID, playlistID)
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
	return repository.FindPlaylist(ctx, ownerPublicID, playlistID)
}

func (repository *UserPlaylistRepository) DeletePlaylist(ctx context.Context, ownerPublicID, playlistID string) (bool, error) {
	var deleted bool
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			delete from user_playlist_track where playlist_id = ?
			and exists (select 1 from user_playlist where id = ? and owner_public_id = ?)
		`, playlistID, playlistID, ownerPublicID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `delete from user_playlist where owner_public_id = ? and id = ?`, ownerPublicID, playlistID)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		deleted = count > 0
		return err
	})
	return deleted, err
}

func (repository *UserPlaylistRepository) ListTracks(ctx context.Context, ownerPublicID, playlistID string, offset, limit int) ([]PlaylistTrack, error) {
	return listPlaylistTracks(ctx, repository.store.reader, `
		select t.id, t.playlist_id, t.music_json, t.sort_order, t.created_at
		from user_playlist_track t join user_playlist p on p.id = t.playlist_id
		where p.owner_public_id = ? and t.playlist_id = ? order by t.sort_order limit ? offset ?
	`, ownerPublicID, playlistID, min(500, max(1, limit)), max(0, offset))
}

func (repository *UserPlaylistRepository) AddTrackIfAbsent(ctx context.Context, ownerPublicID, playlistID string, music *Music) (*PlaylistTrack, error) {
	if music == nil {
		return nil, nil
	}
	musicJSON, err := marshalJavaJSON(music)
	if err != nil {
		return nil, fmt.Errorf("encode user playlist track: %w", err)
	}
	musicKey := music.Platform + ":" + music.ID
	now, id := repository.now().UnixMilli(), repository.newID()
	var track *PlaylistTrack
	err = repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if exists, err := userPlaylistExists(ctx, tx, ownerPublicID, playlistID); err != nil || !exists {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, `select count(1) from user_playlist_track where playlist_id = ? and music_key = ?`, playlistID, musicKey).Scan(&count); err != nil || count > 0 {
			return err
		}
		var sortOrder int
		if err := tx.QueryRowContext(ctx, `select coalesce(max(sort_order), -1) + 1 from user_playlist_track where playlist_id = ?`, playlistID).Scan(&sortOrder); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			insert into user_playlist_track(id, playlist_id, music_json, music_key, sort_order, created_at)
			values (?, ?, ?, ?, ?, ?)
		`, id, playlistID, musicJSON, musicKey, sortOrder, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `update user_playlist set updated_at = ? where id = ?`, now, playlistID); err != nil {
			return err
		}
		value := PlaylistTrack{ID: id, PlaylistID: playlistID, Music: *music, SortOrder: sortOrder, CreatedAt: now}
		track = &value
		return nil
	})
	return track, err
}

// AddTracksIfAbsent adds the complete batch in one writer transaction and
// skips music keys that already exist in the playlist.
func (repository *UserPlaylistRepository) AddTracksIfAbsent(ctx context.Context, ownerPublicID, playlistID string, musics []Music) ([]PlaylistTrack, error) {
	type preparedTrack struct {
		id, musicJSON, musicKey string
		music                   Music
	}
	prepared := make([]preparedTrack, 0, len(musics))
	for _, music := range musics {
		encoded, err := marshalJavaJSON(&music)
		if err != nil {
			return nil, fmt.Errorf("encode user playlist track: %w", err)
		}
		prepared = append(prepared, preparedTrack{id: repository.newID(), musicJSON: encoded, musicKey: music.Platform + ":" + music.ID, music: music})
	}
	now := repository.now().UnixMilli()
	added := make([]PlaylistTrack, 0, len(prepared))
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		exists, err := userPlaylistExists(ctx, tx, ownerPublicID, playlistID)
		if err != nil || !exists {
			return err
		}
		var sortOrder int
		if err := tx.QueryRowContext(ctx, `select coalesce(max(sort_order), -1) + 1 from user_playlist_track where playlist_id = ?`, playlistID).Scan(&sortOrder); err != nil {
			return err
		}
		for _, item := range prepared {
			var count int
			if err := tx.QueryRowContext(ctx, `select count(1) from user_playlist_track where playlist_id = ? and music_key = ?`, playlistID, item.musicKey).Scan(&count); err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			if _, err := tx.ExecContext(ctx, `insert into user_playlist_track(id, playlist_id, music_json, music_key, sort_order, created_at) values (?, ?, ?, ?, ?, ?)`, item.id, playlistID, item.musicJSON, item.musicKey, sortOrder, now); err != nil {
				return err
			}
			added = append(added, PlaylistTrack{ID: item.id, PlaylistID: playlistID, Music: item.music, SortOrder: sortOrder, CreatedAt: now})
			sortOrder++
		}
		if len(added) > 0 {
			_, err = tx.ExecContext(ctx, `update user_playlist set updated_at = ? where id = ?`, now, playlistID)
		}
		return err
	})
	return added, err
}

func (repository *UserPlaylistRepository) DeleteTrack(ctx context.Context, ownerPublicID, playlistID, trackID string) (bool, error) {
	return repository.deleteTrack(ctx, ownerPublicID, playlistID, "id", trackID)
}

func (repository *UserPlaylistRepository) DeleteTrackByMusicKey(ctx context.Context, ownerPublicID, playlistID, musicKey string) (bool, error) {
	return repository.deleteTrack(ctx, ownerPublicID, playlistID, "music_key", musicKey)
}

func (repository *UserPlaylistRepository) ReorderTracks(ctx context.Context, ownerPublicID, playlistID string, orderedTrackIDs []string) error {
	if orderedTrackIDs == nil {
		return nil
	}
	now := repository.now().UnixMilli()
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if exists, err := userPlaylistExists(ctx, tx, ownerPublicID, playlistID); err != nil || !exists {
			return err
		}
		for index, trackID := range orderedTrackIDs {
			if _, err := tx.ExecContext(ctx, `update user_playlist_track set sort_order = ? where playlist_id = ? and id = ?`, index, playlistID, trackID); err != nil {
				return err
			}
		}
		if err := rewriteUserPlaylistOrder(ctx, tx, ownerPublicID, playlistID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `update user_playlist set updated_at = ? where id = ?`, now, playlistID)
		return err
	})
}

func (repository *UserPlaylistRepository) create(ctx context.Context, ownerPublicID, name string, systemKey *string) (Playlist, error) {
	now, id := repository.now().UnixMilli(), repository.newID()
	playlist := Playlist{ID: id, OwnerID: ownerPublicID, Name: name, SystemKey: systemKey, CreatedAt: now, UpdatedAt: now}
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if systemKey == nil {
			_, err := tx.ExecContext(ctx, `insert into user_playlist(id, owner_public_id, name, system_key, created_at, updated_at) values (?, ?, ?, null, ?, ?)`, id, ownerPublicID, name, now, now)
			return err
		}
		_, err := tx.ExecContext(ctx, `
			insert into user_playlist(id, owner_public_id, name, system_key, created_at, updated_at) values (?, ?, ?, ?, ?, ?)
			on conflict(owner_public_id, system_key) do update set name = excluded.name
		`, id, ownerPublicID, name, *systemKey, now, now)
		return err
	})
	return playlist, err
}

func (repository *UserPlaylistRepository) find(ctx context.Context, condition string, arguments ...any) (*Playlist, error) {
	playlist, err := scanUserPlaylist(repository.store.reader.QueryRowContext(ctx, userPlaylistSelect+condition+`
		group by p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at
	`, arguments...))
	if err != nil {
		return nil, fmt.Errorf("find user playlist: %w", err)
	}
	return &playlist, nil
}

func (repository *UserPlaylistRepository) deleteTrack(ctx context.Context, ownerPublicID, playlistID, column, value string) (bool, error) {
	var deleted bool
	now := repository.now().UnixMilli()
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if exists, err := userPlaylistExists(ctx, tx, ownerPublicID, playlistID); err != nil || !exists {
			return err
		}
		result, err := tx.ExecContext(ctx, `delete from user_playlist_track where playlist_id = ? and `+column+` = ?`, playlistID, value)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count == 0 {
			return err
		}
		deleted = true
		if err := rewriteUserPlaylistOrder(ctx, tx, ownerPublicID, playlistID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `update user_playlist set updated_at = ? where id = ?`, now, playlistID)
		return err
	})
	return deleted, err
}

func userPlaylistExists(ctx context.Context, tx *sql.Tx, ownerPublicID, playlistID string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `select count(1) from user_playlist where owner_public_id = ? and id = ?`, ownerPublicID, playlistID).Scan(&count)
	return count > 0, err
}

func rewriteUserPlaylistOrder(ctx context.Context, tx *sql.Tx, ownerPublicID, playlistID string) error {
	rows, err := tx.QueryContext(ctx, `
		select t.id from user_playlist_track t join user_playlist p on p.id = t.playlist_id
		where p.owner_public_id = ? and t.playlist_id = ? order by t.sort_order, t.created_at
	`, ownerPublicID, playlistID)
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
		if _, err := tx.ExecContext(ctx, `update user_playlist_track set sort_order = ? where id = ?`, index, id); err != nil {
			return err
		}
	}
	return nil
}

func scanUserPlaylist(row rowScanner) (Playlist, error) {
	var playlist Playlist
	var systemKey sql.NullString
	if err := row.Scan(&playlist.ID, &playlist.OwnerID, &playlist.Name, &systemKey, &playlist.CreatedAt, &playlist.UpdatedAt, &playlist.TrackCount); err != nil {
		return Playlist{}, err
	}
	if systemKey.Valid {
		playlist.SystemKey = &systemKey.String
	}
	return playlist, nil
}

const userPlaylistSelect = `
	select p.id, p.owner_public_id, p.name, p.system_key, p.created_at, p.updated_at, count(t.id) as track_count
	from user_playlist p left join user_playlist_track t on t.playlist_id = p.id
`
