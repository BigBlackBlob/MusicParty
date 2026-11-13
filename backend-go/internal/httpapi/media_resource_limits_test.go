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
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
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
	api := NewMediaAPI(cfg, account.New(store), store, nil, NewPlatformAPI(cfg), http.DefaultClient, nil)
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
	platforms := NewPlatformAPI(cfg, source)
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

type streamErrorFixture struct {
	fixturePlatform
	name        string
	err         error
	invalidated []string
}

func (f *streamErrorFixture) Name() string                                      { return f.name }
func (f *streamErrorFixture) StreamURL(context.Context, string) (string, error) { return "", f.err }
func (f *streamErrorFixture) CoverURL(context.Context, string) (string, error)  { return "", nil }
func (f *streamErrorFixture) InvalidateStream(id string)                        { f.invalidated = append(f.invalidated, id) }

func TestPlatformStreamResolutionErrorMapsToJSON(t *testing.T) {
	for _, test := range []struct{ kind, code string }{
		{platform.StreamTrialOnly, "stream-trial-only"},
		{platform.StreamUnavailable, "stream-unavailable"},
	} {
		t.Run(test.code, func(t *testing.T) {
			cfg := testConfig(t)
			source := &streamErrorFixture{name: "netease", err: &platform.StreamResolutionError{Kind: test.kind, Reason: 1}}
			api := NewMediaAPI(cfg, account.New(httpStore(t)), nil, nil, NewPlatformAPI(cfg, source), http.DefaultClient, nil)
			response := httptest.NewRecorder()
			err := api.proxyPlatform(response, httptest.NewRequest(http.MethodGet, "/api/netease/stream/42", nil), "netease", "42", false)
			require.Error(t, err)
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			require.Equal(t, http.StatusUnprocessableEntity, apiErr.Status)
			require.Equal(t, test.code, apiErr.Code)
			WriteMappedError(response, err)
			require.Equal(t, http.StatusUnprocessableEntity, response.Code)
			require.Contains(t, response.Body.String(), `"code":"`+test.code+`"`)
		})
	}
}
