package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	callbackAddr = "127.0.0.1:3678"
	callbackPath = "/callback"
	redirectURI  = "http://" + callbackAddr + callbackPath
	authorizeURL = "https://accounts.spotify.com/authorize"
	tokenURL     = "https://accounts.spotify.com/api/token"
	oauthWait    = 120 * time.Second
)

type oauthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Username     string `json:"username"`
}

type callback struct {
	code string
	err  error
}

func authorize(ctx context.Context, client *http.Client, clientID string) (oauthTokens, error) {
	verifier := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	challenge := sha256.Sum256([]byte(verifier))
	state := base64.RawURLEncoding.EncodeToString(randomBytes(32))
	params := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {"streaming user-read-email user-read-private"},
		"code_challenge_method": {"S256"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"state":                 {state},
	}

	code, err := waitForCode(ctx, authorizeURL+"?"+params.Encode(), state)
	if err != nil {
		return oauthTokens{}, fmt.Errorf("OAuth callback failed: %w", err)
	}
	tokens, err := exchangeCode(ctx, client, clientID, verifier, code)
	if err != nil {
		return oauthTokens{}, fmt.Errorf("OAuth exchange failed: %w", err)
	}
	return tokens, nil
}

func waitForCode(ctx context.Context, loginURL, state string) (string, error) {
	listener, err := net.Listen("tcp4", callbackAddr)
	if err != nil {
		return "", err
	}

	results := make(chan callback, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+callbackPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		query := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(state)) != 1 {
			http.Error(w, "OAuth state did not match. Open the original login URL and try again.", http.StatusBadRequest)
			return
		}
		result := callback{code: query.Get("code")}
		switch {
		case query.Get("error") == "access_denied":
			result.err = errors.New("Spotify authorization was denied")
		case query.Get("error") != "":
			result.err = errors.New("Spotify returned an authorization error")
		case result.code == "":
			result.err = errors.New("Spotify callback did not contain an authorization code")
		}
		select {
		case results <- result:
		default:
		}
		if result.err != nil {
			http.Error(w, result.err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Spotify callback received. Return to the terminal to see the result.")
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, oauthWait)
	defer cancel()
	fmt.Println("Open this URL to sign in to Spotify:")
	fmt.Println(loginURL)
	openBrowser(loginURL)
	select {
	case result := <-results:
		return result.code, result.err
	case err := <-serveErr:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("waiting up to %s for Spotify to redirect to %s: %w", oauthWait, redirectURI, ctx.Err())
	}
}

func openBrowser(loginURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Pass the URL as data so PowerShell does not interpret its query string.
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Process -FilePath $env:SPOTIFY_PROBE_LOGIN_URL")
		cmd.Env = append(os.Environ(), "SPOTIFY_PROBE_LOGIN_URL="+loginURL)
	case "linux":
		cmd = exec.Command("xdg-open", loginURL)
	default:
		return
	}
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}

func exchangeCode(ctx context.Context, client *http.Client, clientID, verifier, code string) (oauthTokens, error) {
	if client == nil {
		client = http.DefaultClient
	}
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return oauthTokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// The callback code and PKCE verifier belong only at the token endpoint.
	tokenClient := *client
	tokenClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := tokenClient.Do(req)
	if err != nil {
		return oauthTokens{}, errors.New(safeError(err, code, verifier))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return oauthTokens{}, fmt.Errorf("Spotify token endpoint returned HTTP %d", resp.StatusCode)
	}
	var tokens oauthTokens
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&tokens); err != nil {
		return oauthTokens{}, errors.New("Spotify returned an invalid token response")
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		return oauthTokens{}, errors.New("Spotify did not return both an access token and a refresh token")
	}
	return tokens, nil
}
