package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func (c *Cache) download(job downloadJob) {
	ctx, cancel := context.WithDeadline(c.ctx, job.deadline)
	defer cancel()
	platform := strings.SplitN(job.key, ":", 2)[0]
	if limit := c.limits[platform]; limit != nil {
		select {
		case limit <- struct{}{}:
			defer func() { <-limit }()
		case <-ctx.Done():
			return
		}
	}
	for attempt := 0; attempt <= c.retries; attempt++ {
		if ctx.Err() != nil {
			return
		}
		if attempt > 0 && !waitDownload(ctx, c.retryDelay(attempt)) {
			return
		}
		retry, after := c.downloadAttempt(ctx, job)
		if !retry || ctx.Err() != nil {
			return
		}
		if attempt < c.retries && after > 0 && !waitDownload(ctx, after) {
			return
		}
	}
}

// Each attempt owns one reservation and one partial. The task deadline spans
// queueing, platform admission, all retries and response-body reads.
func (c *Cache) downloadAttempt(ctx context.Context, job downloadJob) (bool, time.Duration) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, job.url, nil)
	if err != nil {
		return false, 0
	}
	response, err := c.client.Do(request)
	if err != nil {
		return true, 0
	}
	defer response.Body.Close()
	// A cache entry represents a full resource, never a 206 fragment.
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusTooManyRequests {
			delay, _ := retryAfter(response.Header.Get("Retry-After"), time.Now())
			return true, delay
		}
		return response.StatusCode >= 500, 0
	}
	if response.ContentLength > c.options.MaxFileBytes {
		return false, 0
	}
	initial := c.options.ReservationBytes
	if response.ContentLength >= 0 {
		initial = response.ContentLength
	}
	reservation, err := c.reserveSpace(initial)
	if err != nil {
		return false, 0
	}
	path := c.pathForKey(job.key)
	part := path + ".part"
	file, err := os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		c.releaseReservation(reservation)
		return false, 0
	}
	committed := false
	defer func() {
		if !committed {
			c.discardPartial(part, reservation)
		}
	}()
	writer := &cacheWriter{ctx: ctx, cache: c, file: file, reservation: reservation}
	size, copyErr := io.Copy(writer, response.Body)
	closeErr := errors.Join(file.Close(), response.Body.Close())
	if ctx.Err() != nil {
		return false, 0
	}
	if copyErr != nil || closeErr != nil {
		return !errors.Is(copyErr, errDownloadTooLarge) && !errors.Is(copyErr, errCacheCapacity), 0
	}
	if err := c.commitPartial(ctx, part, path, size, reservation); err != nil {
		return false, 0
	}
	committed = true
	return false, 0
}

func (c *Cache) retryDelay(attempt int) time.Duration {
	delay := min(c.retryBase, c.options.RetryMaxDelay)
	for i := 1; i < attempt && delay < c.options.RetryMaxDelay; i++ {
		if delay > c.options.RetryMaxDelay/2 {
			return c.options.RetryMaxDelay
		}
		delay *= 2
		if delay == 0 {
			break
		}
	}
	return delay
}

func waitDownload(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
