package media

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	return &LocalLibrary{root: absolute, maxUpload: maxUpload, repository: repository, transcoder: transcoder, now: time.Now}, nil
}

func (l *LocalLibrary) Root() string { return l.root }

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
		_ = os.Remove(sourcePath)
		return *existing, true, nil
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
		_ = os.Remove(sourcePath)
		if existing, findErr := l.repository.FindActiveByOriginalHash(ctx, hash); findErr == nil {
			return *existing, true, nil
		}
		return storesqlite.LocalTrack{}, false, err
	}
	relativeOutput := filepath.ToSlash(filepath.Join("media", id+".ogg"))
	outputPath, _ := ResolveWithin(l.root, relativeOutput)
	err = l.transcoder.Submit(TranscodeJob{ID: id, Input: sourcePath, Output: outputPath, Complete: func(transcodeErr error) {
		l.completeTranscode(id, relativeSource, relativeOutput, transcodeErr)
	}})
	if err != nil {
		message := err.Error()
		_ = l.repository.UpdateStatus(context.Background(), id, storesqlite.LocalTrackStatusUpdate{Status: "FAILED", ErrorMessage: &message, StatusMessage: optional("Transcode queue rejected"), ProgressPercent: intPointer(0), UpdatedAt: l.now().UnixMilli()})
		return track, false, err
	}
	return track, false, nil
}

func (l *LocalLibrary) completeTranscode(id, sourceRelative, outputRelative string, transcodeErr error) {
	now := l.now().UnixMilli()
	if transcodeErr != nil {
		message := transcodeErr.Error()
		_ = l.repository.UpdateStatus(context.Background(), id, storesqlite.LocalTrackStatusUpdate{Status: "FAILED", ErrorMessage: &message, StatusMessage: optional("Transcode failed"), ProgressPercent: intPointer(0), CompletedAt: &now, UpdatedAt: now})
		return
	}
	source, _ := ResolveWithin(l.root, sourceRelative)
	duration := l.probeDuration(source)
	coverRelative, coverMIME := l.extractCover(id, source)
	_ = l.repository.CompleteTranscode(context.Background(), id, outputRelative, duration, coverRelative, coverMIME, now)
	if source, err := ResolveWithin(l.root, sourceRelative); err == nil {
		_ = os.Remove(source)
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
	if err := l.repository.DeleteLocalReferences(ctx, id); err != nil {
		return err
	}
	if err := l.repository.MarkDeleted(ctx, id, l.now().UnixMilli()); err != nil {
		return err
	}
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
