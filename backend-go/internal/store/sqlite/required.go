package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
)

//go:embed required_schema.sql
var requiredSchemaSQL string

//go:embed required_schema.json
var requiredSchemaJSON []byte

type SchemaCompatibilityError struct {
	Differences []SchemaDifference
}

func (err *SchemaCompatibilityError) Error() string {
	if len(err.Differences) == 0 {
		return "SQLite schema is incompatible"
	}
	return fmt.Sprintf("SQLite schema is incompatible: %s: %s", err.Differences[0].Path, err.Differences[0].Issue)
}

func RequiredSchema() (SchemaSnapshot, error) {
	var snapshot SchemaSnapshot
	if err := json.Unmarshal(requiredSchemaJSON, &snapshot); err != nil {
		return snapshot, fmt.Errorf("decode embedded required schema: %w", err)
	}
	return snapshot, nil
}

// EnsureCompatibleSchema initializes only a truly empty database when allowed,
// then verifies the semantic schema required by every typed repository. It
// never migrates or rewrites a non-empty database.
func EnsureCompatibleSchema(ctx context.Context, store *Store, initializeEmpty bool) error {
	var tableCount int
	if err := store.reader.QueryRowContext(ctx, `select count(*) from sqlite_master where type = 'table' and name not like 'sqlite_%'`).Scan(&tableCount); err != nil {
		return fmt.Errorf("count SQLite application tables: %w", err)
	}
	if tableCount == 0 {
		if !initializeEmpty {
			return errors.New("SQLite database is empty and DB_INIT_SCHEMA is disabled")
		}
		if err := store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, requiredSchemaSQL)
			return err
		}); err != nil {
			return fmt.Errorf("initialize frozen SQLite schema: %w", err)
		}
	}
	required, err := RequiredSchema()
	if err != nil {
		return err
	}
	differences, err := VerifySchemaCompatibility(ctx, store.reader, required)
	if err != nil {
		return fmt.Errorf("inspect SQLite schema compatibility: %w", err)
	}
	if len(differences) > 0 {
		return &SchemaCompatibilityError{Differences: differences}
	}
	return nil
}
