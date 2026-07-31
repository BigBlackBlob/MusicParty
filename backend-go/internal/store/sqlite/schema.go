package sqlite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SchemaSnapshot struct {
	Version     int            `json:"version"`
	SQLSHA256   string         `json:"sqlSha256"`
	ObjectCount int            `json:"objectCount"`
	TableCount  int            `json:"tableCount"`
	Objects     []SchemaObject `json:"objects"`
	Tables      []SchemaTable  `json:"tables"`
}

type SchemaObject struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	TableName string `json:"tableName"`
	SQL       string `json:"sql"`
}

type SchemaTable struct {
	Name        string             `json:"name"`
	Columns     []SchemaColumn     `json:"columns"`
	ForeignKeys []SchemaForeignKey `json:"foreignKeys"`
	Indexes     []SchemaIndex      `json:"indexes"`
}

type SchemaColumn struct {
	Position     int     `json:"position"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	NotNull      bool    `json:"notNull"`
	DefaultValue *string `json:"defaultValue"`
	PrimaryKey   int     `json:"primaryKey"`
}

type SchemaForeignKey struct {
	ID       int    `json:"id"`
	Sequence int    `json:"sequence"`
	Table    string `json:"table"`
	From     string `json:"from"`
	To       string `json:"to"`
	OnUpdate string `json:"onUpdate"`
	OnDelete string `json:"onDelete"`
	Match    string `json:"match"`
}

type SchemaIndex struct {
	Position int                 `json:"position"`
	Name     string              `json:"name"`
	Unique   bool                `json:"unique"`
	Origin   string              `json:"origin"`
	Partial  bool                `json:"partial"`
	Columns  []SchemaIndexColumn `json:"columns"`
}

type SchemaIndexColumn struct {
	Position  int     `json:"position"`
	ColumnID  int     `json:"columnId"`
	Name      *string `json:"name"`
	Desc      bool    `json:"desc"`
	Collation string  `json:"collation"`
	Key       bool    `json:"key"`
}

type SchemaDifference struct {
	Path  string `json:"path"`
	Issue string `json:"issue"`
}

func ReadSchemaSnapshot(path string) (SchemaSnapshot, error) {
	var snapshot SchemaSnapshot
	content, err := os.ReadFile(path)
	if err != nil {
		return snapshot, fmt.Errorf("read schema snapshot: %w", err)
	}
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return snapshot, fmt.Errorf("decode schema snapshot: %w", err)
	}
	return snapshot, nil
}

// ValidateSchemaCompatibility compares semantic structures required by Go.
// It intentionally does not compare raw CREATE statements or require foreign
// keys that older Java migrations could not add to existing SQLite tables.
func ValidateSchemaCompatibility(required, actual SchemaSnapshot) []SchemaDifference {
	differences := []SchemaDifference{}
	actualTables := make(map[string]SchemaTable, len(actual.Tables))
	for _, table := range actual.Tables {
		actualTables[table.Name] = table
	}
	requiredExplicitIndexes := map[string]struct{}{}
	for _, object := range required.Objects {
		if object.Type == "index" {
			requiredExplicitIndexes[object.Name] = struct{}{}
		}
		if object.Type == "view" || object.Type == "trigger" {
			if !hasSchemaObject(actual.Objects, object.Type, object.Name) {
				differences = append(differences, SchemaDifference{Path: object.Type + "." + object.Name, Issue: "missing required schema object"})
			}
		}
	}

	for _, requiredTable := range required.Tables {
		actualTable, ok := actualTables[requiredTable.Name]
		if !ok {
			differences = append(differences, SchemaDifference{Path: "table." + requiredTable.Name, Issue: "missing required table"})
			continue
		}
		actualColumns := make(map[string]SchemaColumn, len(actualTable.Columns))
		for _, column := range actualTable.Columns {
			actualColumns[column.Name] = column
		}
		for _, requiredColumn := range requiredTable.Columns {
			actualColumn, ok := actualColumns[requiredColumn.Name]
			path := "table." + requiredTable.Name + ".column." + requiredColumn.Name
			if !ok {
				differences = append(differences, SchemaDifference{Path: path, Issue: "missing required column"})
				continue
			}
			if !strings.EqualFold(requiredColumn.Type, actualColumn.Type) {
				differences = append(differences, SchemaDifference{Path: path, Issue: fmt.Sprintf("type is %s, require %s", actualColumn.Type, requiredColumn.Type)})
			}
			if requiredColumn.NotNull && !actualColumn.NotNull {
				differences = append(differences, SchemaDifference{Path: path, Issue: "column is nullable but must be NOT NULL"})
			}
			if requiredColumn.PrimaryKey != actualColumn.PrimaryKey {
				differences = append(differences, SchemaDifference{Path: path, Issue: fmt.Sprintf("primary-key position is %d, require %d", actualColumn.PrimaryKey, requiredColumn.PrimaryKey)})
			}
		}

		actualIndexes := make(map[string]SchemaIndex, len(actualTable.Indexes))
		for _, index := range actualTable.Indexes {
			actualIndexes[index.Name] = index
		}
		for _, requiredIndex := range requiredTable.Indexes {
			if _, explicit := requiredExplicitIndexes[requiredIndex.Name]; !explicit {
				continue
			}
			actualIndex, ok := actualIndexes[requiredIndex.Name]
			path := "table." + requiredTable.Name + ".index." + requiredIndex.Name
			if !ok {
				differences = append(differences, SchemaDifference{Path: path, Issue: "missing required index"})
				continue
			}
			if requiredIndex.Unique != actualIndex.Unique || requiredIndex.Partial != actualIndex.Partial {
				differences = append(differences, SchemaDifference{Path: path, Issue: "unique or partial-index flags differ"})
			}
			if !equalIndexKeyColumns(requiredIndex.Columns, actualIndex.Columns) {
				differences = append(differences, SchemaDifference{Path: path, Issue: "indexed columns, collation, or direction differ"})
			}
		}
	}
	sort.Slice(differences, func(i, j int) bool {
		if differences[i].Path == differences[j].Path {
			return differences[i].Issue < differences[j].Issue
		}
		return differences[i].Path < differences[j].Path
	})
	return differences
}

func VerifySchemaCompatibility(ctx context.Context, database *sql.DB, required SchemaSnapshot) ([]SchemaDifference, error) {
	actual, _, err := InspectSchema(ctx, database)
	if err != nil {
		return nil, err
	}
	return ValidateSchemaCompatibility(required, actual), nil
}

func hasSchemaObject(objects []SchemaObject, objectType, name string) bool {
	for _, object := range objects {
		if object.Type == objectType && object.Name == name {
			return true
		}
	}
	return false
}

func equalIndexKeyColumns(required, actual []SchemaIndexColumn) bool {
	requiredKeys, actualKeys := keyIndexColumns(required), keyIndexColumns(actual)
	if len(requiredKeys) != len(actualKeys) {
		return false
	}
	for index := range requiredKeys {
		left, right := requiredKeys[index], actualKeys[index]
		if left.ColumnID != right.ColumnID || left.Desc != right.Desc || !strings.EqualFold(left.Collation, right.Collation) {
			return false
		}
		if left.Name == nil || right.Name == nil {
			if left.Name != nil || right.Name != nil {
				return false
			}
		} else if *left.Name != *right.Name {
			return false
		}
	}
	return true
}

func keyIndexColumns(columns []SchemaIndexColumn) []SchemaIndexColumn {
	result := []SchemaIndexColumn{}
	for _, column := range columns {
		if column.Key {
			result = append(result, column)
		}
	}
	return result
}

func InspectSchema(ctx context.Context, database *sql.DB) (SchemaSnapshot, []byte, error) {
	snapshot := SchemaSnapshot{Version: 1, Objects: []SchemaObject{}, Tables: []SchemaTable{}}
	rows, err := database.QueryContext(ctx, `
		select type, name, tbl_name, sql
		from sqlite_master
		where name not like 'sqlite_%' and sql is not null
		order by case type when 'table' then 1 when 'index' then 2 when 'trigger' then 3 when 'view' then 4 else 5 end,
		         name collate binary
	`)
	if err != nil {
		return snapshot, nil, fmt.Errorf("query sqlite_master: %w", err)
	}
	for rows.Next() {
		var object SchemaObject
		if err := rows.Scan(&object.Type, &object.Name, &object.TableName, &object.SQL); err != nil {
			rows.Close()
			return snapshot, nil, fmt.Errorf("scan sqlite_master: %w", err)
		}
		snapshot.Objects = append(snapshot.Objects, object)
	}
	if err := rows.Close(); err != nil {
		return snapshot, nil, fmt.Errorf("close sqlite_master: %w", err)
	}

	for _, object := range snapshot.Objects {
		if object.Type != "table" {
			continue
		}
		table, err := inspectTable(ctx, database, object.Name)
		if err != nil {
			return snapshot, nil, err
		}
		snapshot.Tables = append(snapshot.Tables, table)
	}
	snapshot.ObjectCount = len(snapshot.Objects)
	snapshot.TableCount = len(snapshot.Tables)
	sqlBytes := RenderSchemaSQL(snapshot.Objects)
	digest := sha256.Sum256(sqlBytes)
	snapshot.SQLSHA256 = hex.EncodeToString(digest[:])
	return snapshot, sqlBytes, nil
}

func RenderSchemaSQL(objects []SchemaObject) []byte {
	var output bytes.Buffer
	output.WriteString("-- Generated from the frozen Java SQLite initializer. Do not edit by hand.\n")
	output.WriteString("PRAGMA foreign_keys = ON;\n\n")
	for _, object := range objects {
		output.WriteString(strings.TrimSpace(object.SQL))
		output.WriteString(";\n\n")
	}
	return output.Bytes()
}

func WriteSchemaSnapshot(outputDirectory string, snapshot SchemaSnapshot, schemaSQL []byte) error {
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return fmt.Errorf("create schema snapshot directory: %w", err)
	}
	jsonBuffer := &bytes.Buffer{}
	encoder := json.NewEncoder(jsonBuffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		return fmt.Errorf("encode schema snapshot: %w", err)
	}
	files := map[string][]byte{
		"schema.sql":    schemaSQL,
		"schema.json":   jsonBuffer.Bytes(),
		"schema.sha256": []byte(snapshot.SQLSHA256 + "  schema.sql\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(outputDirectory, name), content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

func inspectTable(ctx context.Context, database *sql.DB, name string) (SchemaTable, error) {
	table := SchemaTable{Name: name, Columns: []SchemaColumn{}, ForeignKeys: []SchemaForeignKey{}, Indexes: []SchemaIndex{}}
	quotedName := quoteIdentifier(name)

	rows, err := database.QueryContext(ctx, "PRAGMA table_info("+quotedName+")")
	if err != nil {
		return table, fmt.Errorf("inspect columns for %s: %w", name, err)
	}
	for rows.Next() {
		var column SchemaColumn
		var notNull int
		var defaultValue sql.NullString
		if err := rows.Scan(&column.Position, &column.Name, &column.Type, &notNull, &defaultValue, &column.PrimaryKey); err != nil {
			rows.Close()
			return table, fmt.Errorf("scan columns for %s: %w", name, err)
		}
		column.NotNull = notNull != 0
		if defaultValue.Valid {
			value := defaultValue.String
			column.DefaultValue = &value
		}
		table.Columns = append(table.Columns, column)
	}
	if err := rows.Close(); err != nil {
		return table, fmt.Errorf("close columns for %s: %w", name, err)
	}

	rows, err = database.QueryContext(ctx, "PRAGMA foreign_key_list("+quotedName+")")
	if err != nil {
		return table, fmt.Errorf("inspect foreign keys for %s: %w", name, err)
	}
	for rows.Next() {
		var foreignKey SchemaForeignKey
		if err := rows.Scan(&foreignKey.ID, &foreignKey.Sequence, &foreignKey.Table, &foreignKey.From, &foreignKey.To, &foreignKey.OnUpdate, &foreignKey.OnDelete, &foreignKey.Match); err != nil {
			rows.Close()
			return table, fmt.Errorf("scan foreign keys for %s: %w", name, err)
		}
		table.ForeignKeys = append(table.ForeignKeys, foreignKey)
	}
	if err := rows.Close(); err != nil {
		return table, fmt.Errorf("close foreign keys for %s: %w", name, err)
	}

	rows, err = database.QueryContext(ctx, "PRAGMA index_list("+quotedName+")")
	if err != nil {
		return table, fmt.Errorf("inspect indexes for %s: %w", name, err)
	}
	for rows.Next() {
		var index SchemaIndex
		var unique, partial int
		if err := rows.Scan(&index.Position, &index.Name, &unique, &index.Origin, &partial); err != nil {
			rows.Close()
			return table, fmt.Errorf("scan indexes for %s: %w", name, err)
		}
		index.Unique = unique != 0
		index.Partial = partial != 0
		index.Columns = []SchemaIndexColumn{}
		table.Indexes = append(table.Indexes, index)
	}
	if err := rows.Close(); err != nil {
		return table, fmt.Errorf("close indexes for %s: %w", name, err)
	}
	for indexNumber := range table.Indexes {
		index := &table.Indexes[indexNumber]
		indexRows, err := database.QueryContext(ctx, "PRAGMA index_xinfo("+quoteIdentifier(index.Name)+")")
		if err != nil {
			return table, fmt.Errorf("inspect index %s: %w", index.Name, err)
		}
		for indexRows.Next() {
			var column SchemaIndexColumn
			var columnName sql.NullString
			var desc, key int
			if err := indexRows.Scan(&column.Position, &column.ColumnID, &columnName, &desc, &column.Collation, &key); err != nil {
				indexRows.Close()
				return table, fmt.Errorf("scan index %s: %w", index.Name, err)
			}
			column.Desc = desc != 0
			column.Key = key != 0
			if columnName.Valid {
				value := columnName.String
				column.Name = &value
			}
			index.Columns = append(index.Columns, column)
		}
		if err := indexRows.Close(); err != nil {
			return table, fmt.Errorf("close index %s: %w", index.Name, err)
		}
	}
	return table, nil
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
