package account

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// A failing account lookup must surface as an error instead of falling through to
// the guest branch, which would hand an authenticated caller a lower-privileged
// but still valid identity.
func TestResolveReportsDatabaseFailureInsteadOfGuest(t *testing.T) {
	store := testStore(t)
	service := New(store)
	seedSessionFixture(t, store, "closed-store-token", sessionFixture{account: true, accountEnabled: true})
	require.NoError(t, store.Close())

	session, err := service.Resolve(context.Background(), "closed-store-token")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrUnknownSession)
	require.Empty(t, session.PublicID)
	require.False(t, session.Guest)
}
