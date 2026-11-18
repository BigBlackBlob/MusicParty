//go:build !windows

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/ap"
	"github.com/devgianlu/go-librespot/audio"
	"github.com/devgianlu/go-librespot/player"
	devicespb "github.com/devgianlu/go-librespot/proto/spotify/connectstate/devices"
	"github.com/devgianlu/go-librespot/session"
)

const (
	compatible = iota
	notCompatible
	inconclusive
)

const (
	credentialFile = "credentials.json"
	checkTimeout   = 30 * time.Second
	minimumSeconds = 5
	minimumBitrate = 96
)

// Keep the same device ID when the credentials move from Windows to Linux.
// Data is the reusable access-point credential returned by go-librespot.
type credentials struct {
	DeviceID string `json:"device_id"`
	Username string `json:"username"`
	Data     []byte `json:"data"`
}

var (
	trackIDPattern = regexp.MustCompile(`^[0-9A-Za-z]{22}$`)
	countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)
)

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
		flags.StringVar(&track, "track", "", "Spotify track URI or bare track ID")
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

	ctx := context.Background()
	path := filepath.Join(*dir, credentialFile)
	var err error
	switch {
	case flags.NArg() != 0:
		err = errors.New("unexpected positional arguments")
	case *engine != "go":
		err = errors.New("--engine must be go")
	case *dir == "":
		err = errors.New("--credential-dir is required")
	case command == "login":
		err = login(ctx, path)
	default:
		err = check(ctx, path, track)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if command == "login" {
		if err != nil {
			return inconclusive
		}
		fmt.Printf("Login successful. Credentials saved to %s.\n", path)
		return compatible
	}
	switch {
	case err == nil:
		fmt.Println("COMPATIBLE")
		return compatible
	case isKeyDenial(err):
		fmt.Println("NOT COMPATIBLE")
		return notCompatible
	default:
		fmt.Println("INCONCLUSIVE")
		return inconclusive
	}
}

func login(ctx context.Context, path string) error {
	clientID := strings.TrimSpace(os.Getenv("SPOTIFY_CLIENT_ID"))
	if clientID == "" {
		return errors.New("OAuth setup failed: set SPOTIFY_CLIENT_ID to your Spotify app's client ID")
	}

	client := librespot.NewHttpClient()
	tokens, err := authorize(ctx, client, clientID)
	if err != nil {
		return err
	}
	deviceID := hex.EncodeToString(randomBytes(20))
	sess, err := connect(ctx, client, deviceID, session.SpotifyTokenCredentials{
		Username: tokens.Username,
		Token:    tokens.AccessToken,
	})
	if err != nil {
		return fmt.Errorf("Session construction failed: %s", safeError(err, tokens.AccessToken, tokens.RefreshToken))
	}
	defer sess.Close()

	if err := saveCredentials(path, sessionCredentials(sess, deviceID)); err != nil {
		return fmt.Errorf("Credential save failed: %w", err)
	}
	return nil
}

func check(ctx context.Context, path, track string) error {
	stored, err := loadCredentials(path)
	if err != nil {
		return fmt.Errorf("Credential load failed: %w", err)
	}
	id, err := parseTrack(track)
	if err != nil {
		return fmt.Errorf("Track load failed: %w", err)
	}

	client := librespot.NewHttpClient()
	secret := base64.StdEncoding.EncodeToString(stored.Data)
	sess, err := connect(ctx, client, stored.DeviceID, session.StoredCredentials{
		Username: stored.Username,
		Data:     stored.Data,
	})
	if err != nil {
		return fmt.Errorf("Session construction failed: %s", safeError(err, secret))
	}
	defer sess.Close()

	// Check for rotation even when loading or decoding the track fails.
	probeErr := probeTrack(ctx, client, sess, id)
	updated := sessionCredentials(sess, stored.DeviceID)
	if len(updated.Data) != 0 && (updated.Username != stored.Username || !bytes.Equal(updated.Data, stored.Data)) {
		if err := saveCredentials(path, updated); err != nil {
			fmt.Fprintf(os.Stderr, "Credential rotation failed: %v\n", err)
			if probeErr == nil {
				return errors.New("Could not save the updated Spotify credentials")
			}
		}
	}
	if probeErr != nil && !isKeyDenial(probeErr) {
		return errors.New(safeError(probeErr, secret, base64.StdEncoding.EncodeToString(updated.Data)))
	}
	return probeErr
}

func connect(ctx context.Context, client *http.Client, deviceID string, auth any) (*session.Session, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	return session.NewSessionFromOptions(ctx, &session.Options{
		Log:         &librespot.NullLogger{},
		DeviceType:  devicespb.DeviceType_COMPUTER,
		DeviceId:    deviceID,
		Credentials: auth,
		Client:      client,
	})
}

func parseTrack(track string) (librespot.SpotifyId, error) {
	bare := strings.TrimPrefix(track, "spotify:track:")
	if !trackIDPattern.MatchString(bare) {
		return librespot.SpotifyId{}, errors.New("--track must be a 22-character Spotify track ID or spotify:track: URI")
	}
	// The upstream parser calls big.Int.FillBytes with a 16-byte buffer.
	if value, ok := new(big.Int).SetString(bare, 62); !ok || value.BitLen() > 128 {
		return librespot.SpotifyId{}, errors.New("Spotify track ID is out of range")
	}
	id, err := librespot.SpotifyIdFromBase62(librespot.SpotifyIdTypeTrack, bare)
	if err != nil {
		return librespot.SpotifyId{}, err
	}
	return *id, nil
}

func probeTrack(ctx context.Context, client *http.Client, sess *session.Session, id librespot.SpotifyId) error {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- loadAndDecode(ctx, client, sess, id) }()
	select {
	case err := <-done:
		if ctx.Err() == nil {
			return err
		}
	case <-ctx.Done():
	}
	return fmt.Errorf("Track check timed out after %s", checkTimeout)
}

func loadAndDecode(ctx context.Context, client *http.Client, sess *session.Session, id librespot.SpotifyId) error {
	var country string
	select {
	case pkt, ok := <-sess.Accesspoint().Receive(ap.PacketTypeCountryCode):
		if !ok || !countryPattern.Match(pkt.Payload) {
			return errors.New("Track load failed: Spotify did not provide a valid account country")
		}
		country = string(pkt.Payload)
	case <-ctx.Done():
		return fmt.Errorf("Track load failed while waiting for the account country: %w", ctx.Err())
	}

	p, err := player.NewPlayer(&player.Options{
		Log:         &librespot.NullLogger{},
		Spclient:    sess.Spclient(),
		AudioKey:    sess.AudioKey(),
		Events:      sess.Events(),
		CountryCode: func() string { return country },
	})
	if err != nil {
		return fmt.Errorf("Player construction failed: %w", err)
	}
	defer p.Close()

	stream, err := p.NewStream(ctx, client, id, minimumBitrate, 0)
	if err != nil {
		if isKeyDenial(err) {
			return fmt.Errorf("Audio key denied: %w", err)
		}
		return fmt.Errorf("Track load failed: %w", err)
	}
	if closer, ok := stream.Source.(io.Closer); ok {
		defer closer.Close()
	}

	return readAudio(ctx, stream.Source, int(stream.SampleRate)*player.Channels*minimumSeconds)
}

func isKeyDenial(err error) bool {
	var denied *audio.KeyProviderError
	return errors.As(err, &denied)
}

func readAudio(ctx context.Context, source librespot.Float32Reader, target int) error {
	if target <= 0 {
		return errors.New("Audio decode failed: invalid sample rate")
	}
	samples := make([]float32, 4096)
	var count int
	var nonSilent bool
	for count < target {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("Audio decode failed after %d samples: %w", count, err)
		}
		n, err := source.Read(samples[:min(len(samples), target-count)])
		for _, s := range samples[:n] {
			if math.IsNaN(float64(s)) || math.IsInf(float64(s), 0) {
				return errors.New("Audio decode failed: non-finite PCM sample")
			}
			nonSilent = nonSilent || s != 0
		}
		count += n
		if err != nil && !(errors.Is(err, io.EOF) && count == target) {
			return fmt.Errorf("Audio decode failed after %d of %d samples: %w", count, target, err)
		}
		if n == 0 {
			return fmt.Errorf("Audio decode failed after %d samples: decoder made no progress", count)
		}
	}
	if !nonSilent {
		return fmt.Errorf("Audio decode failed: all %d samples were silent", count)
	}
	return nil
}
func sessionCredentials(sess *session.Session, deviceID string) credentials {
	return credentials{DeviceID: deviceID, Username: sess.Username(), Data: sess.StoredCredentials()}
}

func loadCredentials(path string) (credentials, error) {
	var creds credentials
	data, err := os.ReadFile(path)
	if err != nil {
		return creds, err
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return creds, errors.New("invalid credentials.json; re-run login")
	}
	if err := creds.validate(); err != nil {
		return creds, err
	}
	return creds, nil
}

func (c credentials) validate() error {
	id, err := hex.DecodeString(c.DeviceID)
	if err != nil || len(id) != 20 || c.Username == "" || len(c.Data) == 0 {
		return errors.New("credentials are incomplete or invalid; re-run login")
	}
	return nil
}

func saveCredentials(path string, creds credentials) error {
	if err := creds.validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".credentials-*.json") // Creates the file with mode 0600.
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := json.NewEncoder(file).Encode(creds); err != nil {
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

// Keep upstream diagnostics while removing request URLs and known secrets.
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
