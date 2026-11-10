package media

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestYTDLPResolverBoundsProcessesAndSharesResolution(t *testing.T) {
	resolver := newYTDLPResolver(1)
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	resolver.run = func(ctx context.Context, executable, id string) (string, error) {
		calls.Add(1)
		close(started)
		select {
		case <-release:
			return "https://cdn.test/audio", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	first := make(chan string, 1)
	go func() { value, _ := resolver.resolve(context.Background(), "unused", "abcdefghijk"); first <- value }()
	<-started
	_, err := resolver.resolve(context.Background(), "unused", "ABCDEFGHIJK")
	require.ErrorIs(t, err, ErrResolverBusy)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = resolver.resolve(canceled, "unused", "abcdefghijk")
	require.ErrorIs(t, err, context.Canceled)
	waiting, stopWaiting := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stopWaiting()
	_, err = resolver.resolve(waiting, "unused", "abcdefghijk")
	require.ErrorIs(t, err, context.DeadlineExceeded, "same-video requests join the in-flight result rather than hitting capacity")
	require.EqualValues(t, 1, calls.Load())
	close(release)
	select {
	case value := <-first:
		require.Equal(t, "https://cdn.test/audio", value)
	case <-time.After(time.Second):
		t.Fatal("resolver did not finish")
	}
	value, err := resolver.resolve(context.Background(), "unused", "abcdefghijk")
	require.NoError(t, err)
	require.Equal(t, "https://cdn.test/audio", value)
	require.EqualValues(t, 1, calls.Load(), "repeated requests must reuse one resolution")
	_, err = resolver.resolve(context.Background(), "unused", "bad&id=whatever")
	require.Error(t, err)
}
