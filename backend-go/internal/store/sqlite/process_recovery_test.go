package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSQLiteAbruptProcessRollback(t *testing.T) {
	if os.Getenv("MUSICPARTY_SQLITE_CRASH_HELPER") == "1" {
		path := os.Getenv("MUSICPARTY_SQLITE_CRASH_DB")
		store, err := OpenStore(context.Background(), StoreConfig{Path: path, BusyTimeout: time.Second})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(90)
		}
		_ = store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, `insert into crash_test(id) values ('uncommitted')`); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(90)
			}
			os.Exit(91)
			return nil
		})
		os.Exit(90)
	}

	path := filepath.Join(t.TempDir(), "crash.db")
	store := openTestStore(t, StoreConfig{Path: path, BusyTimeout: time.Second})
	if err := store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create table crash_test(id text primary key)`)
		return err
	}); err != nil {
		store.Close()
		t.Fatalf("create crash fixture: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close parent store: %v", err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestSQLiteAbruptProcessRollback$")
	command.Env = append(os.Environ(), "MUSICPARTY_SQLITE_CRASH_HELPER=1", "MUSICPARTY_SQLITE_CRASH_DB="+path)
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 91 {
		t.Fatalf("crash helper error = %v, want exit code 91", err)
	}

	recovered, err := OpenStore(context.Background(), StoreConfig{Path: path, BusyTimeout: time.Second})
	if err != nil {
		t.Fatalf("reopen after abrupt process exit: %v", err)
	}
	defer recovered.Close()
	var count int
	if err := recovered.Reader().QueryRowContext(context.Background(), `select count(*) from crash_test`).Scan(&count); err != nil {
		t.Fatalf("read after abrupt process exit: %v", err)
	}
	if count != 0 {
		t.Fatalf("uncommitted rows after abrupt process exit = %d, want 0", count)
	}
	check, err := Check(context.Background(), recovered.Reader())
	if err != nil || !check.OK() {
		t.Fatalf("database after abrupt process exit = %+v, %v", check, err)
	}
}

func TestStoreSerializesConcurrentWritersWithoutLostUpdates(t *testing.T) {
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "concurrent.db"), WriteQueueCapacity: 100})
	defer store.Close()
	ctx := context.Background()
	if err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create table counter(value integer not null); insert into counter(value) values (0)`)
		return err
	}); err != nil {
		t.Fatalf("create counter: %v", err)
	}

	const writers = 50
	var waitGroup sync.WaitGroup
	errorsChannel := make(chan error, writers)
	for index := 0; index < writers; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			errorsChannel <- store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `update counter set value = value + 1`)
				return err
			})
		}()
	}
	waitGroup.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		if err != nil {
			t.Fatalf("concurrent Write() error = %v", err)
		}
	}
	var value int
	if err := store.Reader().QueryRowContext(ctx, `select value from counter`).Scan(&value); err != nil {
		t.Fatalf("read counter: %v", err)
	}
	if value != writers {
		t.Fatalf("counter = %d, want %d", value, writers)
	}
}

func TestStoreRollsBackPanickingWrite(t *testing.T) {
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "panic.db")})
	defer store.Close()
	ctx := context.Background()
	if err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create table example(id text primary key)`)
		return err
	}); err != nil {
		t.Fatalf("create table: %v", err)
	}
	err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `insert into example(id) values ('panic')`); err != nil {
			return err
		}
		panic("fixture panic")
	})
	if err == nil || !strings.Contains(err.Error(), "fixture panic") {
		t.Fatalf("panicking Write() error = %v", err)
	}
	var count int
	if err := store.Reader().QueryRowContext(ctx, `select count(*) from example`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rows after panic = %d, %v", count, err)
	}
}
