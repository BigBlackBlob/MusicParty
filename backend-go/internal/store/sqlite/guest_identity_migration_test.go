package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMigrateMemberAccountsToGuestsPreservesPublicIDData(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, StoreConfig{Path: filepath.Join(t.TempDir(), "migration.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, EnsureCompatibleSchema(ctx, store, true))

	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		statements := []string{
			"insert into user_profile(public_id,display_name,is_guest,current_room_id,created_at,last_seen_at) values('admin','Admin',0,'lounge',1,1),('member','Member',0,'lounge',1,1)",
			"insert into user_account(username,public_id,password_hash,role,enabled,created_at,updated_at) values('admin','admin','hash','PLATFORM_ADMIN',1,1,1),('member','member','hash','MEMBER',1,1,1)",
			"insert into user_session(session_token_hash,public_id,created_at,last_seen_at) values('session','member',1,1)",
			"insert into room(id,name,owner_public_id,visibility,system,created_at,last_active_at) values('lounge','Lounge','member','PUBLIC',1,1,1)",
			"insert into room_membership(room_id,public_id,role,created_at,updated_at) values('lounge','member','OWNER',1,1)",
			"insert into room_invite(id,room_id,created_by_public_id,secret_hash,expires_at,max_uses,created_at) values('invite','lounge','member','secret',9999999999999,1,1)",
			"insert into user_playlist(id,owner_public_id,name,created_at,updated_at) values('playlist','member','Saved',1,1)",
			"insert into user_binding(public_id,platform,account_id) values('member','netease','42')",
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		return nil
	}))

	require.NoError(t, MigrateMemberAccountsToGuests(ctx, store))
	require.NoError(t, MigrateMemberAccountsToGuests(ctx, store))

	var isGuest, accounts, sessions, playlists, bindings, memberships, invites int
	var owner string
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select is_guest from user_profile where public_id='member'").Scan(&isGuest))
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from user_account where public_id='member'").Scan(&accounts))
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from user_session where public_id='member'").Scan(&sessions))
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from user_playlist where owner_public_id='member'").Scan(&playlists))
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from user_binding where public_id='member'").Scan(&bindings))
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from room_membership").Scan(&memberships))
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from room_invite").Scan(&invites))
	require.NoError(t, store.Reader().QueryRowContext(ctx, "select owner_public_id from room where id='lounge'").Scan(&owner))

	require.Equal(t, 1, isGuest)
	require.Zero(t, accounts)
	require.Equal(t, 1, sessions)
	require.Equal(t, 1, playlists)
	require.Equal(t, 1, bindings)
	require.Zero(t, memberships)
	require.Zero(t, invites)
	require.Equal(t, "admin", owner)
}
