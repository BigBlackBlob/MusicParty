package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var ErrSnapshotDestinationExists = errors.New("SQLite snapshot destination already exists")

// SnapshotDatabase copies a stopped SQLite database and any WAL sidecars into
// writable temporary storage, then produces a compact snapshot with VACUUM
// INTO. Callers must ensure the source has no active writer.
func SnapshotDatabase(ctx context.Context, sourcePath, destinationPath string, busyTimeout time.Duration) error {
	sourceAbsolute, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve SQLite snapshot source: %w", err)
	}
	destinationAbsolute, err := filepath.Abs(destinationPath)
	if err != nil {
		return fmt.Errorf("resolve SQLite snapshot destination: %w", err)
	}
	if filepath.Clean(sourceAbsolute) == filepath.Clean(destinationAbsolute) {
		return errors.New("SQLite snapshot source and destination must differ")
	}
	if _, err := os.Stat(destinationAbsolute); err == nil {
		return ErrSnapshotDestinationExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect SQLite snapshot destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destinationAbsolute), 0o755); err != nil {
		return fmt.Errorf("create SQLite snapshot directory: %w", err)
	}

	temporaryDirectory, err := os.MkdirTemp("", "musicparty-snapshot-")
	if err != nil {
		return fmt.Errorf("create SQLite snapshot workspace: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)
	temporarySource := filepath.Join(temporaryDirectory, "source.db")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := copyIfPresent(sourceAbsolute+suffix, temporarySource+suffix, suffix == ""); err != nil {
			return err
		}
	}

	source, err := Open(ctx, temporarySource, busyTimeout, 1)
	if err != nil {
		return fmt.Errorf("open copied SQLite snapshot source: %w", err)
	}
	defer source.Close()
	if _, err := source.ExecContext(ctx, "VACUUM INTO ?", destinationAbsolute); err != nil {
		return fmt.Errorf("create SQLite snapshot: %w", err)
	}
	return nil
}

func copyIfPresent(sourcePath, destinationPath string, required bool) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open SQLite snapshot source file: %w", err)
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create SQLite snapshot workspace file: %w", err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		return fmt.Errorf("copy SQLite snapshot source file: %w", err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close SQLite snapshot workspace file: %w", err)
	}
	return nil
}
