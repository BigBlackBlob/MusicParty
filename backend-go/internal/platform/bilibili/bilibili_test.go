package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestSearchSignsWBIAndMapsFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/web-interface/nav":
			_, _ = w.Write([]byte(`{"data":{"wbi_img":{"img_url":"https://i.test/abcdefghijklmnopqrstuvwxyz0123456789.png","sub_url":"https://i.test/ABCDEFGHIJKLMNOPQRSTUVWXYZ9876543210.png"}}}`))
		case "/x/web-interface/wbi/search/type":
			require.NotEmpty(t, r.URL.Query().Get("w_rid"))
			require.Equal(t, "1785379200", r.URL.Query().Get("wts"))
			_, _ = w.Write([]byte(`{"code":0,"data":{"result":[{"bvid":"BV1","title":"<em class=\"keyword\">标题</em>","author":"作者","duration":"01:02","pic":"//img.test/a.jpg"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 4096}, server.URL, "cookie=value")
	service.now = func() time.Time { return time.Unix(1785379200, 0) }
	songs, err := service.Search(context.Background(), "标题", 0, 20)
	require.NoError(t, err)
	require.Equal(t, "标题", songs[0].Name)
	require.Equal(t, int64(62000), songs[0].Duration)
	require.Equal(t, "https://img.test/a.jpg", songs[0].CoverURL)
}

func TestPlaylistSongsHonorsOffsetsAcrossUpstreamPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("pn"))
		size, _ := strconv.Atoi(r.URL.Query().Get("ps"))
		if size != 20 {
			http.Error(w, "upstream page limit", 400)
			return
		}
		items := []map[string]any{}
		for i := (page - 1) * size; i < min(47, page*size); i++ {
			items = append(items, map[string]any{"bvid": strconv.Itoa(i), "title": "Song", "duration": 60, "upper": map[string]string{"name": "Artist"}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"medias": items}})
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), MaxBody: 1 << 20}, server.URL, "cookie")
	for _, test := range []struct{ offset, limit, count int }{{0, 45, 45}, {17, 25, 25}, {40, 5, 5}, {45, 20, 2}} {
		songs, err := service.PlaylistSongs(context.Background(), "list", test.offset, test.limit)
		require.NoError(t, err)
		require.Len(t, songs, test.count)
		for i, song := range songs {
			require.Equal(t, strconv.Itoa(test.offset+i), song.ID)
		}
	}
}
