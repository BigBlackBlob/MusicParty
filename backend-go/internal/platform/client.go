package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type UpstreamError struct {
	Status     int
	RetryAfter time.Duration
	Cause      error
}

func (e *UpstreamError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("upstream returned HTTP %d", e.Status)
}
func (e *UpstreamError) Unwrap() error { return e.Cause }

type Client struct {
	HTTP             *http.Client
	Retries          int
	MaxBody          int64
	UserAgent        string
	OperationTimeout time.Duration
}

func NewClient(timeout time.Duration) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&netDialer{timeout: min(timeout, 5*time.Second)}).DialContext
	transport.ResponseHeaderTimeout = min(timeout, 10*time.Second)
	return &Client{HTTP: &http.Client{Transport: transport, Timeout: timeout}, Retries: 2, MaxBody: 8 << 20, UserAgent: "MusicParty/Go", OperationTimeout: timeout}
}

type netDialer struct{ timeout time.Duration }

func (d *netDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{Timeout: d.timeout, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}

func (c *Client) JSON(ctx context.Context, rawURL string, headers map[string]string, target any) error {
	if c.OperationTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.OperationTimeout)
		defer cancel()
	}
	var last error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.UserAgent)
		for key, value := range headers {
			if value != "" {
				req.Header.Set(key, value)
			}
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			last = &UpstreamError{Cause: err}
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, c.MaxBody+1))
			resp.Body.Close()
			if readErr != nil {
				return &UpstreamError{Status: resp.StatusCode, Cause: readErr}
			}
			if int64(len(body)) > c.MaxBody {
				return &UpstreamError{Status: resp.StatusCode, Cause: errors.New("upstream response is too large")}
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if err := json.Unmarshal(body, target); err != nil {
					return &UpstreamError{Status: resp.StatusCode, Cause: fmt.Errorf("decode upstream JSON: %w", err)}
				}
				return nil
			}
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
			last = &UpstreamError{Status: resp.StatusCode, RetryAfter: retryAfter}
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return last
			}
		}
		if attempt == c.Retries {
			break
		}
		delay := time.Duration(100*(1<<attempt)) * time.Millisecond
		var upstream *UpstreamError
		if errors.As(last, &upstream) && upstream.RetryAfter > 0 {
			delay = upstream.RetryAfter
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	return when.Sub(now)
}
