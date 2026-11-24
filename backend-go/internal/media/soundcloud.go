package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// soundCloudTrackID matches the numeric track identifiers SoundCloud uses.
var soundCloudTrackID = regexp.MustCompile(`^[0-9]{1,20}$`)

// DownloadSoundCloud materializes a SoundCloud track into a cache-owned file.
//
// SoundCloud serves AAC in HLS segments behind a signed URL that expires after
// roughly two hours, and it forbids storing user audio beyond the session that
// requested it. The caller therefore owns a short-lived cache directory and
// deletes it when the session ends.
//
// yt-dlp resolves the public playback location on its own, which keeps the
// service free of SoundCloud credentials. It writes HLS segments, so ffmpeg
// joins them into one seekable file.
func DownloadSoundCloud(ctx context.Context, executable, ffmpeg, trackID, target string, maxBytes int64) error {
	if !soundCloudTrackID.MatchString(trackID) {
		return errors.New("invalid SoundCloud track ID")
	}
	if strings.TrimSpace(executable) == "" {
		executable = "yt-dlp"
	}
	if strings.TrimSpace(ffmpeg) == "" {
		ffmpeg = "ffmpeg"
	}
	limit := strconv.FormatInt(maxBytes, 10)
	workDir, err := os.MkdirTemp("", "musicparty-sc-")
	if err != nil {
		return errors.New("create SoundCloud working directory")
	}
	defer os.RemoveAll(workDir)
	// The segments are joined into one file, so the intermediate pieces never
	// need to fit inside the cache reservation.
	output := filepath.Join(workDir, "audio.m4a")
	err = runProcessTree(ctx, executable,
		"--ignore-config", "--no-cache-dir", "--no-playlist", "--no-warnings", "--no-progress",
		"--socket-timeout", "20", "--retries", "3", "--continue", "--part", "--fixup", "never",
		"-f", "hls_aac_160k/hls_aac_96k/hls/best", "--max-filesize", limit,
		"-o", output, "https://api.soundcloud.com/tracks/"+trackID)
	if err != nil {
		return errors.New("soundcloud download failed")
	}
	if info, statErr := os.Stat(output); statErr != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return errors.New("soundcloud download did not produce a playable file")
	}
	// Remux the joined segments into a single seekable MP4 container.
	// The target name is chosen by the cache and ends in ".part", so the
	// output format must be stated explicitly.
	err = runProcessTree(ctx, ffmpeg,
		"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-i", output, "-c", "copy", "-movflags", "+faststart", "-f", "mp4", target)
	if err != nil {
		return errors.New("soundcloud remux failed")
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 {
		return errors.New("soundcloud remux did not produce a complete file")
	}
	if info.Size() > maxBytes {
		_ = os.Remove(target)
		return errors.New("soundcloud audio exceeds the single-file limit")
	}
	return nil
}
