package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type UserAccountRepository struct {
	store *Store
}

func NewUserAccountRepository(store *Store) *UserAccountRepository {
	return &UserAccountRepository{store: store}
}

func (repository *UserAccountRepository) HasAdminAccount(ctx context.Context) (bool, error) {
	var count int
	err := repository.store.reader.QueryRowContext(ctx, `
		select count(1) from user_account where role in ('PLATFORM_ADMIN', 'ADMIN') and enabled = 1
	`).Scan(&count)
	return count > 0, err
}

func (repository *UserAccountRepository) ClaimAdminBootstrap(ctx context.Context, claimedAt int64) (bool, error) {
	var claimed bool
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			insert or ignore into admin_bootstrap_claim(claim_key, claimed_at) values ('initial-admin', ?)
		`, claimedAt)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		claimed = rows == 1
		return err
	})
	return claimed, err
}

func (repository *UserAccountRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	var count int
	err := repository.store.reader.QueryRowContext(ctx, "select count(1) from user_account where username = ?", username).Scan(&count)
	return count > 0, err
}

func (repository *UserAccountRepository) Create(ctx context.Context, account UserAccount) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into user_account(username, public_id, password_hash, role, enabled, created_at, updated_at, last_login_at)
			values (?, ?, ?, ?, ?, ?, ?, ?)
		`, account.Username, account.PublicID, account.PasswordHash, account.Role, account.Enabled, account.CreatedAt, account.UpdatedAt, nullableInt64(account.LastLoginAt))
		return err
	})
}

func (repository *UserAccountRepository) UpdateLoginTime(ctx context.Context, username string, lastLoginAt int64) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update user_account set last_login_at = ?, updated_at = ? where username = ?`, lastLoginAt, lastLoginAt, username)
		return err
	})
}

func (repository *UserAccountRepository) UpdatePasswordHash(ctx context.Context, username, passwordHash string, updatedAt int64) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update user_account set password_hash = ?, updated_at = ? where username = ?`, passwordHash, updatedAt, username)
		return err
	})
}

func (repository *UserAccountRepository) FindByUsername(ctx context.Context, username string) (*UserAccount, error) {
	return repository.find(ctx, "where username = ?", username)
}

func (repository *UserAccountRepository) FindByPublicID(ctx context.Context, publicID string) (*UserAccount, error) {
	return repository.find(ctx, "where public_id = ?", publicID)
}

func (repository *UserAccountRepository) find(ctx context.Context, condition string, argument string) (*UserAccount, error) {
	row := repository.store.reader.QueryRowContext(ctx, `
		select username, public_id, password_hash, role, enabled, created_at, updated_at, last_login_at
		from user_account `+condition, argument)
	var account UserAccount
	var lastLoginAt sql.NullInt64
	if err := row.Scan(&account.Username, &account.PublicID, &account.PasswordHash, &account.Role, &account.Enabled,
		&account.CreatedAt, &account.UpdatedAt, &lastLoginAt); err != nil {
		return nil, fmt.Errorf("find user account: %w", err)
	}
	if lastLoginAt.Valid {
		account.LastLoginAt = &lastLoginAt.Int64
	}
	return &account, nil
}

type MigrationStateRepository struct {
	store *Store
	now   func() time.Time
}

func NewMigrationStateRepository(store *Store, now func() time.Time) *MigrationStateRepository {
	if now == nil {
		now = time.Now
	}
	return &MigrationStateRepository{store: store, now: now}
}

func (repository *MigrationStateRepository) IsCompleted(ctx context.Context, migrationKey string) (bool, error) {
	var count int
	err := repository.store.reader.QueryRowContext(ctx, `
		select count(1) from migration_state where migration_key = ? and completed_at is not null
	`, migrationKey).Scan(&count)
	return count > 0, err
}

func (repository *MigrationStateRepository) MarkCompleted(ctx context.Context, migrationKey string) error {
	completedAt := repository.now().UnixMilli()
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into migration_state(migration_key, completed_at) values (?, ?)
			on conflict(migration_key) do update set completed_at = excluded.completed_at
		`, migrationKey, completedAt)
		return err
	})
}
