package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type RoomAccessRepository struct {
	store *Store
}

func NewRoomAccessRepository(store *Store) *RoomAccessRepository {
	return &RoomAccessRepository{store: store}
}

func (repository *RoomAccessRepository) UpsertMembership(ctx context.Context, membership RoomMembership) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into room_membership(room_id, public_id, role, created_at, updated_at) values (?, ?, ?, ?, ?)
			on conflict(room_id, public_id) do update set role = excluded.role, updated_at = excluded.updated_at
		`, membership.RoomID, membership.PublicID, membership.Role, membership.CreatedAt, membership.UpdatedAt)
		return err
	})
}

func (repository *RoomAccessRepository) FindMembership(ctx context.Context, roomID, publicID string) (*RoomMembership, error) {
	var membership RoomMembership
	err := repository.store.reader.QueryRowContext(ctx, `
		select room_id, public_id, role, created_at, updated_at from room_membership where room_id = ? and public_id = ?
	`, roomID, publicID).Scan(&membership.RoomID, &membership.PublicID, &membership.Role, &membership.CreatedAt, &membership.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find room membership: %w", err)
	}
	return &membership, nil
}

func (repository *RoomAccessRepository) ListMemberships(ctx context.Context, roomID string) ([]RoomMembership, error) {
	rows, err := repository.store.reader.QueryContext(ctx, `
		select room_id, public_id, role, created_at, updated_at from room_membership where room_id = ? order by created_at
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list room memberships: %w", err)
	}
	defer rows.Close()
	memberships := []RoomMembership{}
	for rows.Next() {
		var membership RoomMembership
		if err := rows.Scan(&membership.RoomID, &membership.PublicID, &membership.Role, &membership.CreatedAt, &membership.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan room membership: %w", err)
		}
		memberships = append(memberships, membership)
	}
	return memberships, rows.Err()
}

func (repository *RoomAccessRepository) CountOwners(ctx context.Context, roomID string) (int, error) {
	var count int
	err := repository.store.reader.QueryRowContext(ctx, `select count(*) from room_membership where room_id = ? and role = 'OWNER'`, roomID).Scan(&count)
	return count, err
}

func (repository *RoomAccessRepository) DeleteMembership(ctx context.Context, roomID, publicID string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `delete from room_membership where room_id = ? and public_id = ?`, roomID, publicID)
		return err
	})
}

func (repository *RoomAccessRepository) CreateInvite(ctx context.Context, invite RoomInvite) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into room_invite(id, room_id, created_by_public_id, secret_hash, label, expires_at, max_uses, used_at, used_by_public_id, revoked_at, created_at)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, invite.ID, invite.RoomID, invite.CreatedByPublicID, invite.SecretHash, nullableString(invite.Label), invite.ExpiresAt,
			invite.MaxUses, nullableInt64(invite.UsedAt), nullableString(invite.UsedByPublicID), nullableInt64(invite.RevokedAt), invite.CreatedAt)
		return err
	})
}

func (repository *RoomAccessRepository) FindInviteByID(ctx context.Context, inviteID string) (*RoomInvite, error) {
	return repository.findInvite(ctx, "where id = ?", inviteID)
}

func (repository *RoomAccessRepository) FindInviteBySecretHash(ctx context.Context, secretHash string) (*RoomInvite, error) {
	return repository.findInvite(ctx, "where secret_hash = ?", secretHash)
}

func (repository *RoomAccessRepository) ListInvites(ctx context.Context, roomID string) ([]RoomInvite, error) {
	rows, err := repository.store.reader.QueryContext(ctx, `
		select id, room_id, created_by_public_id, secret_hash, label, expires_at, max_uses, used_at, used_by_public_id, revoked_at, created_at
		from room_invite where room_id = ? order by created_at desc
	`, roomID)
	if err != nil {
		return nil, fmt.Errorf("list room invites: %w", err)
	}
	defer rows.Close()
	invites := []RoomInvite{}
	for rows.Next() {
		invite, err := scanInvite(rows)
		if err != nil {
			return nil, fmt.Errorf("scan room invite: %w", err)
		}
		invites = append(invites, invite)
	}
	return invites, rows.Err()
}

func (repository *RoomAccessRepository) ConsumeInvite(ctx context.Context, inviteID, publicID string, usedAt int64) (bool, error) {
	return repository.updateInvite(ctx, `
		update room_invite set used_at = ?, used_by_public_id = ?
		where id = ? and used_at is null and revoked_at is null and expires_at >= ?
	`, usedAt, publicID, inviteID, usedAt)
}

func (repository *RoomAccessRepository) RevokeInvite(ctx context.Context, inviteID string, revokedAt int64) (bool, error) {
	return repository.updateInvite(ctx, `update room_invite set revoked_at = ? where id = ? and revoked_at is null`, revokedAt, inviteID)
}

func (repository *RoomAccessRepository) RevokeRoomInvite(ctx context.Context, roomID, inviteID string, revokedAt int64) (bool, error) {
	return repository.updateInvite(ctx, `update room_invite set revoked_at = ? where room_id = ? and id = ? and revoked_at is null`, revokedAt, roomID, inviteID)
}

func (repository *RoomAccessRepository) findInvite(ctx context.Context, condition string, argument string) (*RoomInvite, error) {
	row := repository.store.reader.QueryRowContext(ctx, `
		select id, room_id, created_by_public_id, secret_hash, label, expires_at, max_uses, used_at, used_by_public_id, revoked_at, created_at
		from room_invite `+condition, argument)
	invite, err := scanInvite(row)
	if err != nil {
		return nil, fmt.Errorf("find room invite: %w", err)
	}
	return &invite, nil
}

func (repository *RoomAccessRepository) updateInvite(ctx context.Context, statement string, arguments ...any) (bool, error) {
	var updated bool
	err := repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, statement, arguments...)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		updated = rows == 1
		return err
	})
	return updated, err
}

func scanInvite(row rowScanner) (RoomInvite, error) {
	var invite RoomInvite
	var label, usedByPublicID sql.NullString
	var usedAt, revokedAt sql.NullInt64
	if err := row.Scan(&invite.ID, &invite.RoomID, &invite.CreatedByPublicID, &invite.SecretHash, &label, &invite.ExpiresAt,
		&invite.MaxUses, &usedAt, &usedByPublicID, &revokedAt, &invite.CreatedAt); err != nil {
		return RoomInvite{}, err
	}
	if label.Valid {
		invite.Label = &label.String
	}
	if usedAt.Valid {
		invite.UsedAt = &usedAt.Int64
	}
	if usedByPublicID.Valid {
		invite.UsedByPublicID = &usedByPublicID.String
	}
	if revokedAt.Valid {
		invite.RevokedAt = &revokedAt.Int64
	}
	return invite, nil
}
