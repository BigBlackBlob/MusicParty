package room

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

// seedOwnership builds one public room owned by a regular member (not a platform admin) and keeps
// a second member in it, so tests can tell the owner, a plain member and an admin apart.
func seedOwnership(t *testing.T) ownershipFixture {
	t.Helper()
	ctx := context.Background()
	store := roomStore(t)
	accounts := account.New(store)
	require.NoError(t, accounts.Bootstrap(ctx, "admin", "strong-password"))
	admin, err := accounts.Login(ctx, "admin", "strong-password")
	require.NoError(t, err)
	owner, err := accounts.CreateGuestSession(ctx, "Owner")
	require.NoError(t, err)
	member, err := accounts.CreateGuestSession(ctx, "Member")
	require.NoError(t, err)
	now := time.Now().UnixMilli()
	require.NoError(t, storesqlite.NewRoomRepository(store).Upsert(ctx, storesqlite.Room{ID: "room", Name: "Room", OwnerPublicID: owner.PublicID, Visibility: "PUBLIC", CreatedAt: now, LastActiveAt: now}))
	access := storesqlite.NewRoomAccessRepository(store)
	require.NoError(t, access.UpsertMembership(ctx, storesqlite.RoomMembership{RoomID: "room", PublicID: owner.PublicID, Role: "OWNER", CreatedAt: now, UpdatedAt: now + 1}))
	require.NoError(t, access.UpsertMembership(ctx, storesqlite.RoomMembership{RoomID: "room", PublicID: member.PublicID, Role: "MEMBER", CreatedAt: now, UpdatedAt: now + 2}))
	return ownershipFixture{service: New(store, accounts), store: store, admin: admin, owner: owner, member: member, roomID: "room"}
}

type ownershipFixture struct {
	service *Service
	store   *storesqlite.Store
	admin   account.Session
	owner   account.Session
	member  account.Session
	roomID  string
}

func (f ownershipFixture) roomOwner(t *testing.T) string {
	t.Helper()
	var owner string
	require.NoError(t, f.store.Reader().QueryRowContext(context.Background(), "select owner_public_id from room where id=?", f.roomID).Scan(&owner))
	return owner
}

func (f ownershipFixture) roles(t *testing.T) map[string]string {
	t.Helper()
	rows, err := f.store.Reader().QueryContext(context.Background(), "select public_id,role from room_membership where room_id=?", f.roomID)
	require.NoError(t, err)
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var publicID, role string
		require.NoError(t, rows.Scan(&publicID, &role))
		result[publicID] = role
	}
	require.NoError(t, rows.Err())
	return result
}

// The bug: SetOwner only touched room_membership while CanEdit only read room.owner_public_id, so a
// freshly promoted OWNER could not actually rename or delete the room.
func TestOwnerGrantImmediatelyAllowsRenameAndDelete(t *testing.T) {
	ctx := context.Background()
	fixture := seedOwnership(t)

	if _, canEdit := fixture.service.CanEdit(ctx, fixture.roomID, fixture.member.SessionToken); canEdit {
		t.Fatal("a plain member must not edit the room before being promoted")
	}
	if _, err := fixture.service.UpdateDetails(ctx, fixture.roomID, fixture.member.SessionToken, UpdateInput{Name: "Member rename"}); err == nil {
		t.Fatal("a plain member must not be allowed to rename the room")
	}

	changed, err := fixture.service.SetOwner(ctx, fixture.roomID, fixture.member.PublicID, fixture.admin.SessionToken, true)
	require.NoError(t, err)
	require.True(t, changed)

	if _, canEdit := fixture.service.CanEdit(ctx, fixture.roomID, fixture.member.SessionToken); !canEdit {
		t.Fatal("a promoted OWNER must be able to edit the room")
	}
	require.Equal(t, fixture.member.PublicID, fixture.roomOwner(t), "owner_public_id must follow the OWNER grant")
	roles := fixture.roles(t)
	require.Equal(t, "OWNER", roles[fixture.member.PublicID])
	require.Equal(t, "MEMBER", roles[fixture.owner.PublicID], "the previous owner must step down to MEMBER")

	updated, err := fixture.service.UpdateDetails(ctx, fixture.roomID, fixture.member.SessionToken, UpdateInput{Name: "Promoted rename"})
	require.NoError(t, err)
	require.Equal(t, "Promoted rename", updated.Name)
	deleted, err := fixture.service.Delete(ctx, fixture.roomID, fixture.member.SessionToken)
	require.NoError(t, err)
	require.True(t, deleted)
}

// A handover must move every source of truth: the new owner can edit, the previous one cannot.
func TestOwnershipHandoverRevokesOriginalOwner(t *testing.T) {
	ctx := context.Background()
	fixture := seedOwnership(t)
	_, err := fixture.service.UpdateDetails(ctx, fixture.roomID, fixture.owner.SessionToken, UpdateInput{Name: "Owner rename"})
	require.NoError(t, err)

	changed, err := fixture.service.SetOwner(ctx, fixture.roomID, fixture.member.PublicID, fixture.admin.SessionToken, true)
	require.NoError(t, err)
	require.True(t, changed)

	if _, canEdit := fixture.service.CanEdit(ctx, fixture.roomID, fixture.owner.SessionToken); canEdit {
		t.Fatal("the previous owner must lose edit rights after a handover")
	}
	if _, err := fixture.service.UpdateDetails(ctx, fixture.roomID, fixture.owner.SessionToken, UpdateInput{Name: "Previous owner rename"}); err == nil {
		t.Fatal("the previous owner must not be allowed to rename the room")
	}
	deleted, err := fixture.service.Delete(ctx, fixture.roomID, fixture.owner.SessionToken)
	require.NoError(t, err)
	require.False(t, deleted, "the previous owner must not be allowed to delete the room")

	if _, canEdit := fixture.service.CanEdit(ctx, fixture.roomID, fixture.member.SessionToken); !canEdit {
		t.Fatal("the new owner must be able to edit the room")
	}
}

// Platform admins stay above the ownership model, and a system room stays admin-only.
func TestCanEditAlwaysAllowsSystemAdmin(t *testing.T) {
	for _, scenario := range []struct {
		name           string
		system         bool
		wantOwnerEdit  bool
		wantMemberEdit bool
		wantAdminEdit  bool
	}{
		{name: "regular room", wantOwnerEdit: true, wantMemberEdit: false, wantAdminEdit: true},
		{name: "system room", system: true, wantOwnerEdit: false, wantMemberEdit: false, wantAdminEdit: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx := context.Background()
			fixture := seedOwnership(t)
			if scenario.system {
				require.NoError(t, fixture.store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
					_, err := tx.ExecContext(ctx, "update room set system=1 where id=?", fixture.roomID)
					return err
				}))
			}
			for _, actor := range []struct {
				name  string
				token string
				want  bool
			}{
				{name: "owner", token: fixture.owner.SessionToken, want: scenario.wantOwnerEdit},
				{name: "member", token: fixture.member.SessionToken, want: scenario.wantMemberEdit},
				{name: "admin", token: fixture.admin.SessionToken, want: scenario.wantAdminEdit},
			} {
				if _, canEdit := fixture.service.CanEdit(ctx, fixture.roomID, actor.token); canEdit != actor.want {
					t.Fatalf("%s CanEdit = %v, want %v", actor.name, canEdit, actor.want)
				}
			}
		})
	}
}

// Removing the member named by owner_public_id must hand the pointer to a remaining OWNER instead
// of leaving edit rights behind with someone who is no longer a member.
func TestRemovingOwnerMovesOwnerPointer(t *testing.T) {
	ctx := context.Background()
	fixture := seedOwnership(t)
	changed, err := fixture.service.SetOwner(ctx, fixture.roomID, fixture.member.PublicID, fixture.admin.SessionToken, true)
	require.NoError(t, err)
	require.True(t, changed)
	// Legacy rows can carry several OWNER memberships; this is the only state in which a member may
	// be removed while still being the room owner.
	require.NoError(t, storesqlite.NewRoomAccessRepository(fixture.store).UpsertMembership(ctx, storesqlite.RoomMembership{RoomID: fixture.roomID, PublicID: fixture.owner.PublicID, Role: "OWNER", CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli()}))

	removed, err := fixture.service.RemoveMember(ctx, fixture.roomID, fixture.member.PublicID, fixture.admin.SessionToken)
	require.NoError(t, err)
	require.True(t, removed)
	if _, canEdit := fixture.service.CanEdit(ctx, fixture.roomID, fixture.member.SessionToken); canEdit {
		t.Fatal("a removed owner must lose edit rights")
	}
	require.Equal(t, fixture.owner.PublicID, fixture.roomOwner(t), "the pointer must move to the remaining OWNER")
}
