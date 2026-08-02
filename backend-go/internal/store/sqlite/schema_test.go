package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestContractSchemaIsSelfConsistentAndReproducible(t *testing.T) {
	contractDirectory := contractSchemaDirectory(t)
	schemaSQL, err := os.ReadFile(filepath.Join(contractDirectory, "schema.sql"))
	if err != nil {
		t.Fatalf("read contract schema: %v", err)
	}
	var contractSnapshot SchemaSnapshot
	snapshotJSON, err := os.ReadFile(filepath.Join(contractDirectory, "schema.json"))
	if err != nil {
		t.Fatalf("read contract snapshot: %v", err)
	}
	if err := json.Unmarshal(snapshotJSON, &contractSnapshot); err != nil {
		t.Fatalf("decode contract snapshot: %v", err)
	}
	digest := sha256.Sum256(schemaSQL)
	wantHash := hex.EncodeToString(digest[:])
	if contractSnapshot.SQLSHA256 != wantHash {
		t.Fatalf("snapshot hash = %s, schema.sql hash = %s", contractSnapshot.SQLSHA256, wantHash)
	}
	hashFile, err := os.ReadFile(filepath.Join(contractDirectory, "schema.sha256"))
	if err != nil {
		t.Fatalf("read hash file: %v", err)
	}
	if fields := strings.Fields(string(hashFile)); len(fields) != 2 || fields[0] != wantHash || fields[1] != "schema.sql" {
		t.Fatalf("unexpected schema.sha256 content: %q", hashFile)
	}
	if contractSnapshot.TableCount != 23 {
		t.Fatalf("table count = %d, want 23", contractSnapshot.TableCount)
	}

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "snapshot.db"), time.Second, 1)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	if _, err := database.ExecContext(context.Background(), string(schemaSQL)); err != nil {
		t.Fatalf("apply contract schema: %v", err)
	}
	regenerated, regeneratedSQL, err := InspectSchema(context.Background(), database)
	if err != nil {
		t.Fatalf("InspectSchema() error = %v", err)
	}
	if regenerated.TableCount != 23 || regenerated.ObjectCount != contractSnapshot.ObjectCount {
		t.Fatalf("regenerated counts = %d tables/%d objects, want %d/%d", regenerated.TableCount, regenerated.ObjectCount, contractSnapshot.TableCount, contractSnapshot.ObjectCount)
	}
	if string(regeneratedSQL) != string(schemaSQL) {
		t.Fatal("schema.sql is not reproducible after applying it to a fresh SQLite database")
	}
}

func TestLegacyUpgradedSchemaIsSemanticallyCompatible(t *testing.T) {
	contractDirectory := contractSchemaDirectory(t)
	required, err := ReadSchemaSnapshot(filepath.Join(contractDirectory, "schema.json"))
	if err != nil {
		t.Fatalf("read required schema: %v", err)
	}
	legacyDirectory := filepath.Join(contractDirectory, "fixtures", "legacy-upgraded")
	legacy, err := ReadSchemaSnapshot(filepath.Join(legacyDirectory, "schema.json"))
	if err != nil {
		t.Fatalf("read legacy schema: %v", err)
	}
	if legacy.SQLSHA256 == required.SQLSHA256 {
		t.Fatal("legacy fixture unexpectedly has the canonical fresh-schema hash")
	}
	if differences := ValidateSchemaCompatibility(required, legacy); len(differences) != 0 {
		t.Fatalf("legacy upgraded schema differences: %+v", differences)
	}

	legacySQL, err := os.ReadFile(filepath.Join(legacyDirectory, "schema.sql"))
	if err != nil {
		t.Fatalf("read legacy schema SQL: %v", err)
	}
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "legacy.db"), time.Second, 1)
	if err != nil {
		t.Fatalf("open legacy fixture: %v", err)
	}
	defer database.Close()
	if _, err := database.ExecContext(context.Background(), string(legacySQL)); err != nil {
		t.Fatalf("apply legacy schema: %v", err)
	}
	differences, err := VerifySchemaCompatibility(context.Background(), database, required)
	if err != nil {
		t.Fatalf("VerifySchemaCompatibility(): %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("applied legacy schema differences: %+v", differences)
	}
}

func TestSchemaCompatibilityReportsMissingRequiredColumn(t *testing.T) {
	required, err := ReadSchemaSnapshot(filepath.Join(contractSchemaDirectory(t), "schema.json"))
	if err != nil {
		t.Fatalf("read required schema: %v", err)
	}
	actual := required
	actual.Tables = append([]SchemaTable(nil), required.Tables...)
	for tableIndex := range actual.Tables {
		if actual.Tables[tableIndex].Name != "room" {
			continue
		}
		columns := append([]SchemaColumn(nil), actual.Tables[tableIndex].Columns...)
		for columnIndex, column := range columns {
			if column.Name == "visibility" {
				columns = append(columns[:columnIndex], columns[columnIndex+1:]...)
				break
			}
		}
		actual.Tables[tableIndex].Columns = columns
	}
	differences := ValidateSchemaCompatibility(required, actual)
	if len(differences) != 1 || differences[0].Path != "table.room.column.visibility" {
		t.Fatalf("schema differences = %+v, want missing room.visibility", differences)
	}
}

func TestSchemaCompatibilityIgnoresPhysicalIndexColumnIDs(t *testing.T) {
	required := inspectSchemaFixture(t, `
		CREATE TABLE item (key TEXT, extra_b TEXT, extra_a TEXT);
		CREATE INDEX item_key_idx ON item(key COLLATE NOCASE DESC);
	`)
	actual := inspectSchemaFixture(t, `
		CREATE TABLE item (extra_a TEXT, key TEXT, extra_b TEXT);
		CREATE INDEX item_key_idx ON item(key COLLATE NOCASE DESC);
	`)
	requiredIndex := schemaFixtureIndex(t, required, "item", "item_key_idx")
	actualIndex := schemaFixtureIndex(t, actual, "item", "item_key_idx")
	requiredKeys, actualKeys := keyIndexColumns(requiredIndex.Columns), keyIndexColumns(actualIndex.Columns)
	if len(requiredKeys) != 1 || len(actualKeys) != 1 {
		t.Fatalf("indexed key columns = %d and %d, want one each", len(requiredKeys), len(actualKeys))
	}
	if requiredKeys[0].ColumnID == actualKeys[0].ColumnID {
		t.Fatalf("indexed key column IDs = %d and %d, want physical-order difference", requiredKeys[0].ColumnID, actualKeys[0].ColumnID)
	}
	if differences := ValidateSchemaCompatibility(required, actual); len(differences) != 0 {
		t.Fatalf("semantically equivalent schemas differ: %+v", differences)
	}
}

func TestSchemaCompatibilityRejectsChangedIndexSemantics(t *testing.T) {
	required := inspectSchemaFixture(t, `
		CREATE TABLE item (first TEXT, second TEXT, payload TEXT);
		CREATE INDEX item_idx ON item(first COLLATE BINARY ASC, second COLLATE NOCASE DESC);
	`)
	tests := map[string]string{
		"key order": `
			CREATE TABLE item (first TEXT, second TEXT, payload TEXT);
			CREATE INDEX item_idx ON item(second COLLATE NOCASE DESC, first COLLATE BINARY ASC);
		`,
		"desc direction": `
			CREATE TABLE item (first TEXT, second TEXT, payload TEXT);
			CREATE INDEX item_idx ON item(first COLLATE BINARY DESC, second COLLATE NOCASE DESC);
		`,
		"collation": `
			CREATE TABLE item (first TEXT, second TEXT, payload TEXT);
			CREATE INDEX item_idx ON item(first COLLATE NOCASE ASC, second COLLATE NOCASE DESC);
		`,
	}
	for name, schemaSQL := range tests {
		t.Run(name, func(t *testing.T) {
			actual := inspectSchemaFixture(t, schemaSQL)
			differences := ValidateSchemaCompatibility(required, actual)
			if len(differences) != 1 || differences[0].Path != "table.item.index.item_idx" {
				t.Fatalf("schema differences = %+v, want indexed-column difference", differences)
			}
		})
	}
}

func inspectSchemaFixture(t *testing.T, schemaSQL string) SchemaSnapshot {
	t.Helper()
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "schema.db"), time.Second, 1)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	if _, err := database.ExecContext(context.Background(), schemaSQL); err != nil {
		t.Fatalf("create fixture schema: %v", err)
	}
	snapshot, _, err := InspectSchema(context.Background(), database)
	if err != nil {
		t.Fatalf("InspectSchema() error = %v", err)
	}
	return snapshot
}

func schemaFixtureIndex(t *testing.T, snapshot SchemaSnapshot, tableName, indexName string) SchemaIndex {
	t.Helper()
	for _, table := range snapshot.Tables {
		if table.Name != tableName {
			continue
		}
		for _, index := range table.Indexes {
			if index.Name == indexName {
				return index
			}
		}
	}
	t.Fatalf("index %s.%s not found", tableName, indexName)
	return SchemaIndex{}
}

func contractSchemaDirectory(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "..", "contracts", "db"))
}
