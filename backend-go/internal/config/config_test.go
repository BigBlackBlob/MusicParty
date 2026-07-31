package config

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func lookup(values map[string]string) LookupEnv {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(lookup(nil))
	require.NoError(t, err)
	if cfg.Server.Port != 8080 || cfg.Database.Path != "data/musicparty.db" {
		t.Fatalf("unexpected defaults: port=%d db=%q", cfg.Server.Port, cfg.Database.Path)
	}
	if cfg.Performance.WebSocketClientQueueCapacity != 256 || cfg.Performance.RoomCommandQueueCapacity != 100 || cfg.Performance.RoomCommandQueueTimeout != 5*time.Second {
		t.Fatalf("compatibility queue defaults changed: %+v", cfg.Performance)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(lookup(map[string]string{
		"SERVER_PORT": "18080", "ALLOWED_ORIGINS": "https://one.example, https://two.example",
		"BASE_URL":            "https://one.example",
		"AUTH_SECURE_COOKIES": "false", "ROOM_COMMAND_QUEUE_TIMEOUT_MS": "7500",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wantOrigins := []string{"https://one.example", "https://two.example"}
	if diff := cmp.Diff(wantOrigins, cfg.Application.AllowedOrigins); diff != "" {
		t.Fatalf("origins mismatch (-want +got):\n%s", diff)
	}
	if cfg.Server.Port != 18080 || cfg.Auth.SecureCookies || cfg.Performance.RoomCommandQueueTimeout != 7500*time.Millisecond {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	_, err := Load(lookup(map[string]string{"SERVER_PORT": "70000", "DB_ENABLED": "sometimes"}))
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func TestProductionValidationMatchesJavaBaseline(t *testing.T) {
	_, err := Load(lookup(map[string]string{
		"APP_ENV":  "production",
		"BASE_URL": "http://localhost:8080",
	}))
	if err == nil || !strings.Contains(err.Error(), "BASE_URL must not point to localhost") {
		t.Fatalf("Load() error = %v", err)
	}

	cfg, err := Load(lookup(map[string]string{
		"APP_ENV":         "production",
		"BASE_URL":        "https://music.example",
		"ALLOWED_ORIGINS": "https://music.example",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Application.Production {
		t.Fatal("Production = false, want true")
	}
}
