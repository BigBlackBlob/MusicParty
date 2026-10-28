package subsonic

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestBatch7ProtocolFailureIsNotEmptySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"subsonic-response":{"status":"failed","error":{"code":40,"message":"Wrong username or password"}}}`)
	}))
	defer server.Close()
	service := New(platform.NewClient(0), Config{BaseURL: server.URL})
	_, err := service.Search(context.Background(), "test", 0, 20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "40")
}

func TestBatch7AlbumSearchSecondPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset := r.URL.Query().Get("albumOffset")
		if offset == "" {
			offset = "0"
		}
		fmt.Fprintf(w, `{"subsonic-response":{"status":"ok","searchResult3":{"album":[{"id":"page-%s","name":"Album"}]}}}`, offset)
	}))
	defer server.Close()
	service := New(platform.NewClient(0), Config{BaseURL: server.URL})
	first, err := service.SearchAlbums(context.Background(), "test", 0, 20)
	require.NoError(t, err)
	second, err := service.SearchAlbums(context.Background(), "test", 20, 20)
	require.NoError(t, err)
	require.Equal(t, "page-0", first.Items[0].ID)
	require.Equal(t, "page-20", second.Items[0].ID)
}
