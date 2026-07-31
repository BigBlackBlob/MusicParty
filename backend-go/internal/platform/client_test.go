package platform

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientRetriesTransientFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	client := &Client{HTTP: server.Client(), Retries: 2, MaxBody: 1024}
	var value struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, client.JSON(context.Background(), server.URL, nil, &value))
	require.True(t, value.OK)
	require.Equal(t, int32(2), calls.Load())
}
func TestClientHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client := &Client{HTTP: server.Client(), Retries: 0, MaxBody: 1024}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := client.JSON(ctx, server.URL, nil, &struct{}{})
	require.True(t, errors.Is(err, context.DeadlineExceeded))
}
func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	require.Equal(t, 3*time.Second, parseRetryAfter("3", now))
	require.Equal(t, 5*time.Second, parseRetryAfter(now.Add(5*time.Second).Format(http.TimeFormat), now))
}
func TestClientRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`not-json`)) }))
	defer server.Close()
	client := &Client{HTTP: server.Client(), Retries: 0, MaxBody: 1024}
	err := client.JSON(context.Background(), server.URL, nil, &struct{}{})
	require.ErrorContains(t, err, "decode upstream JSON")
}
