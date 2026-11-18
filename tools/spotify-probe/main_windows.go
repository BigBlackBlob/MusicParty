//go:build windows

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	compatible    = iota
	notCompatible = iota
	inconclusive  = iota
)

const (
	credentialFile = "credentials.json"
	checkTimeout   = 30 * time.Second
	minimumSeconds = 5
	minimumBitrate = 96
)

// tokenFile is the Windows-side credential format: raw OAuth tokens saved after
// the browser consent flow. The Linux check step exchanges these for stored
// go-librespot credentials and writes them back as credentials.json.
type tokenFile struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	SavedAt      string `json:"saved_at"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 || (args[0] != "login" && args[0] != "check") {
		fmt.Fprintln(os.Stderr, "Usage: spotify-probe login|check --engine go --credential-dir <path> [--track <id>]")
		return inconclusive
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	engine := flags.String("engine", "go", "Playback engine (go)")
	dir := flags.String("credential-dir", "", "Directory for credentials.json")
	var track string
	if command == "check" {
		flags.StringVar(&track, "track", "", "Spotify track URI or bare track ID (unused on Windows)")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return compatible
		}
		if command == "check" {
			fmt.Println("INCONCLUSIVE")
		}
		return inconclusive
	}
	_ = track

	switch {
	case flags.NArg() != 0:
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		return inconclusive
	case *engine != "go":
		fmt.Fprintln(os.Stderr, "--engine must be go")
		return inconclusive
	case *dir == "":
		fmt.Fprintln(os.Stderr, "--credential-dir is required")
		return inconclusive
	}

	if command == "check" {
		fmt.Println("INCONCLUSIVE")
		fmt.Fprintln(os.Stderr, "check is not supported on Windows; copy credentials.json to the Linux server and run check there")
		return inconclusive
	}

	// login: OAuth only, no go-librespot imports needed.
	clientID := strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_ID"))
	if clientID == "" {
		fmt.Fprintln(os.Stderr, "OAuth setup failed: set SPOTIFY_CLIENT_ID to your Spotify app's client ID")
		return inconclusive
	}
	tokens, err := authorize(context.Background(), nil, clientID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return inconclusive
	}
	path := filepath.Join(*dir, credentialFile)
	if err := saveTokens(path, tokenFile{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		SavedAt:      time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		fmt.Fprintln(os.Stderr, "Credential save failed:", err)
		return inconclusive
	}
	fmt.Printf("Login successful. Tokens saved to %s.\n", path)
	fmt.Println("Copy that file to your Linux server, then run: go run . check --engine go --credential-dir <dir> --track <id>")
	return compatible
}

func saveTokens(path string, t tokenFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".credentials-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := json.NewEncoder(file).Encode(t); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func safeError(err error, secrets ...string) string {
	message := err.Error()
	var requestErr *url.Error
	if errors.As(err, &requestErr) && requestErr.URL != "" {
		message = strings.ReplaceAll(message, requestErr.URL, "[request URL]")
	}
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	return message
}

func randomBytes(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}
