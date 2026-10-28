package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
)

func (l *LocalLibrary) reserveTrack(id string) bool {
	l.jobsMu.Lock()
	defer l.jobsMu.Unlock()
	if _, exists := l.inFlight[id]; exists {
		return false
	}
	l.inFlight[id] = struct{}{}
	return true
}

func (l *LocalLibrary) releaseTrack(id string) {
	l.jobsMu.Lock()
	defer l.jobsMu.Unlock()
	delete(l.inFlight, id)
}

// RecoverPending runs once at startup in one cancellable feeder. Keyset pages and
// waiting for queue space keep recovery bounded even when the backlog is large.
func (l *LocalLibrary) RecoverPending(ctx context.Context) error {
	after := ""
	for {
		tracks, err := l.repository.FindPendingAfter(ctx, after, 64)
		if err != nil {
			return err
		}
		if len(tracks) == 0 {
			return nil
		}
		for _, track := range tracks {
			after = track.ID
			if err := ctx.Err(); err != nil {
				return err
			}
			if !l.reserveTrack(track.ID) {
				continue
			}
			if err := l.submitReserved(ctx, track.ID, true); err != nil {
				if ctx.Err() != nil || l.transcoder.ctx.Err() != nil {
					return err
				}
				l.reportError(fmt.Errorf("recover local track %s: %w", track.ID, err))
			}
		}
	}
}

// submitReserved owns the reservation until either submission fails or the job
// callback finishes. Uploads and recovery therefore cannot run the same ID twice.
func (l *LocalLibrary) submitReserved(ctx context.Context, id string, wait bool) (err error) {
	submitted := false
	defer func() {
		if !submitted {
			l.releaseTrack(id)
		}
	}()
	track, err := l.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if track.Status == "DELETED" || track.Status == "COMPLETED" {
		return nil
	}
	var source string
	if track.SourcePath == nil {
		err = errors.New("transcode source is missing")
	} else {
		source, err = ResolveWithin(l.root, *track.SourcePath)
		if err == nil {
			var info os.FileInfo
			info, err = os.Stat(source)
			if err == nil && !info.Mode().IsRegular() {
				err = errors.New("transcode source is not a regular file")
			}
		}
	}
	if err != nil {
		message := err.Error()
		l.updateStatus(id, storesqlite.LocalTrackStatusUpdate{Status: "FAILED", ErrorMessage: &message, StatusMessage: optional("Source unavailable; upload again to retry"), UpdatedAt: l.now().UnixMilli()})
		return err
	}
	relativeOutput := filepath.ToSlash(filepath.Join("media", id+".ogg"))
	output, err := ResolveWithin(l.root, relativeOutput)
	if err != nil {
		return err
	}
	job := TranscodeJob{ID: id, Input: source, Output: output,
		Start: func() error {
			now := l.now().UnixMilli()
			return l.repository.UpdateStatus(context.Background(), id, storesqlite.LocalTrackStatusUpdate{Status: "PROCESSING", StatusMessage: optional("Transcoding"), ProgressPercent: intPointer(0), StartedAt: &now, UpdatedAt: now})
		},
		Complete: func(err error) {
			defer l.releaseTrack(id)
			l.completeTranscode(id, *track.SourcePath, relativeOutput, err)
		},
	}
	if wait {
		err = l.transcoder.SubmitWait(ctx, job)
	} else {
		err = l.transcoder.Submit(job)
	}
	if err != nil {
		// Cancellation leaves recoverable work pending for the next startup.
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			message := err.Error()
			l.updateStatus(id, storesqlite.LocalTrackStatusUpdate{Status: "FAILED", ErrorMessage: &message, StatusMessage: optional("Transcode queue rejected"), UpdatedAt: l.now().UnixMilli()})
		}
		return err
	}
	submitted = true
	return nil
}

func (l *LocalLibrary) retryUpload(ctx context.Context, existing storesqlite.LocalTrack, source string, upload Upload, size int64) (storesqlite.LocalTrack, error) {
	if existing.Status == "COMPLETED" || !l.reserveTrack(existing.ID) {
		l.removeOutputs(&source)
		return existing, nil
	}
	current, err := l.repository.FindByID(ctx, existing.ID)
	if err != nil || current.Status == "DELETED" || current.Status == "COMPLETED" {
		l.releaseTrack(existing.ID)
		l.removeOutputs(&source)
		if err == nil && current.Status == "DELETED" {
			err = storesqlite.ErrLocalTrackGone
		}
		if current != nil {
			return *current, err
		}
		return existing, err
	}
	// Wait for the committed result before discarding either input file.
	if err = ctx.Err(); err == nil {
		err = l.repository.RetryTranscode(context.WithoutCancel(ctx), existing.ID, source, optional(filepath.Base(upload.FileName)), optional(upload.ContentType), size, l.now().UnixMilli())
	}
	if err != nil {
		l.releaseTrack(existing.ID)
		l.removeOutputs(&source)
		return existing, err
	}
	if current.SourcePath != nil && *current.SourcePath != source {
		l.removeOutputs(current.SourcePath)
	}
	err = l.submitReserved(ctx, existing.ID, false)
	track, findErr := l.repository.FindByID(ctx, existing.ID)
	if findErr != nil {
		return existing, errors.Join(err, findErr)
	}
	return *track, err
}
