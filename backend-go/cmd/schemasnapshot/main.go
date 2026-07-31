package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

func main() {
	databasePath := flag.String("db", "", "path to a Java-initialized SQLite database")
	outputDirectory := flag.String("out", "", "directory for schema.sql, schema.json, and schema.sha256")
	flag.Parse()
	if *databasePath == "" || *outputDirectory == "" {
		fmt.Fprintln(os.Stderr, "-db and -out are required")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := storesqlite.OpenReadOnly(ctx, *databasePath, 5*time.Second, 1)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer database.Close()

	snapshot, schemaSQL, err := storesqlite.InspectSchema(ctx, database)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := storesqlite.WriteSchemaSnapshot(*outputDirectory, snapshot, schemaSQL); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d tables and %d schema objects; sha256=%s\n", snapshot.TableCount, snapshot.ObjectCount, snapshot.SQLSHA256)
}
