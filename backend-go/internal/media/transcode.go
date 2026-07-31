package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

var ErrTranscodeQueueFull = errors.New("transcode queue is full")

type TranscodeJob struct {
	ID, Input, Output string
	Complete          func(error)
}

// Transcoder owns a bounded worker queue and cancels subprocesses on shutdown.
type Transcoder struct {
	ctx      context.Context
	cancel   context.CancelFunc
	ffmpeg   string
	timeout  time.Duration
	jobs     chan TranscodeJob
	wait     sync.WaitGroup
	run      func(context.Context, string, ...string) error
	closeOne sync.Once
	stateMu  sync.Mutex
	closed   bool
}

func NewTranscoder(ffmpeg string, workers, capacity int, timeout time.Duration) *Transcoder {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Transcoder{ctx: ctx, cancel: cancel, ffmpeg: ffmpeg, timeout: timeout, jobs: make(chan TranscodeJob, max(1, capacity)), run: runProcessTree}
	for range max(1, workers) {
		t.wait.Add(1)
		go t.worker()
	}
	return t
}

func (t *Transcoder) Submit(job TranscodeJob) error {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	if t.closed {
		return context.Canceled
	}
	select {
	case <-t.ctx.Done():
		return context.Canceled
	case t.jobs <- job:
		return nil
	default:
		return ErrTranscodeQueueFull
	}
}

func (t *Transcoder) Close() {
	t.closeOne.Do(func() {
		t.stateMu.Lock()
		t.closed = true
		t.stateMu.Unlock()
		t.cancel()
		t.wait.Wait()
	})
}

func (t *Transcoder) worker() {
	defer t.wait.Done()
	for {
		select {
		case <-t.ctx.Done():
			return
		case job := <-t.jobs:
			err := t.execute(job)
			if job.Complete != nil {
				job.Complete(err)
			}
		}
	}
}

func (t *Transcoder) execute(job TranscodeJob) error {
	if err := os.MkdirAll(filepath.Dir(job.Output), 0o750); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(t.ctx, t.timeout)
	defer cancel()
	temporary := job.Output + ".part"
	_ = os.Remove(temporary)
	err := t.run(ctx, t.ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", job.Input, "-vn", "-c:a", "libopus", "-b:a", "160k", "-f", "ogg", temporary)
	if err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("ffmpeg transcode: %w", err)
	}
	if err := os.Rename(temporary, job.Output); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func command(ctx context.Context, executable string, arguments ...string) *exec.Cmd {
	return exec.CommandContext(ctx, executable, arguments...)
}
