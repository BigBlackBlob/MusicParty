package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path string, busyTimeout time.Duration, maxConnections int) (*sql.DB, error) {
	return openDatabase(ctx, path, busyTimeout, maxConnections, false, false, false)
}

func OpenReadOnly(ctx context.Context, path string, busyTimeout time.Duration, maxConnections int) (*sql.DB, error) {
	return openDatabase(ctx, path, busyTimeout, maxConnections, true, true, false)
}

// OpenImmutable opens an offline database without creating WAL or shared-memory
// sidecars. The caller must guarantee that no process can modify the file.
func OpenImmutable(ctx context.Context, path string, busyTimeout time.Duration, maxConnections int) (*sql.DB, error) {
	return openDatabase(ctx, path, busyTimeout, maxConnections, true, true, true)
}

func openDatabase(ctx context.Context, path string, busyTimeout time.Duration, maxConnections int, readOnly, queryOnly, immutable bool) (*sql.DB, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite path: %w", err)
	}
	pragmas := []string{
		"foreign_keys(1)",
		"busy_timeout(" + strconv.FormatInt(busyTimeout.Milliseconds(), 10) + ")",
	}
	query := url.Values{"_pragma": pragmas}
	if readOnly {
		query.Set("mode", "ro")
		if immutable {
			query.Set("immutable", "1")
		}
		if queryOnly {
			query["_pragma"] = append(query["_pragma"], "query_only(1)")
		}
	} else {
		query["_pragma"] = append(query["_pragma"], "journal_mode(WAL)", "synchronous(NORMAL)")
	}
	dsnURL := &url.URL{Scheme: "file", RawQuery: query.Encode()}
	if filepath.VolumeName(absolutePath) != "" {
		// modernc SQLite expects file:C:/... on Windows and treats the drive
		// letter in file:///C:/... as an invalid URI authority.
		dsnURL.Opaque = filepath.ToSlash(absolutePath)
	} else {
		// Unix absolute paths require file:///... rather than file:/....
		dsnURL.Path = filepath.ToSlash(absolutePath)
	}
	dsn := dsnURL.String()
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open SQLite: %w", err)
	}
	if maxConnections < 1 {
		maxConnections = 1
	}
	database.SetMaxOpenConns(maxConnections)
	database.SetMaxIdleConns(maxConnections)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping SQLite: %w", err)
	}
	return database, nil
}
