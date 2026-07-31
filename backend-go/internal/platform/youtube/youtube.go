package youtube

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

type Service struct {
	client     *platform.Client
	apiKey     string
	enabled    bool
	maxResults int
	ytdlpPath  string
}

func New(client *platform.Client, enabled bool, apiKey string, maxResults int, ytdlpPath ...string) *Service {
	path := "yt-dlp"
	if len(ytdlpPath) > 0 && strings.TrimSpace(ytdlpPath[0]) != "" {
		path = ytdlpPath[0]
	}
	return &Service{client: client, enabled: enabled, apiKey: apiKey, maxResults: maxResults, ytdlpPath: path}
}
func (*Service) Name() string      { return "youtube" }
func (s *Service) Available() bool { return s.enabled && s.apiKey != "" }
func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	if limit > s.maxResults {
		limit = s.maxResults
	}
	if limit < 1 {
		limit = 1
	}
	q := url.Values{"part": {"snippet"}, "type": {"video"}, "q": {keyword}, "maxResults": {strconv.Itoa(limit)}, "key": {s.apiKey}}
	var search struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet snippet `json:"snippet"`
		} `json:"items"`
	}
	if err := s.client.JSON(ctx, "https://www.googleapis.com/youtube/v3/search?"+q.Encode(), nil, &search); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(search.Items))
	for _, v := range search.Items {
		ids = append(ids, v.ID.VideoID)
	}
	durations := map[string]int64{}
	if len(ids) > 0 {
		var videos struct {
			Items []struct {
				ID      string `json:"id"`
				Content struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		q = url.Values{"part": {"contentDetails"}, "id": {strings.Join(ids, ",")}, "key": {s.apiKey}}
		if err := s.client.JSON(ctx, "https://www.googleapis.com/youtube/v3/videos?"+q.Encode(), nil, &videos); err != nil {
			return nil, err
		}
		for _, v := range videos.Items {
			if d, err := parseDuration(v.Content.Duration); err == nil {
				durations[v.ID] = d.Milliseconds()
			}
		}
	}
	out := make([]platform.Music, 0, len(search.Items))
	for _, v := range search.Items {
		out = append(out, platform.Music{ID: v.ID.VideoID, Name: v.Snippet.Title, Artists: []string{v.Snippet.ChannelTitle}, Duration: durations[v.ID.VideoID], Platform: "youtube", CoverURL: v.Snippet.Thumbnails.High.URL})
	}
	return out, nil
}
func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	var videos struct {
		Items []struct {
			ID      string  `json:"id"`
			Snippet snippet `json:"snippet"`
			Content struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	query := url.Values{"part": {"snippet,contentDetails"}, "id": {id}, "key": {s.apiKey}}
	if err := s.client.JSON(ctx, "https://www.googleapis.com/youtube/v3/videos?"+query.Encode(), nil, &videos); err != nil {
		return platform.PlayableMusic{}, err
	}
	if len(videos.Items) == 0 {
		return platform.PlayableMusic{}, fmt.Errorf("youtube video %s was not found", id)
	}
	item := videos.Items[0]
	duration, err := parseDuration(item.Content.Duration)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	music := platform.Music{ID: item.ID, Name: item.Snippet.Title, Artists: []string{item.Snippet.ChannelTitle}, Duration: duration.Milliseconds(), Platform: "youtube", CoverURL: item.Snippet.Thumbnails.High.URL}
	return platform.PlayableMusic{Music: music, URL: "/api/youtube/stream/" + url.PathEscape(id)}, nil
}

type snippet struct {
	Title        string `json:"title"`
	ChannelTitle string `json:"channelTitle"`
	Thumbnails   struct {
		High struct {
			URL string `json:"url"`
		} `json:"high"`
	} `json:"thumbnails"`
}

func parseDuration(raw string) (time.Duration, error) {
	raw = strings.TrimPrefix(raw, "P")
	var days int64
	if i := strings.IndexByte(raw, 'D'); i >= 0 {
		days, _ = strconv.ParseInt(raw[:i], 10, 64)
		raw = raw[i+1:]
	}
	raw = strings.TrimPrefix(raw, "T")
	var total time.Duration
	number := ""
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			number += string(r)
			continue
		}
		n, err := strconv.ParseInt(number, 10, 64)
		if err != nil {
			return 0, err
		}
		number = ""
		switch r {
		case 'H':
			total += time.Duration(n) * time.Hour
		case 'M':
			total += time.Duration(n) * time.Minute
		case 'S':
			total += time.Duration(n) * time.Second
		}
	}
	return total + time.Duration(days)*24*time.Hour, nil
}
func (*Service) UserPlaylists(context.Context, string) ([]platform.Playlist, error) {
	return []platform.Playlist{}, nil
}
func (*Service) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (*Service) SearchAlbums(context.Context, string) ([]platform.Album, error) {
	return []platform.Album{}, nil
}
func (*Service) AlbumSongs(context.Context, string) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (*Service) SearchUsers(context.Context, string) ([]platform.User, error) {
	return []platform.User{}, nil
}
func (*Service) Lyric(context.Context, string) (platform.Lyric, error) { return platform.Lyric{}, nil }
func (s *Service) StreamURL(ctx context.Context, id string) (string, error) {
	return media.ResolveYTDLP(ctx, s.ytdlpPath, id)
}
func (*Service) CoverURL(context.Context, string) (string, error) { return "", nil }
