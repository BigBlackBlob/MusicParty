package httpapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/stretchr/testify/require"
)

type forbiddenUploadBody struct{ reads int }

func (b *forbiddenUploadBody) Read([]byte) (int, error) { b.reads++; return 0, io.EOF }
func (*forbiddenUploadBody) Close() error               { return nil }

func TestUploadAdmissionHappensBeforeBodyRead(t *testing.T) {
	store := httpStore(t)
	insertStage5User(t, store, "guest-token", "guest", "GUEST")
	insertStage5User(t, store, "admin-token", "admin", "ADMIN")
	cfg := testConfig(t)
	api := NewMediaAPI(cfg, account.New(store), store, nil, NewPlatformAPI(cfg, nil), http.DefaultClient, nil)
	for _, token := range []string{"", "guest-token", "admin-token"} {
		t.Run(token, func(t *testing.T) {
			if token == "admin-token" {
				api.uploads <- struct{}{}
				api.uploads <- struct{}{}
				defer func() { <-api.uploads; <-api.uploads }()
			}
			body := &forbiddenUploadBody{}
			request := httptest.NewRequest("POST", "/api/local/tracks/upload", body)
			request.Header.Set("Content-Type", "multipart/form-data; boundary=example")
			request.AddCookie(&http.Cookie{Name: "MP_SESSION", Value: token})
			response := httptest.NewRecorder()
			err := api.uploadLocal(response, request)
			if token == "admin-token" {
				var apiErr *APIError
				require.ErrorAs(t, err, &apiErr)
				require.Equal(t, 503, apiErr.Status)
			} else {
				require.NoError(t, err)
				require.Equal(t, 403, response.Code)
			}
			require.Zero(t, body.reads)
		})
	}
}

func TestCoverRejectsLargePixelAllocationBeforeDecode(t *testing.T) {
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	_, err := decodeBoundedCover(data.Bytes())
	require.NoError(t, err)
	bomb := append([]byte(nil), data.Bytes()...)
	binary.BigEndian.PutUint32(bomb[16:20], 50000)
	binary.BigEndian.PutUint32(bomb[20:24], 50000)
	binary.BigEndian.PutUint32(bomb[29:33], crc32.ChecksumIEEE(bomb[12:29]))
	require.Less(t, len(bomb), 1000)
	_, err = decodeBoundedCover(bomb)
	require.ErrorContains(t, err, "dimensions exceed limit")
}

type resourceFixture struct {
	fixturePlatform
	name  string
	calls int
}

func (f *resourceFixture) Name() string { return f.name }
func (f *resourceFixture) StreamURL(context.Context, string) (string, error) {
	f.calls++
	return "", nil
}
func (f *resourceFixture) CoverURL(context.Context, string) (string, error) {
	f.calls++
	return "", nil
}

func TestYouTubeRateBudgetAndCoverSlotsRejectBeforeUpstream(t *testing.T) {
	cfg := testConfig(t)
	source := &resourceFixture{name: "youtube"}
	platforms := NewPlatformAPI(cfg, nil, source)
	store := httpStore(t)
	api := NewMediaAPI(cfg, account.New(store), store, nil, platforms, http.DefaultClient, nil)
	for range 30 {
		request := httptest.NewRequest("GET", "/api/youtube/stream/abcdefghijk", nil)
		require.NoError(t, api.proxyPlatform(httptest.NewRecorder(), request, "youtube", "abcdefghijk", false))
	}
	response := httptest.NewRecorder()
	err := api.proxyPlatform(response, httptest.NewRequest("GET", "/api/youtube/stream/abcdefghijk", nil), "youtube", "abcdefghijk", false)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusTooManyRequests, apiErr.Status)
	require.NotEmpty(t, response.Header().Get("Retry-After"))
	require.Equal(t, 30, source.calls)
	cover := &resourceFixture{name: "navidrome"}
	platforms.Register(cover)
	platforms.coverSlots <- struct{}{}
	platforms.coverSlots <- struct{}{}
	err = platforms.coverColor(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/theme/extract-cover-color?url=/api/navidrome/cover/art", nil))
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.Status)
	require.Zero(t, cover.calls)
}
