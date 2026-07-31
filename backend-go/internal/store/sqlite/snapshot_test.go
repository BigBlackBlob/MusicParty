package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotDatabaseCreatesConsistentCopyWithoutOverwriting(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.db")
	destinationPath := filepath.Join(directory, "copy", "snapshot.db")
	source, err := Open(ctx, sourcePath, time.Second, 1)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	if _, err := source.ExecContext(ctx, `create table example(id text primary key); insert into example(id) values ('copied')`); err != nil {
		source.Close()
		t.Fatalf("seed source: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close stopped source: %v", err)
	}
	if err := SnapshotDatabase(ctx, sourcePath, destinationPath, time.Second); err != nil {
		t.Fatalf("SnapshotDatabase(): %v", err)
	}
	copy, err := OpenReadOnly(ctx, destinationPath, time.Second, 1)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer copy.Close()
	var copiedID string
	if err := copy.QueryRowContext(ctx, `select id from example`).Scan(&copiedID); err != nil || copiedID != "copied" {
		t.Fatalf("snapshot row = %q, %v", copiedID, err)
	}
	if err := SnapshotDatabase(ctx, sourcePath, destinationPath, time.Second); !errors.Is(err, ErrSnapshotDestinationExists) {
		t.Fatalf("second SnapshotDatabase() error = %v, want ErrSnapshotDestinationExists", err)
	}
}
