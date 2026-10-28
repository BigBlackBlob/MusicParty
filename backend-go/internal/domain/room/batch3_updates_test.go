package room

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/stretchr/testify/require"
)

func batch3Service(t *testing.T) (*Service, account.Session) {
	t.Helper()
	store := roomStore(t)
	accounts := account.New(store)
	owner, err := accounts.CreateGuestSession(context.Background(), "Owner")
	require.NoError(t, err)
	return New(store, accounts), owner
}

// Hold callers immediately before queuing their write. This makes an outside-transaction
// version read or duplicate-name check observe the same old database state.
func batch3ConcurrentWrites(t *testing.T, s *Service, operations ...func() error) []error {
	t.Helper()
	arrived := make(chan struct{}, len(operations))
	release := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	s.now = func() time.Time { arrived <- struct{}{}; <-release; return time.Now() }
	results := make(chan error, len(operations))
	for _, operation := range operations {
		go func() { results <- operation() }()
	}
	for range operations {
		select {
		case <-arrived:
		case <-time.After(5 * time.Second):
			t.Fatal("writers did not reach the transaction boundary")
		}
	}
	once.Do(func() { close(release) })
	errors := make([]error, 0, len(operations))
	for range operations {
		select {
		case err := <-results:
			errors = append(errors, err)
		case <-time.After(5 * time.Second):
			t.Fatal("writer did not finish")
		}
	}
	return errors
}

func TestBatch3ConcurrentPasswordUpdatesAdvanceVersion(t *testing.T) {
	s, owner := batch3Service(t)
	ctx := context.Background()
	created, err := s.Create(ctx, owner.SessionToken, CreateInput{Name: "Private", Private: true, Password: "initial"})
	require.NoError(t, err)
	before, err := s.Access(ctx, created.RoomID)
	require.NoError(t, err)
	operations := []func() error{}
	for i := range 3 {
		operations = append(operations, func() error {
			_, err := s.UpdateDetails(ctx, created.RoomID, owner.SessionToken, UpdateInput{Name: "Private", Private: new(true), Password: fmt.Sprintf("password-%d", i)})
			return err
		})
	}
	for _, err := range batch3ConcurrentWrites(t, s, operations...) {
		require.NoError(t, err)
	}
	after, err := s.Access(ctx, created.RoomID)
	require.NoError(t, err)
	require.Equal(t, before.PasswordVersion+3, after.PasswordVersion)
}

func TestBatch3ConcurrentCreatesRejectDuplicateNames(t *testing.T) {
	s, owner := batch3Service(t)
	ctx := context.Background()
	operations := []func() error{}
	for _, name := range []string{" Shared ", "SHARED", "shared"} {
		operations = append(operations, func() error { _, err := s.Create(ctx, owner.SessionToken, CreateInput{Name: name}); return err })
	}
	successes := 0
	for _, err := range batch3ConcurrentWrites(t, s, operations...) {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrRoomNameExists)
		}
	}
	require.Equal(t, 1, successes)
}

func TestBatch3CreateAndRenameShareNameUniqueness(t *testing.T) {
	s, owner := batch3Service(t)
	ctx := context.Background()
	existing, err := s.Create(ctx, owner.SessionToken, CreateInput{Name: "Original"})
	require.NoError(t, err)
	successes := 0
	for _, err := range batch3ConcurrentWrites(t, s,
		func() error { _, err := s.Create(ctx, owner.SessionToken, CreateInput{Name: " New name "}); return err },
		func() error {
			_, err := s.UpdateDetails(ctx, existing.RoomID, owner.SessionToken, UpdateInput{Name: "NEW NAME"})
			return err
		},
	) {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrRoomNameExists)
		}
	}
	require.Equal(t, 1, successes)
}

func TestBatch3QueuedMutationCannotUseRevokedOwnership(t *testing.T) {
	for _, operation := range []string{"rename", "delete"} {
		t.Run(operation, func(t *testing.T) {
			fixture := seedOwnership(t)
			arrived, release := make(chan struct{}), make(chan struct{})
			fixture.service.now = func() time.Time { close(arrived); <-release; return time.Now() }
			result := make(chan error, 1)
			go func() {
				if operation == "rename" {
					_, err := fixture.service.Update(context.Background(), fixture.roomID, fixture.owner.SessionToken, "Queued rename")
					result <- err
				} else {
					deleted, err := fixture.service.Delete(context.Background(), fixture.roomID, fixture.owner.SessionToken)
					if deleted {
						result <- fmt.Errorf("revoked owner deleted the room")
					} else {
						result <- err
					}
				}
			}()
			<-arrived
			other := New(fixture.store, fixture.service.accounts)
			changed, err := other.SetOwner(context.Background(), fixture.roomID, fixture.member.PublicID, fixture.admin.SessionToken, true)
			close(release)
			require.NoError(t, err)
			require.True(t, changed)
			if operation == "rename" {
				require.ErrorIs(t, <-result, ErrUpdateForbidden)
			} else {
				require.NoError(t, <-result)
			}
			metadata, err := fixture.service.Access(context.Background(), fixture.roomID)
			require.NoError(t, err)
			require.False(t, metadata.Private)
		})
	}
}

func TestBatch3FailedDeletionKeepsRoomAndSkipsCleanup(t *testing.T) {
	s, owner := batch3Service(t)
	created, err := s.Create(context.Background(), owner.SessionToken, CreateInput{Name: "Keep"})
	require.NoError(t, err)
	calls := 0
	s.SetRoomDeletedHook(func(string) { calls++ })
	require.NoError(t, s.store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create trigger reject_room_delete before update of deleted_at on room begin select raise(abort,'forced deletion failure'); end`)
		return err
	}))
	deleted, err := s.Delete(context.Background(), created.RoomID, owner.SessionToken)
	require.Error(t, err)
	require.False(t, deleted)
	require.Zero(t, calls)
	_, err = s.Access(context.Background(), created.RoomID)
	require.NoError(t, err)
}

func TestBatch3LegacyClientCanKeepOrReplacePassword(t *testing.T) {
	s, owner := batch3Service(t)
	created, err := s.Create(context.Background(), owner.SessionToken, CreateInput{Name: "Private", Private: true, Password: "secret"})
	require.NoError(t, err)
	before, err := s.Access(context.Background(), created.RoomID)
	require.NoError(t, err)
	_, err = s.UpdateDetails(context.Background(), created.RoomID, owner.SessionToken, UpdateInput{Name: "Private", Private: new(true), KeepExistingPassword: true})
	require.NoError(t, err)
	after, err := s.Access(context.Background(), created.RoomID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, err = s.UpdateDetails(context.Background(), created.RoomID, owner.SessionToken, UpdateInput{Name: "Private", Private: new(true)})
	require.ErrorIs(t, err, ErrPrivatePasswordRequired)
	_, err = s.UpdateDetails(context.Background(), created.RoomID, owner.SessionToken, UpdateInput{Name: "Private", Private: new(false)})
	require.NoError(t, err)
	_, err = s.UpdateDetails(context.Background(), created.RoomID, owner.SessionToken, UpdateInput{Name: "Private", Private: new(true), Password: "replacement", KeepExistingPassword: true})
	require.NoError(t, err)
	after, _, err = s.VerifyAccess(context.Background(), created.RoomID, owner.SessionToken, "replacement")
	require.NoError(t, err)
	require.Equal(t, before.PasswordVersion+2, after.PasswordVersion)
}
