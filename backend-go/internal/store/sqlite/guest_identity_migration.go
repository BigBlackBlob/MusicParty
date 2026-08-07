package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const guestIdentityMigrationKey = "auth.guest-only.v1"

// MigrateMemberAccountsToGuests preserves public-id-owned data while removing
// non-administrator credentials and obsolete room access records.
func MigrateMemberAccountsToGuests(ctx context.Context, store *Store) error {
	return store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var completed int
		if err := tx.QueryRowContext(ctx, "select count(1) from migration_state where migration_key=?", guestIdentityMigrationKey).Scan(&completed); err != nil {
			return err
		}
		if completed > 0 {
			return nil
		}

		var adminPublicID string
		err := tx.QueryRowContext(ctx, "select public_id from user_account where role in ('PLATFORM_ADMIN','ADMIN') and enabled=1 order by created_at limit 1").Scan(&adminPublicID)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("guest-only migration requires an enabled platform administrator")
		}
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, "update room set owner_public_id=? where owner_public_id in (select public_id from user_account where role not in ('PLATFORM_ADMIN','ADMIN'))", adminPublicID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "update user_profile set is_guest=1 where public_id in (select public_id from user_account where role not in ('PLATFORM_ADMIN','ADMIN'))"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "delete from user_account where role not in ('PLATFORM_ADMIN','ADMIN')"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "delete from room_membership"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "delete from room_invite"); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "insert into migration_state(migration_key,completed_at) values(?,?)", guestIdentityMigrationKey, time.Now().UnixMilli())
		return err
	})
}
