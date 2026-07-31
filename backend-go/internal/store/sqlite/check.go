package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type ForeignKeyViolation struct {
	Table      string `json:"table"`
	RowID      int64  `json:"rowId"`
	Parent     string `json:"parent"`
	Constraint int64  `json:"constraint"`
}

type CheckResult struct {
	Integrity            []string              `json:"integrity"`
	ForeignKeyViolations []ForeignKeyViolation `json:"foreignKeyViolations"`
	ApplicationTables    int                   `json:"applicationTables"`
}

func Check(ctx context.Context, database *sql.DB) (CheckResult, error) {
	result := CheckResult{Integrity: []string{}, ForeignKeyViolations: []ForeignKeyViolation{}}
	rows, err := database.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return result, fmt.Errorf("integrity_check: %w", err)
	}
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan integrity_check: %w", err)
		}
		result.Integrity = append(result.Integrity, message)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close integrity_check: %w", err)
	}

	rows, err = database.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return result, fmt.Errorf("foreign_key_check: %w", err)
	}
	for rows.Next() {
		var violation ForeignKeyViolation
		if err := rows.Scan(&violation.Table, &violation.RowID, &violation.Parent, &violation.Constraint); err != nil {
			rows.Close()
			return result, fmt.Errorf("scan foreign_key_check: %w", err)
		}
		result.ForeignKeyViolations = append(result.ForeignKeyViolations, violation)
	}
	if err := rows.Close(); err != nil {
		return result, fmt.Errorf("close foreign_key_check: %w", err)
	}

	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&result.ApplicationTables); err != nil {
		return result, fmt.Errorf("count application tables: %w", err)
	}
	return result, nil
}

func (r CheckResult) OK() bool {
	return len(r.Integrity) == 1 && r.Integrity[0] == "ok" && len(r.ForeignKeyViolations) == 0
}
