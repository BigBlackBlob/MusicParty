package netease

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

type Service struct {
	client  *platform.Client
	baseURL string
	mu      sync.RWMutex
	cookie  string
}

func New(client *platform.Client, baseURL, cookie string) *Service {
	return &Service{client: client, baseURL: strings.TrimRight(baseURL, "/"), cookie: cookie}
}
func (*Service) Name() string    { return "netease" }
func (*Service) Available() bool { return true }
func (s *Service) UpdateCredential(value string) {
	s.mu.Lock()
	s.cookie = value
	s.mu.Unlock()
}
func (s *Service) credential() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cookie
}

func (s *Service) get(ctx context.Context, path string, query url.Values, target any) error {
	if credential := s.credential(); credential != "" {
		query.Set("cookie", credential)
	}
	return s.client.JSON(ctx, s.baseURL+path+"?"+query.Encode(), nil, target)
}

func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	var response struct {
		Result struct {
			Songs []song `json:"songs"`
		} `json:"result"`
	}
	err := s.get(ctx, "/cloudsearch", url.Values{"keywords": {keyword}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}, &response)
	return mapSongs(response.Result.Songs, ""), err
}
func (s *Service) ResolvePlayable(ctx context.Context, musicID string) (platform.PlayableMusic, error) {
	var response struct {
		Songs []song `json:"songs"`
	}
	err := s.get(ctx, "/song/detail", url.Values{"ids": {musicID}}, &response)
	if err != nil {
		return platform.PlayableMusic{}, err
	}
	if len(response.Songs) == 0 {
		return platform.PlayableMusic{}, fmt.Errorf("netease song %s was not found", musicID)
	}
	music := mapSongs(response.Songs[:1], "")[0]
	return platform.PlayableMusic{Music: music, URL: "/api/netease/stream/" + url.PathEscape(music.ID)}, nil
}
func (s *Service) UserPlaylists(ctx context.Context, userID string) ([]platform.Playlist, error) {
	var response struct {
		Playlists []struct {
			ID         jsonID `json:"id"`
			Name       string `json:"name"`
			Cover      string `json:"coverImgUrl"`
			TrackCount int    `json:"trackCount"`
		} `json:"playlist"`
	}
	err := s.get(ctx, "/user/playlist", url.Values{"uid": {userID}}, &response)
	result := make([]platform.Playlist, 0, len(response.Playlists))
	for _, item := range response.Playlists {
		result = append(result, platform.Playlist{ID: item.ID.String(), Name: item.Name, CoverImgURL: image(item.Cover), TrackCount: item.TrackCount, Platform: "netease"})
	}
	return result, err
}
func (s *Service) PlaylistSongs(ctx context.Context, playlistID string, offset, limit int) ([]platform.Music, error) {
	var response struct {
		Songs []song `json:"songs"`
	}
	err := s.get(ctx, "/playlist/track/all", url.Values{"id": {playlistID}, "offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(limit)}}, &response)
	return mapSongs(response.Songs, ""), err
}
func (s *Service) SearchAlbums(ctx context.Context, keyword string) ([]platform.Album, error) {
	var response struct {
		Result struct {
			Albums []struct {
				ID           jsonID `json:"id"`
				Name, PicURL string
				Size         int
				Artist       struct {
					Name string `json:"name"`
				} `json:"artist"`
			} `json:"albums"`
		} `json:"result"`
	}
	err := s.get(ctx, "/cloudsearch", url.Values{"keywords": {keyword}, "type": {"10"}}, &response)
	result := make([]platform.Album, 0, len(response.Result.Albums))
	for _, item := range response.Result.Albums {
		result = append(result, platform.Album{ID: item.ID.String(), Name: item.Name, ArtistName: item.Artist.Name, CoverURL: image(item.PicURL), TrackCount: item.Size, Platform: "netease"})
	}
	return result, err
}
func (s *Service) AlbumSongs(ctx context.Context, albumID string) ([]platform.Music, error) {
	var response struct {
		Album struct {
			PicURL string `json:"picUrl"`
		} `json:"album"`
		Songs []song `json:"songs"`
	}
	err := s.get(ctx, "/album", url.Values{"id": {albumID}}, &response)
	return mapSongs(response.Songs, image(response.Album.PicURL)), err
}
func (s *Service) SearchUsers(ctx context.Context, keyword string) ([]platform.User, error) {
	var response struct {
		Result struct {
			Profiles []struct {
				UserID    jsonID `json:"userId"`
				Nickname  string `json:"nickname"`
				AvatarURL string `json:"avatarUrl"`
			} `json:"userprofiles"`
		} `json:"result"`
	}
	err := s.get(ctx, "/search", url.Values{"keywords": {keyword}, "type": {"1002"}}, &response)
	result := make([]platform.User, 0, len(response.Result.Profiles))
	for _, item := range response.Result.Profiles {
		result = append(result, platform.User{ID: item.UserID.String(), Name: item.Nickname, AvatarURL: image(item.AvatarURL), Platform: "netease"})
	}
	return result, err
}
func (s *Service) Lyric(ctx context.Context, musicID string) (platform.Lyric, error) {
	var response struct {
		LRC, TLyric, RomaLRC struct {
			Lyric string `json:"lyric"`
		}
	}
	err := s.get(ctx, "/lyric", url.Values{"id": {musicID}}, &response)
	return platform.Lyric{Lyric: response.LRC.Lyric, TranslatedLyric: response.TLyric.Lyric, RomanizedLyric: response.RomaLRC.Lyric}, err
}

func (s *Service) StreamURL(ctx context.Context, musicID string) (string, error) {
	var response struct {
		Data []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	err := s.get(ctx, "/song/url/v1", url.Values{"id": {musicID}, "level": {"exhigh"}}, &response)
	if err != nil || len(response.Data) == 0 {
		return "", err
	}
	return response.Data[0].URL, nil
}

func (*Service) CoverURL(context.Context, string) (string, error) { return "", nil }

type song struct {
	ID       jsonID `json:"id"`
	Name     string `json:"name"`
	Duration int64  `json:"dt"`
	Artists  []struct {
		Name string `json:"name"`
	} `json:"ar"`
	Album struct {
		PicURL string `json:"picUrl"`
	} `json:"al"`
}

func mapSongs(items []song, fallback string) []platform.Music {
	result := make([]platform.Music, 0, len(items))
	for _, item := range items {
		artists := make([]string, 0, len(item.Artists))
		for _, artist := range item.Artists {
			artists = append(artists, artist.Name)
		}
		cover := image(item.Album.PicURL)
		if cover == "" {
			cover = fallback
		}
		result = append(result, platform.Music{ID: item.ID.String(), Name: item.Name, Artists: artists, Duration: item.Duration, Platform: "netease", CoverURL: cover})
	}
	return result
}
func image(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") {
		raw = "https://" + strings.TrimPrefix(raw, "http://")
	}
	if !strings.Contains(raw, "?") {
		raw += "?param=300y300"
	}
	return raw
}

type jsonID string

func (id *jsonID) UnmarshalJSON(data []byte) error {
	raw := strings.Trim(string(data), "\"")
	*id = jsonID(raw)
	return nil
}
func (id jsonID) String() string { return string(id) }
