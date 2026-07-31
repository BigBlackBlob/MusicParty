package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckHealthyDatabase(t *testing.T) {
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "musicparty.db"), time.Second, 1)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	if _, err := database.Exec(`CREATE TABLE parent (id INTEGER PRIMARY KEY); CREATE TABLE child (id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent(id));`); err != nil {
		t.Fatalf("create fixture schema: %v", err)
	}
	result, err := Check(context.Background(), database)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.OK() || result.ApplicationTables != 2 {
		t.Fatalf("unexpected check result: %+v", result)
	}
}
