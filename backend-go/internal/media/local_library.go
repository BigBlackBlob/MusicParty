package media

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/google/uuid"
)

type Upload struct {
	FileName, ContentType, Title, Artists, Album, UploadedBy string
	Size                                                     int64
	Reader                                                   io.Reader
}

type LocalLibrary struct {
	root       string
	maxUpload  int64
	repository *storesqlite.LocalTrackRepository
	transcoder *Transcoder
	now        func() time.Time
	jobsMu     sync.Mutex
	inFlight   map[string]struct{}
	// report receives background failures (transcode results that can no longer be stored) that
	// no request handler can return to a caller.
	report func(error)
	// removed is called with the deleted track and the rooms that referenced it right after its
	// references were removed, so the caller can refresh the in-memory room state it owns.
	removed func(ctx context.Context, trackID string, roomIDs []string)
}

func NewLocalLibrary(root string, maxUpload int64, repository *storesqlite.LocalTrackRepository, transcoder *Transcoder) (*LocalLibrary, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	for _, directory := range []string{"source", "media", "covers"} {
		if err := os.MkdirAll(filepath.Join(absolute, directory), 0o750); err != nil {
			return nil, err
		}
	}
	library := &LocalLibrary{root: absolute, maxUpload: maxUpload, repository: repository, transcoder: transcoder, now: time.Now, inFlight: make(map[string]struct{})}
	library.report = func(err error) { log.Printf("local library: %v", err) }
	return library, nil
}

func (l *LocalLibrary) Root() string { return l.root }

// SetErrorReporter replaces the sink for background failures, e.g. to route them through the
// server logger. A nil reporter only silences output; failures are never swallowed silently.
func (l *LocalLibrary) SetErrorReporter(report func(error)) {
	if report == nil {
		report = func(error) {}
	}
	l.report = report
}

// SetTracksRemovedHook installs the callback that runs after a deleted track's room references are
// gone. The library does not know about the realtime package; the caller wires the two together.
func (l *LocalLibrary) SetTracksRemovedHook(hook func(ctx context.Context, trackID string, roomIDs []string)) {
	l.removed = hook
}

func (l *LocalLibrary) notifyTracksRemoved(ctx context.Context, trackID string, roomIDs []string) {
	if l.removed == nil || len(roomIDs) == 0 {
		return
	}
	l.removed(ctx, trackID, roomIDs)
}

func (l *LocalLibrary) reportError(err error) {
	if err == nil {
		return
	}
	if l.report != nil {
		l.report(err)
	}
}

func (l *LocalLibrary) Upload(ctx context.Context, upload Upload) (storesqlite.LocalTrack, bool, error) {
	if upload.Reader == nil || upload.Size <= 0 || upload.Size > l.maxUpload {
		return storesqlite.LocalTrack{}, false, errors.New("uploaded file size is invalid")
	}
	id := uuid.NewString()
	extension := strings.ToLower(filepath.Ext(filepath.Base(upload.FileName)))
	if len(extension) > 12 {
		extension = ""
	}
	relativeSource := filepath.ToSlash(filepath.Join("source", id+extension))
	sourcePath, err := ResolveWithin(l.root, relativeSource)
	if err != nil {
		return storesqlite.LocalTrack{}, false, err
	}
	file, err := os.OpenFile(sourcePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return storesqlite.LocalTrack{}, false, err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(upload.Reader, l.maxUpload+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written != upload.Size || written > l.maxUpload {
		_ = os.Remove(sourcePath)
		return storesqlite.LocalTrack{}, false, errors.Join(copyErr, closeErr, errors.New("uploaded file length does not match request"))
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	if existing, findErr := l.repository.FindActiveByOriginalHash(ctx, hash); findErr == nil {
		track, err := l.retryUpload(ctx, *existing, relativeSource, upload, written)
		return track, true, err
	} else if !errors.Is(findErr, sql.ErrNoRows) && !strings.Contains(findErr.Error(), sql.ErrNoRows.Error()) {
		_ = os.Remove(sourcePath)
		return storesqlite.LocalTrack{}, false, findErr
	}
	now := l.now().UnixMilli()
	title := strings.TrimSpace(upload.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(upload.FileName), extension)
	}
	artists := splitInputArtists(upload.Artists)
	album := optional(upload.Album)
	track := storesqlite.LocalTrack{ID: id, OriginalHash: &hash, OriginalFileName: optional(filepath.Base(upload.FileName)), SourcePath: &relativeSource, SourceMIMEType: optional(upload.ContentType), SourceSizeBytes: written, Title: title, Artists: artists, Album: album, Status: "PENDING", StatusMessage: optional("Waiting for transcode"), ProgressPercent: intPointer(0), UploadedBy: optional(upload.UploadedBy), CreatedAt: now, UpdatedAt: now}
	if err := l.repository.Upsert(ctx, track); err != nil {
		if existing, findErr := l.repository.FindActiveByOriginalHash(ctx, hash); findErr == nil {
			track, err := l.retryUpload(ctx, *existing, relativeSource, upload, written)
			return track, true, err
		}
		_ = os.Remove(sourcePath)
		return storesqlite.LocalTrack{}, false, err
	}
	if l.reserveTrack(id) {
		err = l.submitReserved(ctx, id, false)
	}
	return track, false, err
}

func (l *LocalLibrary) completeTranscode(id, sourceRelative, outputRelative string, transcodeErr error) {
	now := l.now().UnixMilli()
	if transcodeErr != nil {
		if errors.Is(transcodeErr, storesqlite.ErrLocalTrackGone) {
			return
		}
		message := transcodeErr.Error()
		if errors.Is(transcodeErr, context.Canceled) {
			l.updateStatus(id, storesqlite.LocalTrackStatusUpdate{Status: "PENDING", StatusMessage: optional("Waiting for restart"), ProgressPercent: intPointer(0), UpdatedAt: now})
			return
		}
		l.updateStatus(id, storesqlite.LocalTrackStatusUpdate{Status: "FAILED", ErrorMessage: &message, StatusMessage: optional("Transcode failed"), ProgressPercent: intPointer(0), CompletedAt: &now, UpdatedAt: now})
		return
	}
	source, err := ResolveWithin(l.root, sourceRelative)
	if err != nil {
		l.reportError(fmt.Errorf("complete transcode %s: %w", id, err))
		return
	}
	duration := l.probeDuration(source)
	coverRelative, coverMIME := l.extractCover(id, source)
	if err := l.repository.CompleteTranscode(context.Background(), id, outputRelative, duration, coverRelative, coverMIME, now); err != nil {
		// The track was deleted while ffmpeg was running. Keep the source file (never delete media
		// for a write that did not land) and drop the transcode output that just became worthless.
		l.reportError(fmt.Errorf("complete transcode %s: %w", id, err))
		if errors.Is(err, storesqlite.ErrLocalTrackGone) {
			l.removeOutputs(&outputRelative, coverRelative)
		}
		return
	}
	if source, err := ResolveWithin(l.root, sourceRelative); err == nil {
		_ = os.Remove(source)
	}
}

// updateStatus records a job status, reporting a rejected write instead of dropping it.
func (l *LocalLibrary) updateStatus(id string, update storesqlite.LocalTrackStatusUpdate) {
	if err := l.repository.UpdateStatus(context.Background(), id, update); err != nil {
		l.reportError(fmt.Errorf("update local track %s status: %w", id, err))
	}
}

// removeOutputs deletes obsolete library files, including superseded retry inputs.
func (l *LocalLibrary) removeOutputs(relative ...*string) {
	for _, value := range relative {
		if value == nil {
			continue
		}
		path, err := ResolveWithin(l.root, *value)
		if err != nil {
			l.reportError(err)
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			l.reportError(err)
		}
	}
}

func (l *LocalLibrary) probeDuration(source string) int64 {
	probe := "ffprobe"
	if directory := filepath.Dir(l.transcoder.ffmpeg); directory != "." {
		probe = filepath.Join(directory, "ffprobe"+filepath.Ext(l.transcoder.ffmpeg))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := runProcessTreeOutput(ctx, probe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", source)
	if err != nil {
		return 0
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return int64(seconds * 1000)
}

func (l *LocalLibrary) extractCover(id, source string) (*string, *string) {
	relative := filepath.ToSlash(filepath.Join("covers", id+".jpg"))
	path, err := ResolveWithin(l.root, relative)
	if err != nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := runProcessTree(ctx, l.transcoder.ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", source, "-map", "0:v:0", "-frames:v", "1", path); err != nil {
		_ = os.Remove(path)
		return nil, nil
	}
	mimeType := "image/jpeg"
	return &relative, &mimeType
}

func (l *LocalLibrary) Delete(ctx context.Context, id string) error {
	track, err := l.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}
	rooms, err := l.repository.DeleteWithReferences(ctx, id, l.now().UnixMilli())
	if err != nil {
		return err
	}
	l.notifyTracksRemoved(ctx, id, rooms)
	for _, relative := range []*string{track.SourcePath, track.OGGPath, track.CoverPath} {
		if relative == nil {
			continue
		}
		path, resolveErr := ResolveWithin(l.root, *relative)
		if resolveErr != nil {
			return fmt.Errorf("delete local media: %w", resolveErr)
		}
		_ = os.Remove(path)
	}
	return nil
}

func splitInputArtists(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || r == '/' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return []string{"Unknown"}
	}
	return result
}

func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func intPointer(value int) *int { return &value }
