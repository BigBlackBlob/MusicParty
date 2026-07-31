package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type SiteSettingRepository struct{ store *Store }

func NewSiteSettingRepository(store *Store) *SiteSettingRepository {
	return &SiteSettingRepository{store: store}
}

func (repository *SiteSettingRepository) Find(ctx context.Context, key string) (*SiteSetting, error) {
	var setting SiteSetting
	var value sql.NullString
	err := repository.store.reader.QueryRowContext(ctx, `
		select setting_key, setting_value, secret, updated_at from site_setting where setting_key = ?
	`, key).Scan(&setting.Key, &value, &setting.Secret, &setting.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find site setting: %w", err)
	}
	if value.Valid {
		setting.Value = &value.String
	}
	return &setting, nil
}

func (repository *SiteSettingRepository) FindValue(ctx context.Context, key string) (*string, error) {
	setting, err := repository.Find(ctx, key)
	if err != nil {
		return nil, err
	}
	return setting.Value, nil
}

func (repository *SiteSettingRepository) Upsert(ctx context.Context, setting SiteSetting) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into site_setting(setting_key, setting_value, secret, updated_at) values (?, ?, ?, ?)
			on conflict(setting_key) do update set
			  setting_value = excluded.setting_value, secret = excluded.secret, updated_at = excluded.updated_at
		`, setting.Key, nullableString(setting.Value), setting.Secret, setting.UpdatedAt)
		return err
	})
}

type SubsonicSourceRepository struct{ store *Store }

func NewSubsonicSourceRepository(store *Store) *SubsonicSourceRepository {
	return &SubsonicSourceRepository{store: store}
}

func (repository *SubsonicSourceRepository) FindAll(ctx context.Context) ([]SubsonicSource, error) {
	rows, err := repository.store.reader.QueryContext(ctx, `
		select id, owner_room_id, label, base_url, username, password, client, api_version, allowed_users,
		       enabled, system, created_at, updated_at
		from subsonic_source order by system desc, label asc
	`)
	if err != nil {
		return nil, fmt.Errorf("find all Subsonic sources: %w", err)
	}
	defer rows.Close()
	return scanSubsonicSources(rows)
}

func (repository *SubsonicSourceRepository) FindByID(ctx context.Context, id string) (*SubsonicSource, error) {
	row := repository.store.reader.QueryRowContext(ctx, `
		select id, owner_room_id, label, base_url, username, password, client, api_version, allowed_users,
		       enabled, system, created_at, updated_at
		from subsonic_source where id = ?
	`, id)
	source, err := scanSubsonicSource(row)
	if err != nil {
		return nil, fmt.Errorf("find Subsonic source: %w", err)
	}
	return &source, nil
}

func (repository *SubsonicSourceRepository) Upsert(ctx context.Context, source SubsonicSource) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into subsonic_source(id, label, base_url, username, password, client, api_version,
			                            allowed_users, enabled, system, owner_room_id, created_at, updated_at)
			values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			on conflict(id) do update set
			  label = excluded.label, base_url = excluded.base_url, username = excluded.username,
			  password = excluded.password, client = excluded.client, api_version = excluded.api_version,
			  allowed_users = excluded.allowed_users, enabled = excluded.enabled, system = excluded.system,
			  owner_room_id = excluded.owner_room_id, updated_at = excluded.updated_at
		`, source.ID, source.Label, source.BaseURL, source.Username, source.Password, source.Client, source.APIVersion,
			nullableString(source.AllowedUsers), source.Enabled, source.System, nullableString(source.OwnerRoomID), source.CreatedAt, source.UpdatedAt)
		return err
	})
}

func (repository *SubsonicSourceRepository) Delete(ctx context.Context, id string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "delete from subsonic_source where id = ? and system = 0", id)
		return err
	})
}

func (repository *SubsonicSourceRepository) FindRoomBindings(ctx context.Context, roomID string) ([]RoomSubsonicSource, error) {
	return repository.queryBindings(ctx, `
		select room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at
		from room_subsonic_source where room_id = ? order by sort_order asc, source_id asc
	`, roomID)
}

func (repository *SubsonicSourceRepository) FindAllRoomBindings(ctx context.Context) ([]RoomSubsonicSource, error) {
	return repository.queryBindings(ctx, `
		select room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at
		from room_subsonic_source order by room_id asc, sort_order asc, source_id asc
	`)
}

func (repository *SubsonicSourceRepository) FindRoomBinding(ctx context.Context, roomID, sourceID string) (*RoomSubsonicSource, error) {
	row := repository.store.reader.QueryRowContext(ctx, `
		select room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at
		from room_subsonic_source where room_id = ? and source_id = ?
	`, roomID, sourceID)
	binding, err := scanRoomSubsonicSource(row)
	if err != nil {
		return nil, fmt.Errorf("find room Subsonic binding: %w", err)
	}
	return &binding, nil
}

func (repository *SubsonicSourceRepository) UpsertRoomBinding(ctx context.Context, binding RoomSubsonicSource) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			insert into room_subsonic_source(room_id, source_id, enabled, display_label, allowed_users, sort_order, created_at, updated_at)
			values (?, ?, ?, ?, ?, ?, ?, ?)
			on conflict(room_id, source_id) do update set
			  enabled = excluded.enabled, display_label = excluded.display_label, allowed_users = excluded.allowed_users,
			  sort_order = excluded.sort_order, updated_at = excluded.updated_at
		`, binding.RoomID, binding.SourceID, binding.Enabled, nullableString(binding.DisplayLabel), nullableString(binding.AllowedUsers),
			binding.SortOrder, binding.CreatedAt, binding.UpdatedAt)
		return err
	})
}

func (repository *SubsonicSourceRepository) DeleteRoomBinding(ctx context.Context, roomID, sourceID string) error {
	return repository.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "delete from room_subsonic_source where room_id = ? and source_id = ?", roomID, sourceID)
		return err
	})
}

func (repository *SubsonicSourceRepository) queryBindings(ctx context.Context, query string, arguments ...any) ([]RoomSubsonicSource, error) {
	rows, err := repository.store.reader.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query room Subsonic bindings: %w", err)
	}
	defer rows.Close()
	bindings := []RoomSubsonicSource{}
	for rows.Next() {
		binding, err := scanRoomSubsonicSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan room Subsonic binding: %w", err)
		}
		bindings = append(bindings, binding)
	}
	return bindings, rows.Err()
}

func scanSubsonicSources(rows *sql.Rows) ([]SubsonicSource, error) {
	sources := []SubsonicSource{}
	for rows.Next() {
		source, err := scanSubsonicSource(rows)
		if err != nil {
			return nil, fmt.Errorf("scan Subsonic source: %w", err)
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func scanSubsonicSource(row rowScanner) (SubsonicSource, error) {
	var source SubsonicSource
	var ownerRoomID, allowedUsers sql.NullString
	if err := row.Scan(&source.ID, &ownerRoomID, &source.Label, &source.BaseURL, &source.Username, &source.Password,
		&source.Client, &source.APIVersion, &allowedUsers, &source.Enabled, &source.System, &source.CreatedAt, &source.UpdatedAt); err != nil {
		return SubsonicSource{}, err
	}
	if ownerRoomID.Valid {
		source.OwnerRoomID = &ownerRoomID.String
	}
	if allowedUsers.Valid {
		source.AllowedUsers = &allowedUsers.String
	}
	return source, nil
}

func scanRoomSubsonicSource(row rowScanner) (RoomSubsonicSource, error) {
	var binding RoomSubsonicSource
	var displayLabel, allowedUsers sql.NullString
	if err := row.Scan(&binding.RoomID, &binding.SourceID, &binding.Enabled, &displayLabel, &allowedUsers,
		&binding.SortOrder, &binding.CreatedAt, &binding.UpdatedAt); err != nil {
		return RoomSubsonicSource{}, err
	}
	if displayLabel.Valid {
		binding.DisplayLabel = &displayLabel.String
	}
	if allowedUsers.Valid {
		binding.AllowedUsers = &allowedUsers.String
	}
	return binding, nil
}
