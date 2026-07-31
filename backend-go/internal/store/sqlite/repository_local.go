package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type LocalTrackRepository struct{ store *Store }

func NewLocalTrackRepository(store *Store) *LocalTrackRepository {
	return &LocalTrackRepository{store: store}
}

func (repository *LocalTrackRepository) FindByID(ctx context.Context, id string) (*LocalTrack, error) {
	track, err := scanLocalTrack(repository.store.reader.QueryRowContext(ctx, "select * from local_track where id = ?", id))
	if err != nil {
		return nil, fmt.Errorf("find local track: %w", err)
	}
	return &track, nil
}

func (repository *LocalTrackRepository) FindActiveByOriginalHash(ctx context.Context, originalHash string) (*LocalTrack, error) {
	if strings.TrimSpace(originalHash) == "" {
		return nil, sql.ErrNoRows
	}
	track, err := scanLocalTrack(repository.store.reader.QueryRowContext(ctx, `
		select * from local_track where original_hash = ? and status <> 'DELETED' order by created_at asc limit 1
	`, originalHash))
	if err != nil {
		return nil, fmt.Errorf("find local track by original hash: %w", err)
	}
	return &track, nil
}

func (repository *LocalTrackRepository) FindAll(ctx context.Context) ([]LocalTrack, error) {
	return repository.query(ctx, "select * from local_track where status <> 'DELETED' order by updated_at desc")
}

func (repository *LocalTrackRepository) SearchCompleted(ctx context.Context, keyword string, offset, limit int) ([]LocalTrack, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(keyword)) + "%"
	return repository.query(ctx, `
		select * from local_track
		where status = 'COMPLETED' and (lower(title) like ? or lower(artists) like ? or lower(coalesce(album, '')) like ?)
		order by updated_at desc limit ? offset ?
	`, like, like, like, max(1, limit), max(0, offset))
}

func (repository *LocalTrackRepository) Upsert(ctx context.Context, track LocalTrack) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into local_track(
				id, original_hash, original_file_name, source_path, source_mime_type, source_size_bytes,
				title, artists, album, duration_ms, cover_path, cover_mime_type, ogg_path, status,
				error_message, status_message, progress_percent, uploaded_by, created_at, updated_at, started_at, completed_at
			) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			on conflict(id) do update set
			  original_hash = excluded.original_hash, original_file_name = excluded.original_file_name,
			  source_path = excluded.source_path, source_mime_type = excluded.source_mime_type,
			  source_size_bytes = excluded.source_size_bytes, title = excluded.title, artists = excluded.artists,
			  album = excluded.album, duration_ms = excluded.duration_ms, cover_path = excluded.cover_path,
			  cover_mime_type = excluded.cover_mime_type, ogg_path = excluded.ogg_path, status = excluded.status,
			  error_message = excluded.error_message, status_message = excluded.status_message,
			  progress_percent = excluded.progress_percent, uploaded_by = excluded.uploaded_by,
			  updated_at = excluded.updated_at, started_at = excluded.started_at, completed_at = excluded.completed_at
		`, track.ID, nullableString(track.OriginalHash), nullableString(track.OriginalFileName), nullableString(track.SourcePath),
			nullableString(track.SourceMIMEType), track.SourceSizeBytes, track.Title, joinArtists(track.Artists), nullableString(track.Album),
			track.DurationMS, nullableString(track.CoverPath), nullableString(track.CoverMIMEType), nullableString(track.OGGPath), track.Status,
			nullableString(track.ErrorMessage), nullableString(track.StatusMessage), nullableInt(track.ProgressPercent), nullableString(track.UploadedBy),
			track.CreatedAt, track.UpdatedAt, nullableInt64(track.StartedAt), nullableInt64(track.CompletedAt))
		return err
	})
}

func (repository *LocalTrackRepository) UpdateStatus(ctx context.Context, id string, update LocalTrackStatusUpdate) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			update local_track set
			  status = ?, ogg_path = coalesce(?, ogg_path),
			  source_path = case when ? = 'COMPLETED' then null else source_path end,
			  error_message = ?, status_message = ?, progress_percent = ?,
			  started_at = coalesce(?, started_at), completed_at = coalesce(?, completed_at), updated_at = ?
			where id = ?
		`, update.Status, nullableString(update.OGGPath), update.Status, nullableString(update.ErrorMessage), nullableString(update.StatusMessage),
			nullableInt(update.ProgressPercent), nullableInt64(update.StartedAt), nullableInt64(update.CompletedAt), update.UpdatedAt, id)
		return err
	})
}

func (repository *LocalTrackRepository) CompleteTranscode(ctx context.Context, id, oggPath string, durationMS int64, coverPath, coverMIMEType *string, now int64) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			update local_track set status='COMPLETED', ogg_path=?, source_path=null, duration_ms=?,
			  cover_path=?, cover_mime_type=?, error_message=null, status_message='Ready',
			  progress_percent=100, completed_at=?, updated_at=? where id=?
		`, oggPath, durationMS, nullableString(coverPath), nullableString(coverMIMEType), now, now, id)
		return err
	})
}

func (repository *LocalTrackRepository) MarkDeleted(ctx context.Context, id string, updatedAt int64) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update local_track set status = 'DELETED', updated_at = ? where id = ?", updatedAt, id)
		return err
	})
}

func (repository *LocalTrackRepository) DeleteLocalReferences(ctx context.Context, trackID string) error {
	idNeedle := `"id":"` + strings.ReplaceAll(trackID, `"`, `\"`) + `"`
	platformNeedle := `"platform":"local"`
	platformPattern, idPattern := "%"+platformNeedle+"%", "%"+idNeedle+"%"
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		statements := []struct {
			query string
			args  []any
		}{
			{"delete from room_queue where music_json like ? and music_json like ?", []any{platformPattern, idPattern}},
			{"delete from room_history where music_json like ? and music_json like ?", []any{platformPattern, idPattern}},
			{"delete from room_history_track where platform = ? and music_id = ?", []any{"local", trackID}},
			{"delete from user_playlist_track where music_json like ? and music_json like ?", []any{platformPattern, idPattern}},
			{"delete from room_playlist_track where music_json like ? and music_json like ?", []any{platformPattern, idPattern}},
			{"update room_playback_state set current_music_json = null where current_music_json like ? and current_music_json like ?", []any{platformPattern, idPattern}},
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *LocalTrackRepository) FindAllowedUploadUsers(ctx context.Context) (map[string]struct{}, error) {
	rows, err := repository.store.reader.QueryContext(ctx, "select user_name from local_upload_access order by user_name")
	if err != nil {
		return nil, fmt.Errorf("find allowed upload users: %w", err)
	}
	defer rows.Close()
	users := map[string]struct{}{}
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			return nil, err
		}
		users[user] = struct{}{}
	}
	return users, rows.Err()
}

func (repository *LocalTrackRepository) GrantUploadUser(ctx context.Context, userName string, now int64) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into local_upload_access(user_name, created_at, updated_at) values (?, ?, ?)
			on conflict(user_name) do update set updated_at = excluded.updated_at
		`, userName, now, now)
		return err
	})
}

func (repository *LocalTrackRepository) RevokeUploadUser(ctx context.Context, userName string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "delete from local_upload_access where user_name = ?", userName)
		return err
	})
}

func (repository *LocalTrackRepository) query(ctx context.Context, query string, arguments ...any) ([]LocalTrack, error) {
	rows, err := repository.store.reader.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query local tracks: %w", err)
	}
	defer rows.Close()
	tracks := []LocalTrack{}
	for rows.Next() {
		track, err := scanLocalTrack(rows)
		if err != nil {
			return nil, fmt.Errorf("scan local track: %w", err)
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

func scanLocalTrack(row rowScanner) (LocalTrack, error) {
	var track LocalTrack
	var originalHash, originalFileName, sourcePath, sourceMIMEType, album, coverPath, coverMIMEType, oggPath sql.NullString
	var errorMessage, statusMessage, uploadedBy sql.NullString
	var progressPercent sql.NullInt64
	var startedAt, completedAt sql.NullInt64
	var artists string
	if err := row.Scan(&track.ID, &originalHash, &originalFileName, &sourcePath, &sourceMIMEType, &track.SourceSizeBytes,
		&track.Title, &artists, &album, &track.DurationMS, &coverPath, &coverMIMEType, &oggPath, &track.Status,
		&errorMessage, &statusMessage, &progressPercent, &uploadedBy, &track.CreatedAt, &track.UpdatedAt, &startedAt, &completedAt); err != nil {
		return LocalTrack{}, err
	}
	track.Artists = splitArtists(artists)
	setString := func(value sql.NullString, target **string) {
		if value.Valid {
			*target = &value.String
		}
	}
	setString(originalHash, &track.OriginalHash)
	setString(originalFileName, &track.OriginalFileName)
	setString(sourcePath, &track.SourcePath)
	setString(sourceMIMEType, &track.SourceMIMEType)
	setString(album, &track.Album)
	setString(coverPath, &track.CoverPath)
	setString(coverMIMEType, &track.CoverMIMEType)
	setString(oggPath, &track.OGGPath)
	setString(errorMessage, &track.ErrorMessage)
	setString(statusMessage, &track.StatusMessage)
	setString(uploadedBy, &track.UploadedBy)
	if progressPercent.Valid {
		value := int(progressPercent.Int64)
		track.ProgressPercent = &value
	}
	if startedAt.Valid {
		track.StartedAt = &startedAt.Int64
	}
	if completedAt.Valid {
		track.CompletedAt = &completedAt.Int64
	}
	return track, nil
}

func joinArtists(artists []string) string {
	if len(artists) == 0 {
		return "Unknown"
	}
	filtered := make([]string, 0, len(artists))
	for _, artist := range artists {
		if strings.TrimSpace(artist) != "" {
			filtered = append(filtered, artist)
		}
	}
	return strings.Join(filtered, "; ")
}

func splitArtists(artists string) []string {
	if strings.TrimSpace(artists) == "" {
		return []string{"Unknown"}
	}
	parts := strings.Split(artists, ";")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}
