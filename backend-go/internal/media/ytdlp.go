package media

import (
	"context"
	"errors"
	"strings"
	"time"
)

func ResolveYTDLP(ctx context.Context, executable, videoID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := runProcessTreeOutput(ctx, executable, "--no-playlist", "--no-warnings", "-f", "bestaudio", "-g", "https://www.youtube.com/watch?v="+videoID)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(output), "\n") {
		if value := strings.TrimSpace(line); strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
			return value, nil
		}
	}
	return "", errors.New("yt-dlp did not return a media URL")
}
