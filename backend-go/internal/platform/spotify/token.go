package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type tokenManager struct {
	clientID, clientSecret, baseURL string
	client                          *http.Client
	now                             func() time.Time
	mu                              sync.Mutex
	bearer                          string
	expiry                          time.Time
}

func (m *tokenManager) AppToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if m.bearer != "" && m.expiry.Sub(m.now()) > time.Minute {
		return m.bearer, nil
	}
	return m.fetchToken(ctx)
}

// Invalidate clears only the rejected bearer, since another request may have renewed it already.
func (m *tokenManager) Invalidate(rejected string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bearer == rejected {
		m.bearer = ""
	}
}

func (m *tokenManager) fetchToken(ctx context.Context) (string, error) {
	started := m.now()
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("spotify: invalid token request")
	}
	req.SetBasicAuth(m.clientID, m.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := requestJSON(m.client, req, "token", 64<<10, &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" || token.ExpiresIn <= 0 || token.ExpiresIn > int64((1<<63-1)/time.Second) {
		return "", fmt.Errorf("spotify: invalid token response")
	}
	m.bearer, m.expiry = token.AccessToken, started.Add(time.Duration(token.ExpiresIn)*time.Second)
	return m.bearer, nil
}

type responseError struct {
	operation string
	status    int
}

func (e *responseError) Error() string {
	return fmt.Sprintf("spotify: %s returned HTTP %d", e.operation, e.status)
}

type RateLimitError struct {
	RetryAfter    time.Duration
	QuotaExceeded bool
}

func (e *RateLimitError) Error() string {
	if e.QuotaExceeded {
		return "Spotify API quota exceeded"
	}
	return "Spotify API rate limit reached"
}

func retryDelay(value string) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 && seconds <= int64((1<<63-1)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return max(0, time.Until(date))
	}
	return 0
}

func requestJSON(client *http.Client, req *http.Request, operation string, limit int64, target any) error {
	res, err := client.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return req.Context().Err()
		}
		return fmt.Errorf("spotify: %s request failed", operation)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		var info struct {
			Reason string `json:"reason"`
			Error  struct {
				Reason string `json:"reason"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&info)
		return &RateLimitError{retryDelay(res.Header.Get("Retry-After")), info.Reason == "QUOTA_EXCEEDED" || info.Error.Reason == "QUOTA_EXCEEDED"}
	}
	if res.StatusCode != http.StatusOK {
		return &responseError{operation, res.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return fmt.Errorf("spotify: could not read %s response", operation)
	}
	if int64(len(body)) > limit {
		return fmt.Errorf("spotify: %s response is too large", operation)
	}
	if json.Unmarshal(body, target) != nil {
		return fmt.Errorf("spotify: invalid %s response", operation)
	}
	return nil
}
