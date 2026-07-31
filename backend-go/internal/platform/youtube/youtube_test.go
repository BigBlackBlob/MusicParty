package youtube

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	value, err := parseDuration("P1DT2H3M4S")
	require.NoError(t, err)
	require.Equal(t, 26*time.Hour+3*time.Minute+4*time.Second, value)
}
