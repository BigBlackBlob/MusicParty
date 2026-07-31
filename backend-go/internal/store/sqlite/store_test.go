package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStoreWriteCommitsBeforeReturn(t *testing.T) {
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "commit.db")})
	defer store.Close()
	ctx := context.Background()
	if err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE example (id TEXT PRIMARY KEY)`)
		return err
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO example(id) VALUES ('committed')`)
		return err
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var count int
	if err := store.Reader().QueryRowContext(ctx, `SELECT count(*) FROM example WHERE id = 'committed'`).Scan(&count); err != nil {
		t.Fatalf("read committed row: %v", err)
	}
	if count != 1 {
		t.Fatalf("committed row count = %d, want 1", count)
	}
}

func TestStoreWriteRollsBackFailure(t *testing.T) {
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "rollback.db")})
	defer store.Close()
	ctx := context.Background()
	if err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE example (id TEXT PRIMARY KEY)`)
		return err
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	wantErr := errors.New("abort write")
	err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO example(id) VALUES ('rolled-back')`); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Write() error = %v, want %v", err, wantErr)
	}
	var count int
	if err := store.Reader().QueryRowContext(ctx, `SELECT count(*) FROM example`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("row count after rollback = %d, want 0", count)
	}
}

func TestStoreRejectsWhenWriteQueueIsFullAndHonorsCancellation(t *testing.T) {
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "queue.db"), WriteQueueCapacity: 1})
	defer store.Close()
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- store.Write(context.Background(), func(context.Context, *sql.Tx) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	secondCtx, cancelSecond := context.WithCancel(context.Background())
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- store.Write(secondCtx, func(context.Context, *sql.Tx) error { return nil })
	}()
	waitForQueueDepth(t, store, 1)
	if err := store.Write(context.Background(), func(context.Context, *sql.Tx) error { return nil }); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("full queue Write() error = %v, want ErrQueueFull", err)
	}
	cancelSecond()
	if err := <-secondDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled queued Write() error = %v, want context.Canceled", err)
	}
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
}

func TestStoreCloseCancelsActiveAndRejectsPendingWrites(t *testing.T) {
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "close.db"), WriteQueueCapacity: 1})
	started := make(chan struct{})
	activeDone := make(chan error, 1)
	go func() {
		activeDone <- store.Write(context.Background(), func(ctx context.Context, _ *sql.Tx) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	<-started
	pendingDone := make(chan error, 1)
	go func() {
		pendingDone <- store.Write(context.Background(), func(context.Context, *sql.Tx) error { return nil })
	}()
	waitForQueueDepth(t, store, 1)
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := <-activeDone; !errors.Is(err, ErrClosed) {
		t.Fatalf("active Write() error = %v, want ErrClosed", err)
	}
	if err := <-pendingDone; !errors.Is(err, ErrClosed) {
		t.Fatalf("pending Write() error = %v, want ErrClosed", err)
	}
	if err := store.Write(context.Background(), func(context.Context, *sql.Tx) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("Write() after Close error = %v, want ErrClosed", err)
	}
}

func TestStoreHonorsSQLiteBusyTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "busy.db")
	store := openTestStore(t, StoreConfig{Path: path, BusyTimeout: 40 * time.Millisecond})
	defer store.Close()
	if err := store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `CREATE TABLE example (id TEXT PRIMARY KEY)`)
		return err
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}

	blocker, err := Open(context.Background(), path, time.Second, 1)
	if err != nil {
		t.Fatalf("open blocker: %v", err)
	}
	defer blocker.Close()
	blockerTx, err := blocker.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin blocker: %v", err)
	}
	if _, err := blockerTx.Exec(`INSERT INTO example(id) VALUES ('lock-holder')`); err != nil {
		t.Fatalf("acquire write lock: %v", err)
	}
	defer blockerTx.Rollback()

	started := time.Now()
	err = store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO example(id) VALUES ('blocked')`)
		return err
	})
	if err == nil {
		t.Fatal("write under external lock unexpectedly succeeded")
	}
	if elapsed := time.Since(started); elapsed < 20*time.Millisecond || elapsed > time.Second {
		t.Fatalf("busy timeout elapsed = %v, want bounded wait near configured timeout", elapsed)
	}
}

func openTestStore(t *testing.T, config StoreConfig) *Store {
	t.Helper()
	store, err := OpenStore(context.Background(), config)
	if err != nil {
		t.Fatalf("OpenStore() error = %v", err)
	}
	return store
}

func waitForQueueDepth(t *testing.T, store *Store, depth int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for len(store.writes) != depth && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if len(store.writes) != depth {
		t.Fatalf("write queue depth = %d, want %d", len(store.writes), depth)
	}
}
