package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTranscoderTimeoutRemovesPartialOutput(t *testing.T) {
	transcoder := NewTranscoder("unused", 1, 1, 20*time.Millisecond)
	transcoder.run = func(ctx context.Context, _ string, _ ...string) error {
		<-ctx.Done()
		return ctx.Err()
	}
	defer transcoder.Close()
	output := filepath.Join(t.TempDir(), "output.ogg")
	done := make(chan error, 1)
	require.NoError(t, transcoder.Submit(TranscodeJob{Input: "input", Output: output, Complete: func(err error) { done <- err }}))
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("transcode timeout did not complete")
	}
	require.NoFileExists(t, output)
	require.NoFileExists(t, output+".part")
}

func TestTranscoderWithRealFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	root := t.TempDir()
	input := filepath.Join(root, "input.wav")
	generate := exec.Command(ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.2", input)
	require.NoError(t, generate.Run())
	output := filepath.Join(root, "output.ogg")
	transcoder := NewTranscoder(ffmpeg, 1, 1, 10*time.Second)
	defer transcoder.Close()
	done := make(chan error, 1)
	require.NoError(t, transcoder.Submit(TranscodeJob{Input: input, Output: output, Complete: func(err error) { done <- err }}))
	require.NoError(t, <-done)
	require.FileExists(t, output)
	require.NoFileExists(t, output+".part")
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Greater(t, len(data), 4)
	require.Equal(t, "OggS", string(data[:4]))
}
