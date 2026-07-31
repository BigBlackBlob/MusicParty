package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

func main() {
	databasePath := flag.String("db", "", "path to a SQLite database copy")
	busyTimeout := flag.Duration("busy-timeout", 5*time.Second, "SQLite busy timeout")
	flag.Parse()
	if *databasePath == "" {
		fmt.Fprintln(os.Stderr, "-db is required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := storesqlite.OpenImmutable(ctx, *databasePath, *busyTimeout, 1)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer database.Close()
	result, err := storesqlite.Check(ctx, database)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !result.OK() {
		os.Exit(1)
	}
}
