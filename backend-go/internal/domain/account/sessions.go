package account

import (
	"context"
	"database/sql"
	"time"
)

const (
	AdminSessionLifetime  = 12 * time.Hour
	MemberSessionLifetime = 90 * 24 * time.Hour
)

func sessionLifetime(admin bool) time.Duration {
	if admin {
		return AdminSessionLifetime
	}
	return MemberSessionLifetime
}

// SetSessionsRevokedHook is configured before serving requests. Hashes identify
// committed revocations without exposing raw browser credentials to the hook.
func (s *Service) SetSessionsRevokedHook(hook func([]string)) { s.onSessionsRevoked = hook }
func (s *Service) sessionsRevoked(hashes []string) {
	if len(hashes) > 0 && s.onSessionsRevoked != nil {
		s.onSessionsRevoked(hashes)
	}
}

func revokeSessionsTx(ctx context.Context, tx *sql.Tx, where string, args []any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "select session_token_hash from user_session where "+where, args...)
	if err != nil {
		return nil, err
	}
	var hashes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			rows.Close()
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, "delete from user_session where "+where, args...)
	return hashes, err
}

func (s *Service) revokeSessions(ctx context.Context, where string, args []any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = context.WithoutCancel(ctx)
	var hashes []string
	err := s.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var err error
		hashes, err = revokeSessionsTx(ctx, tx, where, args)
		return err
	})
	if err == nil {
		s.sessionsRevoked(hashes)
	}
	return err
}
func (s *Service) revokeUserSessions(ctx context.Context, publicID string) error {
	return s.revokeSessions(ctx, "public_id=?", []any{publicID})
}

// CleanupExpiredSessions applies the same absolute lifetime to historical rows.
// Profiles remain intact because playlists, history and JSON can reference them.
func (s *Service) CleanupExpiredSessions(ctx context.Context) error {
	now := s.now().UnixMilli()
	return s.revokeSessions(ctx, `created_at <= ? or (created_at <= ? and exists(select 1 from user_account a where a.public_id=user_session.public_id and a.role in ('ADMIN','PLATFORM_ADMIN')))`, []any{now - MemberSessionLifetime.Milliseconds(), now - AdminSessionLifetime.Milliseconds()})
}
