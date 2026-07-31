package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmbeddedRequiredSchemaMatchesContractAssets(t *testing.T) {
	contractDirectory := contractSchemaDirectory(t)
	contractSQL, err := os.ReadFile(filepath.Join(contractDirectory, "schema.sql"))
	if err != nil {
		t.Fatalf("read contract SQL: %v", err)
	}
	contractJSON, err := os.ReadFile(filepath.Join(contractDirectory, "schema.json"))
	if err != nil {
		t.Fatalf("read contract JSON: %v", err)
	}
	if string(contractSQL) != requiredSchemaSQL {
		t.Fatal("embedded required_schema.sql differs from contracts/db/schema.sql")
	}
	if string(contractJSON) != string(requiredSchemaJSON) {
		t.Fatal("embedded required_schema.json differs from contracts/db/schema.json")
	}
}

func TestEnsureCompatibleSchemaInitializesOnlyEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "empty.db")})
	defer store.Close()
	if err := EnsureCompatibleSchema(ctx, store, true); err != nil {
		t.Fatalf("EnsureCompatibleSchema(empty): %v", err)
	}
	check, err := Check(ctx, store.Reader())
	if err != nil || !check.OK() || check.ApplicationTables != 23 {
		t.Fatalf("initialized database = %+v, %v", check, err)
	}
	if err := EnsureCompatibleSchema(ctx, store, true); err != nil {
		t.Fatalf("EnsureCompatibleSchema(idempotent): %v", err)
	}
}

func TestEnsureCompatibleSchemaRejectsEmptyWhenInitializationDisabled(t *testing.T) {
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "disabled.db")})
	defer store.Close()
	err := EnsureCompatibleSchema(context.Background(), store, false)
	if err == nil || !strings.Contains(err.Error(), "DB_INIT_SCHEMA is disabled") {
		t.Fatalf("EnsureCompatibleSchema() error = %v", err)
	}
}

func TestEnsureCompatibleSchemaRejectsNonEmptyIncompatibleDatabase(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "incompatible.db")})
	defer store.Close()
	if _, err := store.writer.ExecContext(ctx, `create table unrelated(id text primary key)`); err != nil {
		t.Fatalf("create incompatible schema: %v", err)
	}
	err := EnsureCompatibleSchema(ctx, store, true)
	var compatibilityError *SchemaCompatibilityError
	if !errors.As(err, &compatibilityError) || len(compatibilityError.Differences) == 0 {
		t.Fatalf("EnsureCompatibleSchema() error = %v, want SchemaCompatibilityError", err)
	}
	var unrelatedCount int
	if err := store.Reader().QueryRowContext(ctx, `select count(*) from sqlite_master where type = 'table' and name = 'unrelated'`).Scan(&unrelatedCount); err != nil || unrelatedCount != 1 {
		t.Fatalf("non-empty database was modified: count=%d err=%v", unrelatedCount, err)
	}
}

func TestEnsureCompatibleSchemaAcceptsLegacyUpgradedFixture(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "legacy.db"), BusyTimeout: time.Second})
	defer store.Close()
	legacySQL, err := os.ReadFile(filepath.Join(contractSchemaDirectory(t), "fixtures", "legacy-upgraded", "schema.sql"))
	if err != nil {
		t.Fatalf("read legacy schema: %v", err)
	}
	if _, err := store.writer.ExecContext(ctx, string(legacySQL)); err != nil {
		t.Fatalf("apply legacy schema: %v", err)
	}
	if err := EnsureCompatibleSchema(ctx, store, false); err != nil {
		t.Fatalf("EnsureCompatibleSchema(legacy): %v", err)
	}
}
