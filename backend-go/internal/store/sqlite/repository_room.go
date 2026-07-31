package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type RoomRepository struct {
	store *Store
}

func NewRoomRepository(store *Store) *RoomRepository {
	return &RoomRepository{store: store}
}

func (repository *RoomRepository) FindAllActive(ctx context.Context) ([]Room, error) {
	return repository.query(ctx, `
		select id, name, owner_public_id, visibility, password_hash, password_version, system, created_at, last_active_at, deleted_at
		from room
		where deleted_at is null
		order by system desc, last_active_at desc, created_at asc
	`)
}

func (repository *RoomRepository) FindLobbyRooms(ctx context.Context, requesterPublicID *string) ([]Room, error) {
	var requester any
	if requesterPublicID != nil {
		requester = *requesterPublicID
	}
	return repository.query(ctx, `
		select id, name, owner_public_id, visibility, password_hash, password_version, system, created_at, last_active_at, deleted_at
		from room
		where deleted_at is null
		  and (system = 1 or visibility = 'PUBLIC' or (? is not null and owner_public_id = ?))
		order by system desc, last_active_at desc, created_at asc
	`, requester, requester)
}

func (repository *RoomRepository) FindByID(ctx context.Context, roomID string) (*Room, error) {
	row := repository.store.reader.QueryRowContext(ctx, `
		select id, name, owner_public_id, visibility, password_hash, password_version, system, created_at, last_active_at, deleted_at
		from room
		where id = ? and deleted_at is null
	`, roomID)
	room, err := scanRoom(row)
	if err != nil {
		return nil, fmt.Errorf("find room by id: %w", err)
	}
	return &room, nil
}

func (repository *RoomRepository) Upsert(ctx context.Context, room Room) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into room(id, name, owner_public_id, visibility, password_hash, password_version, system, created_at, last_active_at, deleted_at)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			on conflict(id) do update set
			  name = excluded.name,
			  owner_public_id = excluded.owner_public_id,
			  visibility = excluded.visibility,
			  password_hash = excluded.password_hash,
			  password_version = excluded.password_version,
			  system = excluded.system,
			  created_at = excluded.created_at,
			  last_active_at = excluded.last_active_at,
			  deleted_at = excluded.deleted_at
		`, room.ID, room.Name, room.OwnerPublicID, room.Visibility, nullableString(room.PasswordHash), room.PasswordVersion,
			room.System, room.CreatedAt, room.LastActiveAt, nullableInt64(room.DeletedAt))
		return err
	})
}

func (repository *RoomRepository) Touch(ctx context.Context, roomID string, lastActiveAt int64) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update room set last_active_at = ? where id = ?", lastActiveAt, roomID)
		return err
	})
}

func (repository *RoomRepository) SoftDelete(ctx context.Context, roomID string, deletedAt int64) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update room set deleted_at = ? where id = ?", deletedAt, roomID)
		return err
	})
}

func (repository *RoomRepository) query(ctx context.Context, query string, args ...any) ([]Room, error) {
	rows, err := repository.store.reader.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query rooms: %w", err)
	}
	defer rows.Close()
	rooms := []Room{}
	for rows.Next() {
		room, err := scanRoom(rows)
		if err != nil {
			return nil, fmt.Errorf("scan room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rooms: %w", err)
	}
	return rooms, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRoom(row rowScanner) (Room, error) {
	var room Room
	var passwordHash sql.NullString
	var deletedAt sql.NullInt64
	if err := row.Scan(&room.ID, &room.Name, &room.OwnerPublicID, &room.Visibility, &passwordHash, &room.PasswordVersion,
		&room.System, &room.CreatedAt, &room.LastActiveAt, &deletedAt); err != nil {
		return Room{}, err
	}
	if passwordHash.Valid {
		room.PasswordHash = &passwordHash.String
	}
	if deletedAt.Valid {
		room.DeletedAt = &deletedAt.Int64
	}
	return room, nil
}
