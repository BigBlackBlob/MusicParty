package bilibili

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/assert"
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

func TestSearchTrackURLs(t *testing.T) {
	const id = "BV1abC23dEF4"
	const link = "https://www.bilibili.com/video/" + id
	const video = `{"code":0,"data":{"bvid":"` + id + `","title":"Track","owner":{"name":"Artist"},"duration":62,"pic":"//img.test/cover.jpg"}}`
	want := []platform.Music{{ID: id, Name: "Track", Artists: []string{"Artist"}, Duration: 62000, Platform: "bilibili", CoverURL: "https://img.test/cover.jpg"}}
	for _, test := range []struct {
		name, keyword string
		status        int
		body          string
		offset        int
		fallback      bool
		noCredential  bool
	}{
		{name: "video", keyword: link + "?p=1", body: video},
		{name: "later page", keyword: link, offset: 1},
		{name: "missing BVID", keyword: link, body: `{"code":0,"data":{}}`},
		{name: "nonzero code", keyword: link, body: `{"code":-404,"data":{"bvid":"` + id + `"}}`},
		{name: "not found", keyword: link, status: http.StatusNotFound},
		{name: "unauthorized", keyword: link, status: http.StatusUnauthorized},
		{name: "upstream failure", keyword: link, status: http.StatusInternalServerError},
		{name: "missing SESSDATA", keyword: link, noCredential: true},
		{name: "profile", keyword: "https://www.bilibili.com/space/123", fallback: true},
		{name: "playlist", keyword: "https://www.bilibili.com/list/123", fallback: true},
		{name: "wrong host", keyword: "https://bilibili.com/video/" + id, fallback: true},
		{name: "short link", keyword: "https://b23.tv/abcd123", fallback: true},
		{name: "http scheme", keyword: "http://www.bilibili.com/video/" + id, fallback: true},
		{name: "invalid BVID", keyword: "https://www.bilibili.com/video/BV1abC23d_F4", fallback: true},
		{name: "extra path segment", keyword: "https://www.bilibili.com/video/extra/" + id, fallback: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				assert.Equal(t, http.MethodGet, r.Method)
				if test.fallback {
					switch r.URL.Path {
					case "/x/web-interface/nav":
						_, _ = w.Write([]byte(`{"data":{"wbi_img":{"img_url":"https://i.test/abcdefghijklmnopqrstuvwxyz0123456789.png","sub_url":"https://i.test/ABCDEFGHIJKLMNOPQRSTUVWXYZ9876543210.png"}}}`))
					case "/x/web-interface/wbi/search/type":
						assert.Equal(t, test.keyword, r.URL.Query().Get("keyword"))
						_, _ = w.Write([]byte(`{"code":0,"data":{"result":[]}}`))
					default:
						t.Errorf("unexpected keyword search request: %s", r.URL)
						http.NotFound(w, r)
					}
					return
				}
				assert.Equal(t, "/x/web-interface/view", r.URL.Path)
				assert.Equal(t, id, r.URL.Query().Get("bvid"))
				assert.Contains(t, r.Header.Get("Cookie"), "SESSDATA=secret")
				assert.Equal(t, "https://www.bilibili.com/", r.Header.Get("Referer"))
				if test.status != 0 {
					w.WriteHeader(test.status)
				}
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)
			credential := "SESSDATA=secret"
			if test.noCredential {
				credential = ""
			}
			client := platform.NewClient(time.Second)
			client.HTTP.Transport = rewriteToFixture{base: server.URL}
			client.Retries = 0
			service := New(client, "https://api.bilibili.com", credential)
			tracks, err := service.Search(t.Context(), test.keyword, test.offset, 20)
			switch {
			case test.noCredential:
				require.EqualError(t, err, "bilibili SESSDATA is not configured")
				require.Zero(t, calls.Load())
			case test.offset != 0:
				require.NoError(t, err)
				require.Equal(t, []platform.Music{}, tracks)
				require.Zero(t, calls.Load())
			case test.status == http.StatusUnauthorized || test.status == http.StatusInternalServerError:
				var upstream *platform.UpstreamError
				require.ErrorAs(t, err, &upstream)
				require.Equal(t, test.status, upstream.Status)
				require.Empty(t, tracks)
			case test.body == video:
				require.NoError(t, err)
				require.Equal(t, want, tracks)
			default:
				require.NoError(t, err)
				require.Equal(t, []platform.Music{}, tracks)
			}
			switch {
			case test.fallback:
				require.EqualValues(t, 2, calls.Load())
			case test.offset == 0 && !test.noCredential:
				require.EqualValues(t, 1, calls.Load())
			}
		})
	}
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

type rewriteToFixture struct{ base string }

func (t rewriteToFixture) RoundTrip(request *http.Request) (*http.Response, error) {
	fixture, err := url.Parse(t.base)
	if err != nil {
		return nil, err
	}
	forked := request.Clone(request.Context())
	forked.URL.Scheme = fixture.Scheme
	forked.URL.Host = fixture.Host
	forked.Host = ""
	return http.DefaultTransport.RoundTrip(forked)
}
