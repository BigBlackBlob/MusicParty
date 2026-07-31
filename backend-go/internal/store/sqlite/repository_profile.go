package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type UserProfileRepository struct {
	store *Store
	now   func() time.Time
}

func NewUserProfileRepository(store *Store, now func() time.Time) *UserProfileRepository {
	if now == nil {
		now = time.Now
	}
	return &UserProfileRepository{store: store, now: now}
}

func (repository *UserProfileRepository) UpsertProfile(ctx context.Context, profile UserProfile) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into user_profile(public_id, display_name, is_guest, current_room_id, created_at, last_seen_at)
			values (?, ?, ?, ?, ?, ?)
			on conflict(public_id) do update set
			  display_name = excluded.display_name,
			  is_guest = excluded.is_guest,
			  current_room_id = excluded.current_room_id,
			  created_at = excluded.created_at,
			  last_seen_at = excluded.last_seen_at
		`, profile.PublicID, profile.DisplayName, profile.Guest, profile.CurrentRoomID, profile.CreatedAt, profile.LastSeenAt)
		return err
	})
}

func (repository *UserProfileRepository) FindByPublicID(ctx context.Context, publicID string) (*UserProfile, error) {
	var profile UserProfile
	err := repository.store.reader.QueryRowContext(ctx, `
		select public_id, display_name, is_guest, current_room_id, created_at, last_seen_at
		from user_profile where public_id = ?
	`, publicID).Scan(&profile.PublicID, &profile.DisplayName, &profile.Guest, &profile.CurrentRoomID, &profile.CreatedAt, &profile.LastSeenAt)
	if err != nil {
		return nil, fmt.Errorf("find user profile: %w", err)
	}
	return &profile, nil
}

func (repository *UserProfileRepository) FindBindingsByPublicID(ctx context.Context, publicID string) (map[string]string, error) {
	rows, err := repository.store.reader.QueryContext(ctx, `
		select platform, account_id from user_binding where public_id = ? order by platform asc
	`, publicID)
	if err != nil {
		return nil, fmt.Errorf("find user bindings: %w", err)
	}
	defer rows.Close()
	bindings := map[string]string{}
	for rows.Next() {
		var platform, accountID string
		if err := rows.Scan(&platform, &accountID); err != nil {
			return nil, fmt.Errorf("scan user binding: %w", err)
		}
		bindings[platform] = accountID
	}
	return bindings, rows.Err()
}

func (repository *UserProfileRepository) ReplaceBindings(ctx context.Context, publicID string, bindings map[string]string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "delete from user_binding where public_id = ?", publicID); err != nil {
			return err
		}
		platforms := make([]string, 0, len(bindings))
		for platform := range bindings {
			platforms = append(platforms, platform)
		}
		sort.Strings(platforms)
		for _, platform := range platforms {
			if _, err := tx.ExecContext(ctx, `insert into user_binding(public_id, platform, account_id) values (?, ?, ?)`, publicID, platform, bindings[platform]); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *UserProfileRepository) UpsertSession(ctx context.Context, session Session) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into user_session(session_token_hash, public_id, created_at, last_seen_at)
			values (?, ?, ?, ?)
			on conflict(session_token_hash) do update set
			  public_id = excluded.public_id,
			  created_at = excluded.created_at,
			  last_seen_at = excluded.last_seen_at
		`, session.SessionTokenHash, session.PublicID, session.CreatedAt, session.LastSeenAt)
		return err
	})
}

func (repository *UserProfileRepository) FindSessionByHash(ctx context.Context, sessionTokenHash string) (*Session, error) {
	var session Session
	err := repository.store.reader.QueryRowContext(ctx, `
		select session_token_hash, public_id, created_at, last_seen_at
		from user_session where session_token_hash = ?
	`, sessionTokenHash).Scan(&session.SessionTokenHash, &session.PublicID, &session.CreatedAt, &session.LastSeenAt)
	if err != nil {
		return nil, fmt.Errorf("find user session: %w", err)
	}
	return &session, nil
}

func (repository *UserProfileRepository) DeleteSessionByHash(ctx context.Context, sessionTokenHash string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "delete from user_session where session_token_hash = ?", sessionTokenHash)
		return err
	})
}

func (repository *UserProfileRepository) MoveUsersToRoom(ctx context.Context, fromRoomID, toRoomID string) error {
	lastSeenAt := repository.now().UnixMilli()
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			update user_profile set current_room_id = ?, last_seen_at = ? where current_room_id = ?
		`, toRoomID, lastSeenAt, fromRoomID)
		return err
	})
}
