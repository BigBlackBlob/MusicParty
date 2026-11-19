package sqlite

import (
	"context"
	"database/sql"
	"time"
)

const playlistMetadataMigrationKey = "playlist.metadata.v1"

// migratePlaylistMetadata upgrades the frozen baseline before playlist repositories are used.
func migratePlaylistMetadata(ctx context.Context, store *Store) error {
	return store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		var completed int
		if err := tx.QueryRowContext(ctx, `select count(*) from migration_state where migration_key = ?`, playlistMetadataMigrationKey).Scan(&completed); err != nil || completed > 0 {
			return err
		}
		for _, statement := range []string{
			`alter table user_playlist add column cover_url text not null default ''`,
			`alter table room_playlist add column cover_url text not null default ''`,
			`alter table user_playlist add column cover_data blob`,
			`alter table room_playlist add column cover_data blob`,
			`alter table user_playlist add column description text not null default ''`,
			`alter table room_playlist add column description text not null default ''`,
			`alter table user_playlist add column sort_order integer not null default 0`,
			`alter table user_playlist_track add column notes text not null default ''`,
			`alter table room_playlist_track add column notes text not null default ''`,
			`alter table user_playlist_track add column play_count integer not null default 0`,
			`alter table room_playlist_track add column play_count integer not null default 0`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `insert into migration_state(migration_key, completed_at) values (?, ?)`, playlistMetadataMigrationKey, time.Now().UnixMilli())
		return err
	})
}
