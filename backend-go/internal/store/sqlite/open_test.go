package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenEnablesCompatibilityPragmas(t *testing.T) {
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "musicparty.db"), 5*time.Second, 1)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	for query, want := range map[string]string{
		"PRAGMA foreign_keys": "1",
		"PRAGMA journal_mode": "wal",
	} {
		var got string
		if err := database.QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("%s error = %v", query, err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", query, got, want)
		}
	}
}

func TestOpenReadOnlyUsesQueryOnlyConnections(t *testing.T) {
	path := filepath.Join(t.TempDir(), "musicparty.db")
	writer, err := Open(context.Background(), path, time.Second, 1)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if _, err := writer.Exec(`CREATE TABLE example (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create fixture schema: %v", err)
	}
	writer.Close()

	reader, err := OpenReadOnly(context.Background(), path, time.Second, 2)
	if err != nil {
		t.Fatalf("OpenReadOnly() error = %v", err)
	}
	defer reader.Close()
	var queryOnly int
	if err := reader.QueryRow("PRAGMA query_only").Scan(&queryOnly); err != nil {
		t.Fatalf("PRAGMA query_only error = %v", err)
	}
	if queryOnly != 1 {
		t.Fatalf("PRAGMA query_only = %d, want 1", queryOnly)
	}
	if _, err := reader.Exec(`INSERT INTO example(id) VALUES ('should-fail')`); err == nil {
		t.Fatal("read-only database unexpectedly accepted a write")
	}
}

func TestOpenUsesAbsoluteSQLiteFileURI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absolute.db")
	database, err := Open(context.Background(), path, time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE uri_probe (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create table through absolute SQLite URI: %v", err)
	}
}

func TestOpenImmutableRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "immutable.db")
	writer, err := Open(context.Background(), path, time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec(`CREATE TABLE example (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	writer.Close()

	reader, err := OpenImmutable(context.Background(), path, time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := reader.Exec(`INSERT INTO example(id) VALUES ('should-fail')`); err == nil {
		t.Fatal("immutable database unexpectedly accepted a write")
	}
}
