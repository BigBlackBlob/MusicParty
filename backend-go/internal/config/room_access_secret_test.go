package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func productionTestConfig(production bool, secret string) Config {
	return Config{
		Server: Server{Port: 8080},
		Application: Application{
			Mode:           "server",
			Production:     production,
			BaseURL:        "https://music.example",
			AllowedOrigins: []string{"https://music.example"},
		},
		Chat:  Chat{MaxHistorySize: 1000, MaxMessageLength: 200},
		Queue: Queue{MaxSize: 1000, MaxUserSongs: 100, HistorySize: 50},
		Performance: Performance{
			WebSocketClientQueueCapacity: 1,
			RoomCommandQueueCapacity:     1,
			DBWriteQueueCapacity:         1,
		},
		Database: Database{ConnectionTimeout: time.Second},
		Auth:     Auth{RoomAccessTokenSecret: secret},
	}
}

func TestValidateRequiresRoomAccessTokenSecretInProduction(t *testing.T) {
	tests := []struct {
		name          string
		production    bool
		secret        string
		wantErrSubstr string
	}{
		{
			name:          "production without secret",
			production:    true,
			secret:        "",
			wantErrSubstr: "ROOM_ACCESS_TOKEN_SECRET must be set in production deployments",
		},
		{
			name:          "production with blank secret",
			production:    true,
			secret:        "   ",
			wantErrSubstr: "ROOM_ACCESS_TOKEN_SECRET must be set in production deployments",
		},
		{
			name:          "production with short secret",
			production:    true,
			secret:        "too-short",
			wantErrSubstr: "ROOM_ACCESS_TOKEN_SECRET must be at least 16 characters",
		},
		{
			name:       "production with valid secret",
			production: true,
			secret:     "0123456789abcdef",
		},
		{
			name:       "development without secret",
			production: false,
			secret:     "",
		},
		{
			name:       "development with short secret",
			production: false,
			secret:     "short",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := productionTestConfig(test.production, test.secret)
			err := cfg.Validate()
			if test.wantErrSubstr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if !strings.Contains(err.Error(), test.wantErrSubstr) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.wantErrSubstr)
			}
		})
	}
}
