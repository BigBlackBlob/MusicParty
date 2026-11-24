package media

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strconv"
)

var youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// DownloadYTDLP downloads an anonymous audio-only rendition to a cache-owned
// partial path. yt-dlp's .part file supports HTTP resume; the cache publishes
// the completed file only after this command succeeds.
func DownloadYTDLP(ctx context.Context, executable, videoID, target string, maxBytes int64) error {
	if !youtubeID.MatchString(videoID) {
		return errors.New("invalid YouTube video ID")
	}
	if executable == "" {
		executable = "yt-dlp"
	}
	limit := strconv.FormatInt(maxBytes, 10)
	// Restrict to a single HTTPS file with a known size. Turning off fixups also
	// keeps yt-dlp from creating a second copy outside the cache reservation.
	format := "140[protocol=https][ext=m4a][acodec^=mp4a][filesize<=" + limit + "]"
	err := runProcessTree(ctx, executable,
		"--ignore-config", "--no-cache-dir", "--no-playlist", "--no-warnings", "--no-progress",
		"--js-runtimes", "node", "--socket-timeout", "15", "--retries", "3",
		"--continue", "--part", "--fixup", "never", "--max-filesize", limit, "-f", format,
		"-o", target, "https://www.youtube.com/watch?v="+videoID)
	if err != nil {
		return errors.New("yt-dlp download failed")
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return errors.New("yt-dlp did not produce a complete audio file within the size limit")
	}
	return nil
}
