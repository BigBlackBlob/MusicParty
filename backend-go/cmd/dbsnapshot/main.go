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
	source := flag.String("source", "", "path to the source SQLite database (opened read-only)")
	destination := flag.String("destination", "", "path for a new consistent SQLite snapshot")
	busyTimeout := flag.Duration("busy-timeout", 5*time.Second, "SQLite busy timeout")
	flag.Parse()
	if *source == "" || *destination == "" {
		fmt.Fprintln(os.Stderr, "-source and -destination are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := storesqlite.SnapshotDatabase(ctx, *source, *destination, *busyTimeout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
