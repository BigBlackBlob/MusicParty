package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBatch8DownloadConfiguration(t *testing.T) {
	cfg, err := Load(lookup(map[string]string{
		"DOWNLOAD_MAX_FILE_BYTES": "8192", "DOWNLOAD_RESERVATION_BYTES": "4096", "DOWNLOAD_TASK_TTL_MS": "250", "DOWNLOAD_RETRY_MAX_DELAY_MS": "20",
	}))
	require.NoError(t, err)
	require.EqualValues(t, 8192, cfg.Performance.DownloadMaxFileBytes)
	require.EqualValues(t, 4096, cfg.Performance.DownloadReservationBytes)
	require.Equal(t, 250*time.Millisecond, cfg.Performance.DownloadTaskTTL)
	require.Equal(t, 20*time.Millisecond, cfg.Performance.DownloadRetryMaxDelay)
	_, err = Load(lookup(map[string]string{"DOWNLOAD_MAX_FILE_BYTES": "0"}))
	require.ErrorContains(t, err, "cache download limits")
}
