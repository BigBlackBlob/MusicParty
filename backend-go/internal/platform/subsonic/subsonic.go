package subsonic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
)

type Config struct {
	ID, BaseURL, Username, Password, Client, Version, ProxyPrefix string
	Label, RoomID                                                 string
	Enabled                                                       bool
}
type Service struct {
	client *platform.Client
	cfg    Config
}

func New(client *platform.Client, cfg Config) *Service { return &Service{client: client, cfg: cfg} }
func (s *Service) Name() string                        { return s.cfg.ID }
func (s *Service) Label() string {
	if s.cfg.Label != "" {
		return s.cfg.Label
	}
	return s.cfg.ID
}
func (s *Service) RoomID() string { return s.cfg.RoomID }
func (s *Service) Available() bool {
	return s.cfg.Enabled && s.cfg.BaseURL != "" && s.cfg.Username != "" && s.cfg.Password != ""
}
func (s *Service) get(ctx context.Context, endpoint string, values url.Values, target any) error {
	return s.client.JSON(ctx, s.endpoint(endpoint, values), nil, target)
}
func (s *Service) endpoint(endpoint string, values url.Values) string {
	salt := "musicparty"
	sum := md5.Sum([]byte(s.cfg.Password + salt))
	values.Set("u", s.cfg.Username)
	values.Set("t", hex.EncodeToString(sum[:]))
	values.Set("s", salt)
	values.Set("v", s.cfg.Version)
	values.Set("c", s.cfg.Client)
	values.Set("f", "json")
	return strings.TrimRight(s.cfg.BaseURL, "/") + "/rest/" + endpoint + "?" + values.Encode()
}
func (s *Service) StreamURL(_ context.Context, id string) (string, error) {
	return s.endpoint("stream.view", url.Values{"id": {id}, "format": {"mp3"}}), nil
}
func (s *Service) CoverURL(_ context.Context, id string) (string, error) {
	return s.endpoint("getCoverArt.view", url.Values{"id": {id}, "size": {"300"}}), nil
}
func (s *Service) Search(ctx context.Context, keyword string, offset, limit int) ([]platform.Music, error) {
	var r response
	err := s.get(ctx, "search3.view", url.Values{"query": {keyword}, "songOffset": {strconv.Itoa(offset)}, "songCount": {strconv.Itoa(limit)}, "albumCount": {"0"}, "artistCount": {"0"}}, &r)
	return s.mapSongs(r.Body.Search.Songs), err
}
func (s *Service) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	var r struct {
		Body struct {
			Song song `json:"song"`
		} `json:"subsonic-response"`
	}
	if err := s.get(ctx, "getSong.view", url.Values{"id": {id}}, &r); err != nil {
		return platform.PlayableMusic{}, err
	}
	items := s.mapSongs([]song{r.Body.Song})
	if len(items) == 0 || items[0].ID == "" {
		return platform.PlayableMusic{}, fmt.Errorf("subsonic song %s was not found", id)
	}
	streamPrefix := strings.TrimSuffix(strings.TrimRight(s.cfg.ProxyPrefix, "/"), "/cover") + "/stream"
	return platform.PlayableMusic{Music: items[0], URL: streamPrefix + "/" + url.PathEscape(id)}, nil
}
func (s *Service) SearchAlbums(ctx context.Context, keyword string) ([]platform.Album, error) {
	var r response
	err := s.get(ctx, "search3.view", url.Values{"query": {keyword}, "songCount": {"0"}, "albumCount": {"50"}, "artistCount": {"0"}}, &r)
	out := make([]platform.Album, 0, len(r.Body.Search.Albums))
	for _, a := range r.Body.Search.Albums {
		out = append(out, platform.Album{ID: a.ID, Name: a.Name, ArtistName: a.Artist, CoverURL: s.cover(a.CoverArt), TrackCount: a.SongCount, Platform: s.cfg.ID})
	}
	return out, err
}
func (s *Service) AlbumSongs(ctx context.Context, id string) ([]platform.Music, error) {
	var r response
	err := s.get(ctx, "getAlbum.view", url.Values{"id": {id}}, &r)
	return s.mapSongs(r.Body.Album.Songs), err
}
func (*Service) UserPlaylists(context.Context, string) ([]platform.Playlist, error) {
	return []platform.Playlist{}, nil
}
func (*Service) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (*Service) SearchUsers(context.Context, string) ([]platform.User, error) {
	return []platform.User{}, nil
}
func (*Service) Lyric(context.Context, string) (platform.Lyric, error) { return platform.Lyric{}, nil }

type response struct {
	Body struct {
		Search struct {
			Songs  []song  `json:"song"`
			Albums []album `json:"album"`
		} `json:"searchResult3"`
		Album struct {
			Songs []song `json:"song"`
		} `json:"album"`
	} `json:"subsonic-response"`
}
type song struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration int64  `json:"duration"`
	CoverArt string `json:"coverArt"`
}
type album struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Artist    string `json:"artist"`
	SongCount int    `json:"songCount"`
	CoverArt  string `json:"coverArt"`
}

func (s *Service) mapSongs(items []song) []platform.Music {
	out := make([]platform.Music, 0, len(items))
	for _, v := range items {
		artists := []string{"Unknown"}
		if v.Artist != "" {
			artists = []string{v.Artist}
		}
		out = append(out, platform.Music{ID: v.ID, Name: v.Title, Artists: artists, Duration: v.Duration * 1000, Platform: s.cfg.ID, CoverURL: s.cover(v.CoverArt)})
	}
	return out
}
func (s *Service) cover(id string) string {
	if id == "" {
		return ""
	}
	return strings.TrimRight(s.cfg.ProxyPrefix, "/") + "/" + url.PathEscape(id)
}
