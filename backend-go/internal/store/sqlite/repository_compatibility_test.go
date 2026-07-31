package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestRoomRepositoryMatchesJavaQueriesAndOrdering(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	repository := NewRoomRepository(store)
	rooms := []Room{
		{ID: "private-other", Name: "Other", OwnerPublicID: "u-other", Visibility: "PRIVATE", CreatedAt: 1, LastActiveAt: 400},
		{ID: "public", Name: "Public", OwnerPublicID: "u-other", Visibility: "PUBLIC", CreatedAt: 2, LastActiveAt: 300},
		{ID: "private-owned", Name: "Owned", OwnerPublicID: "u-owner", Visibility: "PRIVATE", CreatedAt: 3, LastActiveAt: 200},
		{ID: "lounge", Name: "Lounge", OwnerPublicID: "system", Visibility: "PUBLIC", System: true, CreatedAt: 4, LastActiveAt: 100},
	}
	for _, room := range rooms {
		if err := repository.Upsert(ctx, room); err != nil {
			t.Fatalf("Upsert(%s): %v", room.ID, err)
		}
	}

	active, err := repository.FindAllActive(ctx)
	if err != nil {
		t.Fatalf("FindAllActive(): %v", err)
	}
	if got := roomIDs(active); !cmp.Equal(got, []string{"lounge", "private-other", "public", "private-owned"}) {
		t.Fatalf("active room order mismatch (-got +want):\n%s", cmp.Diff(got, []string{"lounge", "private-other", "public", "private-owned"}))
	}
	requester := "u-owner"
	lobby, err := repository.FindLobbyRooms(ctx, &requester)
	if err != nil {
		t.Fatalf("FindLobbyRooms(owner): %v", err)
	}
	if got := roomIDs(lobby); !cmp.Equal(got, []string{"lounge", "public", "private-owned"}) {
		t.Fatalf("owner lobby mismatch (-got +want):\n%s", cmp.Diff(got, []string{"lounge", "public", "private-owned"}))
	}
	lobby, err = repository.FindLobbyRooms(ctx, nil)
	if err != nil {
		t.Fatalf("FindLobbyRooms(nil): %v", err)
	}
	if got := roomIDs(lobby); !cmp.Equal(got, []string{"lounge", "public"}) {
		t.Fatalf("anonymous lobby mismatch (-got +want):\n%s", cmp.Diff(got, []string{"lounge", "public"}))
	}
	if err := repository.Touch(ctx, "private-owned", 999); err != nil {
		t.Fatalf("Touch(): %v", err)
	}
	if err := repository.SoftDelete(ctx, "private-owned", 1000); err != nil {
		t.Fatalf("SoftDelete(): %v", err)
	}
	if _, err := repository.FindByID(ctx, "private-owned"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindByID(deleted) error = %v, want sql.ErrNoRows", err)
	}
}

func TestUserProfileSessionBindingAndAccountRepositories(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	fixedNow := time.UnixMilli(9000)
	profiles := NewUserProfileRepository(store, func() time.Time { return fixedNow })
	accounts := NewUserAccountRepository(store)
	profile := UserProfile{PublicID: "u-owner", DisplayName: "Owner", CurrentRoomID: "old-room", CreatedAt: 100, LastSeenAt: 200}
	if err := profiles.UpsertProfile(ctx, profile); err != nil {
		t.Fatalf("UpsertProfile(): %v", err)
	}
	bindings := map[string]string{"youtube": "yt-1", "netease": "ne-1"}
	if err := profiles.ReplaceBindings(ctx, profile.PublicID, bindings); err != nil {
		t.Fatalf("ReplaceBindings(): %v", err)
	}
	gotBindings, err := profiles.FindBindingsByPublicID(ctx, profile.PublicID)
	if err != nil {
		t.Fatalf("FindBindingsByPublicID(): %v", err)
	}
	if diff := cmp.Diff(bindings, gotBindings); diff != "" {
		t.Fatalf("bindings mismatch (-want +got):\n%s", diff)
	}
	session := Session{SessionTokenHash: "hash", PublicID: profile.PublicID, CreatedAt: 300, LastSeenAt: 400}
	if err := profiles.UpsertSession(ctx, session); err != nil {
		t.Fatalf("UpsertSession(): %v", err)
	}
	gotSession, err := profiles.FindSessionByHash(ctx, session.SessionTokenHash)
	if err != nil {
		t.Fatalf("FindSessionByHash(): %v", err)
	}
	if diff := cmp.Diff(&session, gotSession); diff != "" {
		t.Fatalf("session mismatch (-want +got):\n%s", diff)
	}
	if err := profiles.MoveUsersToRoom(ctx, "old-room", "lounge"); err != nil {
		t.Fatalf("MoveUsersToRoom(): %v", err)
	}
	gotProfile, err := profiles.FindByPublicID(ctx, profile.PublicID)
	if err != nil {
		t.Fatalf("FindByPublicID(): %v", err)
	}
	if gotProfile.CurrentRoomID != "lounge" || gotProfile.LastSeenAt != 9000 {
		t.Fatalf("moved profile = %+v, want room lounge and lastSeenAt 9000", gotProfile)
	}

	account := UserAccount{Username: "owner", PublicID: profile.PublicID, PasswordHash: "bcrypt", Role: "PLATFORM_ADMIN", Enabled: true, CreatedAt: 500, UpdatedAt: 600}
	if err := accounts.Create(ctx, account); err != nil {
		t.Fatalf("Create(account): %v", err)
	}
	hasAdmin, err := accounts.HasAdminAccount(ctx)
	if err != nil || !hasAdmin {
		t.Fatalf("HasAdminAccount() = %v, %v; want true, nil", hasAdmin, err)
	}
	claimed, err := accounts.ClaimAdminBootstrap(ctx, 700)
	if err != nil || !claimed {
		t.Fatalf("first ClaimAdminBootstrap() = %v, %v; want true, nil", claimed, err)
	}
	claimed, err = accounts.ClaimAdminBootstrap(ctx, 800)
	if err != nil || claimed {
		t.Fatalf("second ClaimAdminBootstrap() = %v, %v; want false, nil", claimed, err)
	}
	if err := accounts.UpdateLoginTime(ctx, account.Username, 1000); err != nil {
		t.Fatalf("UpdateLoginTime(): %v", err)
	}
	if err := accounts.UpdatePasswordHash(ctx, account.Username, "new-hash", 1100); err != nil {
		t.Fatalf("UpdatePasswordHash(): %v", err)
	}
	gotAccount, err := accounts.FindByPublicID(ctx, profile.PublicID)
	if err != nil {
		t.Fatalf("FindByPublicID(account): %v", err)
	}
	if gotAccount.PasswordHash != "new-hash" || gotAccount.UpdatedAt != 1100 || gotAccount.LastLoginAt == nil || *gotAccount.LastLoginAt != 1000 {
		t.Fatalf("updated account = %+v", gotAccount)
	}
	if err := profiles.DeleteSessionByHash(ctx, session.SessionTokenHash); err != nil {
		t.Fatalf("DeleteSessionByHash(): %v", err)
	}
	if _, err := profiles.FindSessionByHash(ctx, session.SessionTokenHash); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindSessionByHash(deleted) error = %v, want sql.ErrNoRows", err)
	}
}

func TestRoomAccessAndMigrationRepositoriesMatchJavaMutationRules(t *testing.T) {
	store := openContractStore(t)
	defer store.Close()
	ctx := context.Background()
	profiles := NewUserProfileRepository(store, nil)
	rooms := NewRoomRepository(store)
	access := NewRoomAccessRepository(store)
	for _, profile := range []UserProfile{
		{PublicID: "u-owner", DisplayName: "Owner", CurrentRoomID: "room", CreatedAt: 1, LastSeenAt: 1},
		{PublicID: "u-member", DisplayName: "Member", CurrentRoomID: "room", CreatedAt: 2, LastSeenAt: 2},
	} {
		if err := profiles.UpsertProfile(ctx, profile); err != nil {
			t.Fatalf("UpsertProfile(%s): %v", profile.PublicID, err)
		}
	}
	if err := rooms.Upsert(ctx, Room{ID: "room", Name: "Room", OwnerPublicID: "u-owner", Visibility: "PRIVATE", CreatedAt: 1, LastActiveAt: 1}); err != nil {
		t.Fatalf("Upsert(room): %v", err)
	}
	owner := RoomMembership{RoomID: "room", PublicID: "u-owner", Role: "OWNER", CreatedAt: 10, UpdatedAt: 10}
	member := RoomMembership{RoomID: "room", PublicID: "u-member", Role: "MEMBER", CreatedAt: 20, UpdatedAt: 20}
	for _, membership := range []RoomMembership{member, owner} {
		if err := access.UpsertMembership(ctx, membership); err != nil {
			t.Fatalf("UpsertMembership(%s): %v", membership.PublicID, err)
		}
	}
	memberships, err := access.ListMemberships(ctx, "room")
	if err != nil {
		t.Fatalf("ListMemberships(): %v", err)
	}
	if got := []string{memberships[0].PublicID, memberships[1].PublicID}; !cmp.Equal(got, []string{"u-owner", "u-member"}) {
		t.Fatalf("membership order mismatch (-got +want):\n%s", cmp.Diff(got, []string{"u-owner", "u-member"}))
	}
	owners, err := access.CountOwners(ctx, "room")
	if err != nil || owners != 1 {
		t.Fatalf("CountOwners() = %d, %v; want 1, nil", owners, err)
	}
	label := "first invite"
	invite := RoomInvite{ID: "invite", RoomID: "room", CreatedByPublicID: "u-owner", SecretHash: "secret", Label: &label, ExpiresAt: 1000, MaxUses: 5, CreatedAt: 100}
	if err := access.CreateInvite(ctx, invite); err != nil {
		t.Fatalf("CreateInvite(): %v", err)
	}
	gotInvite, err := access.FindInviteBySecretHash(ctx, invite.SecretHash)
	if err != nil {
		t.Fatalf("FindInviteBySecretHash(): %v", err)
	}
	if !gotInvite.Active(1000) || gotInvite.Active(1001) {
		t.Fatalf("invite boundary activity mismatch: %+v", gotInvite)
	}
	consumed, err := access.ConsumeInvite(ctx, invite.ID, "u-member", 1000)
	if err != nil || !consumed {
		t.Fatalf("first ConsumeInvite() = %v, %v; want true, nil", consumed, err)
	}
	consumed, err = access.ConsumeInvite(ctx, invite.ID, "u-member", 1000)
	if err != nil || consumed {
		t.Fatalf("second ConsumeInvite() = %v, %v; want false, nil", consumed, err)
	}

	migrations := NewMigrationStateRepository(store, func() time.Time { return time.UnixMilli(12345) })
	if err := migrations.MarkCompleted(ctx, "go.compatibility"); err != nil {
		t.Fatalf("MarkCompleted(): %v", err)
	}
	completed, err := migrations.IsCompleted(ctx, "go.compatibility")
	if err != nil || !completed {
		t.Fatalf("IsCompleted() = %v, %v; want true, nil", completed, err)
	}
	var completedAt int64
	if err := store.Reader().QueryRowContext(ctx, `select completed_at from migration_state where migration_key = 'go.compatibility'`).Scan(&completedAt); err != nil {
		t.Fatalf("read migration timestamp: %v", err)
	}
	if completedAt != 12345 {
		t.Fatalf("completed_at = %d, want 12345", completedAt)
	}
}

func openContractStore(t *testing.T) *Store {
	t.Helper()
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "contract.db"), BusyTimeout: time.Second, ReadConnections: 2})
	schemaSQL, err := os.ReadFile(filepath.Join(contractSchemaDirectory(t), "schema.sql"))
	if err != nil {
		store.Close()
		t.Fatalf("read contract schema: %v", err)
	}
	if err := store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, string(schemaSQL))
		return err
	}); err != nil {
		store.Close()
		t.Fatalf("apply contract schema: %v", err)
	}
	return store
}

func roomIDs(rooms []Room) []string {
	ids := make([]string, len(rooms))
	for index, room := range rooms {
		ids[index] = room.ID
	}
	return ids
}
